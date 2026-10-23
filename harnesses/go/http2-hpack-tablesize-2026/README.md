# Go HTTP/2 HPACK Table-Size Race (CVE-2026-97032)

Race in Go's HTTP/2 server. The server writes response headers from one
goroutine while another goroutine processes a client `SETTINGS` frame that
changes the HPACK encoder table size.

**Project**: [golang/go](https://github.com/golang/go)  
**Affected package**: `net/http` HTTP/2 server  
**Affected releases**: Go releases before the CVE-2026-97032 fixes  
**Fix issue**: [go.dev/issue/81867](https://go.dev/issue/81867)  
**Fix CL**: [go.dev/cl/847188](https://go.dev/cl/847188)

## Gate

The race needs a specific client input: a `SETTINGS` frame with setting id `1`,
`SETTINGS_HEADER_TABLE_SIZE`, interleaved with requests that cause response
headers to be encoded. Other settings do not resize the HPACK encoder and are
escapes for this harness.

## Harness

The harness starts a cleartext HTTP/2 server on localhost, sends repeated
requests and inserts one client setting between each request.

```bash
cd harness
go build -race -o http2-hpack-tablesize .

# Trigger: setting id 1 is SETTINGS_HEADER_TABLE_SIZE.
./http2-hpack-tablesize 1

# Escape: setting id 2 is SETTINGS_ENABLE_PUSH.
./http2-hpack-tablesize 2
```

With a vulnerable Go toolchain, `go -race` reports races for input `1` and
prints `OK` for input `2`.

For symbolic or binary tools, build without the race detector:

```bash
go build -gcflags="all=-N -l" -o http2-hpack-tablesize .
```
