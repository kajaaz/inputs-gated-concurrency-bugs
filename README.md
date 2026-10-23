# Input-gated concurrency bugs: harnesses and evaluation results

Artifact for the paper *The Data Behind the Race: Rethinking How We Separate
Program Inputs from Concurrency Bugs*. It contains the eleven harnesses of
data races fixed in real projects whose advisories or fix commits state an
input condition, and the commands, summaries and full logs of every tool run
reported in the paper.

## Contents

```
harnesses/
  go/<case>/                        nine Go harnesses
  c/xz-mt-decoder-2026/             liblzma (C)
  cpp/rocksdb-inplace-update-2024/  RocksDB (C++)
results/
  race-detector/    go -race and TSan on triggers, escapes and seeds (Table II)
  symqemu/          SymQEMU exploration and replay (Table II)
  zorya/
    benign-seeds/   Zorya from benign seeds (Table II)
    triggers/       Zorya from the triggers (Table II)
    c-cpp/          Zorya on the C and C++ harnesses (n/a in Table II)
  repeat-runs/
    go/             20 runs of each Go trigger, default and GOMAXPROCS=1
    c-cpp/          20 runs of each C/C++ trigger, all CPUs and one CPU
  klee-symcc/       why KLEE and SymCC are not in Table II
```

Each harness README gives the source of the bug, the gate, the trigger and
escape inputs, and the build and run commands. In `results/`, the last column
of each `*.tsv` summary points to the log of the run, relative to the
summary's directory, and each log starts with its command. Logs keep the
absolute paths of the machine where they ran. Binaries, upstream source trees,
generated inputs and Zorya's per-run output directories are not included; the
harness READMEs and `make-inputs.sh` rebuild them.

## Harnesses

| Harness | Language | Bug | Gate |
|---|---|---|---|
| `nethttp-expect-continue-2020` | Go | CVE-2020-15586 | R |
| `resty-pool-retry-2023` | Go | CVE-2023-45286 | R |
| `http2-hpack-tablesize-2026` | Go | CVE-2026-97032 | R |
| `hoverfly-diff-mode-2026` | Go | CVE-2026-50013 | R |
| `gotenberg-downloadfrom-2026` | Go | CVE-2026-45742 | R, P |
| `gorest-2fa-secret-2026` | Go | CVE-2026-48154 | R, P |
| `mailpit-proxy-assets-2026` | Go | CVE-2026-45712 | R |
| `olivetin-oauth-state-2026` | Go | CVE-2026-28789 | R, P |
| `free5gc-bsf-put-2026` | Go | CVE-2026-44318 | R |
| `xz-mt-decoder-2026` | C | xz commit c6e3aad | R, P |
| `rocksdb-inplace-update-2024` | C++ | RocksDB PR 12624 | R, A |

Gate: reachability (R), parallelism (P) or aliasing (A) part, as defined in
the paper. `free5gc-bsf-put-2026/setup.sh` fetches free5GC BSF v1.0.1 and adds
the harness to it.

## Table II

| Harness | Inputs | TSan/go -race race | SymQEMU race | Zorya race | Zorya gate |
|---|---|---|---|---|---|
| net/http | trigger / benign | no / no | no / no | timeout / no | no / no |
| go-resty | trigger / benign | yes / no | yes / yes | timeout / no | no / no |
| HTTP/2 | trigger / benign | yes / no | yes / yes | timeout / no | no / no |
| Hoverfly | trigger / benign | yes / no | yes / yes | timeout / no | no / no |
| Gotenberg | trigger / benign | yes / no | yes / yes | yes / yes | yes / yes |
| gorest | trigger / benign | yes / no | yes / yes | yes / yes | yes / yes |
| Mailpit | trigger / benign | yes / no | yes / yes | yes / yes | yes / yes |
| OliveTin | trigger / benign | yes / no | yes / yes | yes / yes | yes / yes |
| free5GC | trigger / benign | yes / no | yes / yes | timeout / timeout | no / no |
| liblzma | trigger / benign | yes / no | no / no | n/a | n/a |
| RocksDB | trigger / benign | yes / no | yes / yes | n/a | n/a |

Neither the race detectors nor SymQEMU report a gate. Sources:

- TSan/go -race: `results/race-detector/`. net/http's race does not reproduce,
  as Go 1.26.5 contains its fix.
- SymQEMU: `results/symqemu/symqemu-summary.tsv`, with every generated input
  in `symqemu-results.jsonl` and the replay of each racing input in `proofs/`.
  `run-symqemu.py` documents the method: SymQEMU explores from each seed for
  up to 600 s, and each generated input is replayed under the race detector.
- Zorya from benign seeds: `results/zorya/benign-seeds/` (`summary.tsv`;
  `found-seeds.tsv` lists the racing input and the round that found it;
  `long-runs.tsv` the 900 s and 3600 s reruns; the logs contain the gates).
- Zorya from triggers: `results/zorya/triggers/summary.tsv`, and
  `summary-900s.tsv` for the 900 s Gotenberg run.
- Zorya on C/C++: `results/zorya/c-cpp/README.md`.

## Shapes (repeat runs)

- Go: each trigger except net/http's raced in 20 of 20 runs, with the default
  GOMAXPROCS and with GOMAXPROCS=1, except one go-resty run with the default
  (`results/repeat-runs/go/summary.tsv`).
- RocksDB: 20 of 20 under TSan, on all CPUs and pinned to one CPU.
- liblzma: 20 of 20 under TSan on all CPUs, but 1 of 20 pinned to one CPU
  (`results/repeat-runs/c-cpp/`).

## Setup

All runs used one laptop (Intel Core i7-1165G7, no GPU) running Ubuntu Linux,
with Go 1.26.5 (linux/amd64), Ubuntu clang 18.1.3, xz 5.8.3, SymQEMU at commit
6e37dd2c, and Zorya with its data race analysis
(`zorya <bin> --analysis data-race`).
