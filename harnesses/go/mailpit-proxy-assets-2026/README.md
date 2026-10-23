# Mailpit Proxy Asset Map Race (CVE-2026-45712)

Race in Mailpit's screenshot/print proxy. The `/proxy?data=...` path keeps a
package-level `assets` map for message assets. The CSS rewriting path reads that
map without holding `assetsMutex`, while cache and cleanup paths write the same
map under the mutex. A concurrent read and write can terminate the whole
process with Go's unrecoverable concurrent-map runtime error.

**Project**: [axllent/mailpit](https://github.com/axllent/mailpit)  
**Affected versions**: before `1.30.0`  
**Fixed version**: `1.30.0`  
**Advisory**: [GHSA-w4vj-r5pg-3722](https://github.com/axllent/mailpit/security/advisories/GHSA-w4vj-r5pg-3722)

## Gate

The race needs both of these input conditions:

1. a request reaches the screenshot/print proxy path;
2. the proxied message has a stylesheet link, so CSS asset rewriting touches the
   shared asset cache.

Requests outside the proxy path do not use the asset map. Proxy requests for
messages without stylesheets skip the CSS rewriting path.

## Harness

The harness models one proxy request stream and one cache/cleanup writer.

```bash
cd harness
go build -race -o mailpit-proxy-assets .

# Trigger: proxy path and CSS rewriting enabled.
./mailpit-proxy-assets 1 1

# Escapes.
./mailpit-proxy-assets 0 1
./mailpit-proxy-assets 1 0
```

For symbolic or binary tools, build without the race detector:

```bash
go build -gcflags="all=-N -l" -o mailpit-proxy-assets .
```
