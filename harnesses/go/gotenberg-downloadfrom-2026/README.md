# Gotenberg `downloadFrom` Map Race (CVE-2026-45742)

Race in Gotenberg's multipart request handling. When `downloadFrom` is enabled,
the request parser starts one goroutine per `downloadFrom` entry. Those
goroutines write shared request maps without synchronization.

**Project**: [gotenberg/gotenberg](https://github.com/gotenberg/gotenberg)  
**Affected versions**: `v8.10.0` through `v8.32.x`  
**Fixed version**: `v8.33.0`  
**Advisory**: [GHSA-vp73-vjw8-8f32](https://github.com/gotenberg/gotenberg/security/advisories/GHSA-vp73-vjw8-8f32)

## Gate

The race needs both of these input conditions:

1. the `downloadFrom` feature is enabled;
2. the multipart request contains at least two `downloadFrom` entries.

With one entry there is only one writer. With the feature disabled the vulnerable
branch is skipped.

## Harness

The harness extracts the vulnerable request-map update pattern into a small
program with two command-line inputs.

```bash
cd harness
go build -race -o gotenberg-downloadfrom .

# Trigger: feature enabled, two entries.
./gotenberg-downloadfrom 1 2

# Escapes.
./gotenberg-downloadfrom 1 1
./gotenberg-downloadfrom 0 4
```

For symbolic or binary tools, build without the race detector:

```bash
go build -gcflags="all=-N -l" -o gotenberg-downloadfrom .
```
