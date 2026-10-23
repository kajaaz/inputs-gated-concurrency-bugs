# gorest In-Memory 2FA Secret Race (CVE-2026-48154)

Race in `github.com/pilinux/gorest`. The in-memory two-factor-authentication
secret store was a package-level Go map. Several login and 2FA handlers read,
write and delete that map while HTTP requests run concurrently, which can crash
the process with `fatal error: concurrent map read and map write`.

**Project**: [pilinux/gorest](https://github.com/pilinux/gorest)  
**Affected versions**: before `1.12.2`  
**Fixed version**: `1.12.2`  
**Advisory**: [GHSA-cpwg-x64r-rgwg](https://github.com/pilinux/gorest/security/advisories/GHSA-cpwg-x64r-rgwg)

## Gate

The race needs both of these input conditions:

1. the deployment uses the in-memory 2FA secret store;
2. at least two 2FA-related requests run concurrently, such as two logins or a
   login while another user verifies 2FA.

With the in-memory store disabled, this shared map is not touched. With only one
2FA operation, the harness executes the same map operations sequentially.

## Harness

The harness extracts the vulnerable store access pattern into a small program.

```bash
cd harness
go build -race -o gorest-2fa-secret .

# Trigger: in-memory 2FA store and concurrent operations.
./gorest-2fa-secret 1 2

# Escapes.
./gorest-2fa-secret 0 2
./gorest-2fa-secret 1 1
```

For symbolic or binary tools, build without the race detector:

```bash
go build -gcflags="all=-N -l" -o gorest-2fa-secret .
```
