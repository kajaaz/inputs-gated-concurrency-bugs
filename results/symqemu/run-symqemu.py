#!/usr/bin/env python3
"""SymQEMU on the 11 harnesses, then replay of every generated input on the
race oracle (go build -race, or clang TSan for xz and RocksDB).

SymQEMU makes only stdin or SYMCC_INPUT_FILE symbolic. The Go and RocksDB
harnesses therefore read their arguments from stdin (harness/symstdin.go,
harness/rocksdb-inplace-update-stdin.cc). The xz harness reads the .xz file
named by SYMCC_INPUT_FILE; its thread count stays concrete on argv.

Exploration is breadth-first over generated test cases until --budget seconds
are spent in SymQEMU, or until the oracle confirms a generated input (other
than the seed) as racy; replays run in the background during exploration. Inputs are deduplicated by what the harness parses from them (the
os.Args fields for Go, the first two bytes for RocksDB, the whole file for xz),
and every unique input is then replayed on the race oracle.

Usage: python3 run-symqemu.py [--jobs 4] [--budget 600] [case ...]
"""
import argparse
import collections
import concurrent.futures as cf
import hashlib
import io
import json
import os
import re
import shlex
import shutil
import subprocess
import threading
import time
from pathlib import Path

E = Path(__file__).resolve().parent
ROOT = E.parents[1]
GO = ROOT / "harnesses/go"
QEMU = Path.home() / "Programs/symqemu/build/qemu-x86_64"
XZIN = E / "work/xz-inputs"

GO_SEEDS = {
    "nethttp-expect-continue-2020": ("1 1", ["0 1", "1 0", "0 0"]),
    "resty-pool-retry-2023": ("1 503", ["0 503", "1 200", "0 100"]),
    "http2-hpack-tablesize-2026": ("1", ["2"]),
    "hoverfly-diff-mode-2026": ("1 1", ["0 1", "0 0"]),
    "gotenberg-downloadfrom-2026": ("1 2", ["0 4", "1 1", "0 0"]),
    "gorest-2fa-secret-2026": ("1 2", ["0 2", "1 1", "0 0"]),
    "mailpit-proxy-assets-2026": ("1 1", ["0 1", "1 0", "0 0"]),
    "olivetin-oauth-state-2026": ("1 2", ["0 2", "1 1", "0 0"]),
    "free5gc-bsf-put-2026": ("0", ["1"]),
}


def go_case(case):
    return {
        "mode": "stdin",
        "target": [str(GO / case / "harness/myprog-symqemu")],
        "oracle": [str(E / "bin" / f"{case}-race-stdin")],
        "oracle_env": {"GORACE": "halt_on_error=1 exitcode=66"},
        "replays": 1,
        "canon": canon,
        "join": b" ",
    }


def seeds():
    out = []
    for case, (trig, benign) in GO_SEEDS.items():
        out.append((case, "trigger", trig.encode(), go_case(case)))
        for b in benign:
            out.append((case, "benign", b.encode(), go_case(case)))
    # -cpu qemu64 hides AVX and CLMUL from glibc and liblzma's runtime
    # dispatch; with the default CPU model SymQEMU crashes (QEMU internal
    # SIGSEGV) right after the first block header.
    xz = lambda t: {
        "mode": "file",
        "qemu_args": ["-cpu", "qemu64"],
        "target": [str(E / "bin/xz-mt-decoder-plain"), t],
        "oracle": [str(E / "bin/xz-mt-decoder-tsan"), t],
        "oracle_env": {"TSAN_OPTIONS": "halt_on_error=1 exitcode=66"},
        "replays": 10,
    }
    out.append(("xz-mt-decoder-2026", "trigger", XZIN / "multi-64k-4KiB.xz", xz("4")))
    out.append(("xz-mt-decoder-2026", "benign", XZIN / "multi-64k-4KiB.xz", xz("1")))
    out.append(("xz-mt-decoder-2026", "benign", XZIN / "oneblock-4KiB-crc32.xz", xz("4")))
    rocks = {
        "mode": "stdin",
        "target": [str(E / "bin/rocksdb-inplace-update-stdin-plain")],
        "oracle": [str(E / "bin/rocksdb-inplace-update-stdin-tsan")],
        "oracle_env": {"TSAN_OPTIONS": "halt_on_error=1 exitcode=66"},
        "replays": 2,
        "canon": lambda b: (b[:2],),
        "join": b"",
    }
    for label, s in (("trigger", b"11"), ("benign", b"10"), ("benign", b"01")):
        out.append(("rocksdb-inplace-update-2024", label, s, rocks))
    return out


def canon(b):
    """The os.Args fields that harness/symstdin.go derives from stdin."""
    return tuple(f for f in re.split(rb"[ \n]", b) if f)


def seed_name(seed):
    if isinstance(seed, Path):
        return seed.name
    return seed.decode().replace(" ", "_")


def show(seed):
    return seed.name if isinstance(seed, Path) else seed.decode()


def run(cmd, env, stdin_bytes, timeout, log):
    log.write(f"$ {' '.join(f'{k}={shlex.quote(v)}' for k, v in env.items())} "
              f"{shlex.join(cmd)}{' < input' if stdin_bytes is not None else ''}\n")
    log.flush()
    t = time.time()
    try:
        p = subprocess.run(cmd, input=stdin_bytes if stdin_bytes is not None else b"",
                           env={**os.environ, **env}, stdout=subprocess.PIPE,
                           stderr=subprocess.STDOUT, timeout=timeout)
        rc, out = p.returncode, p.stdout
    except subprocess.TimeoutExpired as e:
        rc, out = "timeout", e.stdout or b""
    return rc, out, time.time() - t


def explore(case, label, seed, cfg, args):
    name = f"{case}_{label}_{seed_name(seed)}"
    work = E / "symqemu-output" / name
    shutil.rmtree(work, ignore_errors=True)
    work.mkdir(parents=True)
    logp = E / "logs" / f"symqemu_{name}.log"
    log = logp.open("w")
    lock = threading.Lock()
    data = seed.read_bytes() if isinstance(seed, Path) else seed
    key = cfg.get("canon", lambda b: hashlib.sha1(b).hexdigest())
    seed_key = key(data)

    def replay(k, b):
        stdin, cmd = b, list(cfg["oracle"])
        if cfg["mode"] == "stdin":
            stdin = cfg["join"].join(k)
        else:
            f = work / f"replay-{k[:12]}.xz"
            f.write_bytes(b)
            cmd, stdin = cmd + [str(f)], None
        for _ in range(cfg["replays"]):
            buf = io.StringIO()
            rc, _, _ = run(cmd, cfg["oracle_env"], stdin, args.replay_timeout, buf)
            with lock:
                log.write(buf.getvalue().rstrip("\n") + f"  # replay exit={rc}\n")
            if rc == 66:
                return k
        return None

    pool = cf.ThreadPoolExecutor(args.replay_jobs)
    pending = [pool.submit(replay, seed_key, data)]
    racy = []

    def collect(wait):
        nonlocal pending
        done = [f for f in pending if wait or f.done()]
        pending = [f for f in pending if f not in done]
        racy.extend(k for k in (f.result() for f in done if not f.cancelled()) if k is not None)
        return any(k != seed_key for k in racy)

    seen = {seed_key: data}
    frontier = collections.deque([(0, data)])
    execs, codes, sym_s, start = 0, [], 0.0, time.time()
    depth_max, stop = 0, "frontier-empty"
    while frontier and execs < args.max_execs:
        if collect(False):
            stop = "racy-input-found"
            break
        left = args.budget - sym_s
        if left <= 5:
            stop = "budget"
            break
        depth, inp = frontier.popleft()
        depth_max = max(depth_max, depth)
        out = work / f"e{execs:05d}"
        out.mkdir()
        env = {"SYMCC_OUTPUT_DIR": str(out)}
        cmd = [str(QEMU)] + cfg.get("qemu_args", []) + cfg["target"]
        stdin = inp
        if cfg["mode"] == "file":
            f = work / f"e{execs:05d}.in"
            f.write_bytes(inp)
            env["SYMCC_INPUT_FILE"] = str(f)
            cmd = cmd + [str(f)]
            stdin = None
        buf = io.StringIO()
        rc, txt, dt = run(cmd, env, stdin, min(args.exec_timeout, left), buf)
        sym_s += dt
        codes.append(rc)
        new = []
        for tc in sorted(out.iterdir()):
            b = tc.read_bytes()
            k = key(b)
            if k not in seen:
                seen[k] = b
                frontier.append((depth + 1, b))
                pending.append(pool.submit(replay, k, b))
                new.append(repr(k) if cfg["mode"] == "stdin" else k[:12])
        shutil.rmtree(out)
        tail = txt.decode(errors="replace").splitlines()
        tail = [l for l in tail if not l.startswith(("[STAT]", "[INFO] New testcase"))][-6:]
        with lock:
            log.write(buf.getvalue())
            log.write(f"# exec {execs} depth {depth}: exit={rc} time={dt:.1f}s new_inputs={len(new)}\n")
            for l in tail:
                log.write(f"#   {l}\n")
            if new:
                log.write(f"#   new: {' '.join(new)}\n")
        execs += 1
    if stop != "racy-input-found":
        if collect(True):
            stop = "racy-input-found"
    else:
        for f in pending:
            f.cancel()
        collect(True)
    pool.shutdown()
    with lock:
        log.write(f"# exploration stopped ({stop}): {execs} executions, {sym_s:.0f}s in "
                  f"SymQEMU, {time.time() - start:.0f}s wall, {len(frontier)} inputs unexplored\n")
    racy_new = [k for k in racy if k != seed_key]
    crashes = sum(1 for c in codes if c not in (0, 1, "timeout"))
    timeouts = sum(1 for c in codes if c == "timeout")
    shown = [cfg["join"].join(k).decode(errors="replace") if cfg["mode"] == "stdin" else k[:12]
             for k in racy_new[:8]]
    row = {
        "case": case, "row": label, "seed": show(seed), "stop": stop,
        "max_depth": depth_max, "executions": execs, "crashes": crashes,
        "timeouts": timeouts, "symqemu_s": round(sym_s), "wall_s": round(time.time() - start),
        "unique_inputs": len(seen) - 1, "seed_racy": seed_key in racy,
        "racy_generated": len(racy_new), "racy_examples": shown,
        "exit_codes": sorted({str(c) for c in codes}),
        "log": str(logp.relative_to(E)),
    }
    log.write("# result: " + json.dumps(row) + "\n")
    log.close()
    shutil.rmtree(work)
    return row


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("cases", nargs="*")
    ap.add_argument("--jobs", type=int, default=4)
    ap.add_argument("--budget", type=int, default=600, help="exploration seconds per seed")
    ap.add_argument("--max-execs", type=int, default=100000)
    ap.add_argument("--replay-jobs", type=int, default=3)
    ap.add_argument("--exec-timeout", type=int, default=300)
    ap.add_argument("--replay-timeout", type=int, default=120)
    args = ap.parse_args()
    todo = [s for s in seeds() if not args.cases or s[0] in args.cases]
    (E / "logs").mkdir(exist_ok=True)
    results = E / "symqemu-results.jsonl"
    with cf.ThreadPoolExecutor(args.jobs) as pool, results.open("a") as out:
        futs = {pool.submit(explore, *s, args): s for s in todo}
        for f in cf.as_completed(futs):
            row = f.result()
            out.write(json.dumps(row) + "\n")
            out.flush()
            print(json.dumps(row), flush=True)


if __name__ == "__main__":
    main()
