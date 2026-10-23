# Go `net/http` `Expect: 100-continue` Race (CVE-2020-15586)

Race in Go's HTTP server. On affected Go releases, the server can write
`100 Continue` to the connection when the Handler first reads the request body.
If the Handler reads the body while writing a response, the implicit
`100 Continue` write can race with the Handler's response write.

**Project**: [golang/go](https://github.com/golang/go)  
**Affected package**: `net/http`  
**Fixed versions**: Go `1.13.13` and Go `1.14.5`  
**Fix issue**: [go.dev/issue/34902](https://go.dev/issue/34902)  
**Fix commit**: [`fa98f46`](https://go.googlesource.com/go/+/fa98f46741f818913a8c11b877520a548715131f)

## Gate

The race needs all of these request conditions:

1. the request has `Expect: 100-continue`;
2. the request is HTTP/1.1 or later;
3. the request has a non-empty body.

The vulnerable server path is guarded by these checks in `net/http/server.go` on
affected releases.

## Harness

The harness starts a local HTTP server whose Handler reads the request body in a
goroutine while writing a response.

```bash
cd harness
go build -race -o nethttp-expect-continue .

# Trigger on affected Go releases: Expect header and non-empty body.
./nethttp-expect-continue 1 1

# Escapes.
./nethttp-expect-continue 0 1
./nethttp-expect-continue 1 0
```

Current Go releases contain the fix, so the trigger prints `OK` under the current
toolchain. To reproduce the race with `go -race`, build and run the harness with
an affected Go release.

For symbolic or binary tools, build without the race detector:

```bash
go build -gcflags="all=-N -l" -o nethttp-expect-continue .
```
