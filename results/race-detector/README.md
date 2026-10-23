# Race detector on trigger, escape and seed inputs

Ground truth for Table II: each harness built with `go build -race` (Go) or
`-fsanitize=thread` (C, C++), and run with
`GORACE`/`TSAN_OPTIONS="halt_on_error=1 exitcode=66"`.

- `go-race-summary.tsv`: the nine Go harnesses.
- `tsan-summary.tsv`: liblzma (C) and RocksDB (C++).
- `logs/`: one log per run; its first lines are the commands.

Inputs: the documented trigger and escapes of each harness (see its README),
and the all-ones seed (`1`, `1 1`, or `1 111` for go-resty, whose second
argument is an HTTP status). The all-ones seed is itself a trigger for HTTP/2,
Hoverfly, Mailpit and RocksDB.

`result` is `race` when the detector reported a data race (exit 66), `clean`
otherwise. net/http's race does not reproduce: Go 1.26.5 contains its fix.
