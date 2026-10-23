# Hoverfly Diff Mode Races (CVE-2026-50013)

Data races in Hoverfly, an HTTP API simulator written in Go. In Diff mode,
Hoverfly forwards each request to the real server and compares the response with
the simulated one. Every proxied request is served by its own goroutine, but all
goroutines share one `DiffMode` value and one `responsesDiff` map.

**Project**: [SpectoLabs/hoverfly](https://github.com/SpectoLabs/hoverfly)  
**Vulnerable version**: `v1.12.7`  
**Fix commit**: [`cb576ef7`](https://github.com/SpectoLabs/hoverfly/commit/cb576ef7393b58b109fbb90f834c7ba8df795031) "fix(diff): guard responsesDiff with RWMutex"  
**Advisory**: [GHSA-qrh4-p6v4-mrfg](https://github.com/SpectoLabs/hoverfly/security/advisories/GHSA-qrh4-p6v4-mrfg)  
**Fixed version**: `v1.12.8` (for the map race only)

## Gate

The gates are nested:

1. Diff mode alone makes concurrent requests write shared `DiffMode` fields
   (`core/modes/diff_mode.go:49`, `this.DiffReport = ...`, and line 68,
   `this.Arguments.Headers`);
2. Diff mode plus a real response that differs from the simulation also runs
   `addEntry` (line 109) and `Hoverfly.AddDiff`, which writes the
   `responsesDiff` map without a lock. This map race is the CVE.

In Simulate mode, Hoverfly never contacts the real server and none of these
writes happen.

The fix guards only the map with `responsesDiffMu`. The races on the shared
`DiffMode` fields remain in v1.12.8. The v1.12.15 source still writes
`this.DiffReport` on the shared value, but we did not run v1.12.15 because it
requires Go 1.27.1.

## Harness

The harness uses the upstream Hoverfly core. It loads one simulated pair
(`GET /bomb` returns `expected body`), sets the mode, starts the proxy and sends
32 concurrent requests through it to a local server.

```text
Usage: ./hoverfly-diff-mode <diff> <differs>
  <diff>     1 sets Diff mode, 0 sets Simulate mode
  <differs>  1 makes the real server answer differently from the simulation,
             0 makes it answer exactly as simulated
```

```bash
cd harness
go build -race -o hoverfly-diff-mode .

# Trigger for the CVE map race.
./hoverfly-diff-mode 1 1

# Diff mode only: shared DiffMode field races, no map race.
./hoverfly-diff-mode 1 0

# Escape.
./hoverfly-diff-mode 0 1
```

For symbolic or binary tools, build without the race detector:

```bash
go build -gcflags="all=-N -l" -o hoverfly-diff-mode .
```

## Observed results

Observed with `go -race` on Go 1.26 on Linux x86-64.

| Hoverfly | Input | Races per run | Locations |
|---|---|---|---|
| v1.12.7 | `1 1` | 4 to 7; one run aborted with "concurrent map writes" | `AddDiff` map, `diff_mode.go:49`, `diff_mode.go:109` |
| v1.12.7 | `1 0` | 1 | `diff_mode.go:49`, `diff_mode.go:68` |
| v1.12.7 | `0 1` | 0 | |
| v1.12.8 | `1 1` | map race gone | `diff_mode.go:49`, `diff_mode.go:109` |
| v1.12.8 | `1 0` | | `diff_mode.go:49`, `diff_mode.go:68` |
