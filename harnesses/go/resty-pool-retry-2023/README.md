# go-resty Pooled Body Retry Race (CVE-2023-45286)

Race in `github.com/go-resty/resty/v2` v2.10.0. A retry can release the same
request body buffer to a package-level `sync.Pool` twice. Later concurrent
requests can then receive the same buffer, which can disclose one request body
to another request.

**Project**: [go-resty/resty](https://github.com/go-resty/resty)  
**Vulnerable version**: `v2.10.0`  
**Fix commit**: [`577fed8`](https://github.com/go-resty/resty/commit/577fed8730d79f583eb48dfc81674164e1fc471e)  
**Advisory**: [GO-2023-2328](https://pkg.go.dev/vuln/GO-2023-2328)

## Gate

The race needs both of these input conditions:

1. the client is configured with at least one retry;
2. the first response to a request has status `>= 500`, so the retry path runs.

Without a retry, the pooled buffer is not released twice. Without a retryable
response, the retry path is not taken.

## Harness

The harness starts a local HTTP server and sends concurrent Resty requests. It
prints `LEAK` when the server receives a body containing more than one request
marker. `go -race` also reports data races on the shared buffer.

```bash
cd harness
go build -race -o resty-pool-retry .

# Trigger: retry enabled, retryable response.
./resty-pool-retry 1 503

# Escapes.
./resty-pool-retry 0 503
./resty-pool-retry 1 200
```

For symbolic or binary tools, build without the race detector:

```bash
go build -gcflags="all=-N -l" -o resty-pool-retry .
```
