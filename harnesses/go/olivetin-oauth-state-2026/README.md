# OliveTin OAuth2 State Map Race (CVE-2026-28789)

Race in OliveTin's OAuth2 login flow. The OAuth2 handler stores login state in a
shared `registeredStates` map. Concurrent `/oauth/login` and `/oauth/callback`
traffic can read and write this map without synchronization, causing Go's
runtime to abort the process.

**Project**: [OliveTin/OliveTin](https://github.com/OliveTin/OliveTin)  
**Affected versions**: before `3000.10.3` / commit `f044d90d5525c`  
**Fixed version**: `3000.10.3`  
**Advisory**: [GHSA-45m3-398w-m2m9](https://github.com/advisories/GHSA-45m3-398w-m2m9)

## Gate

The race needs both of these input conditions:

1. OAuth2 authentication is enabled;
2. at least two OAuth2 requests run concurrently, such as parallel login
   requests that both register state.

Deployments without OAuth2 enabled skip this state map. A single login request
does not race with itself.

## Harness

The harness extracts the vulnerable state-map access pattern into a small
program.

```bash
cd harness
go build -race -o olivetin-oauth-state .

# Trigger: OAuth2 enabled and concurrent login requests.
./olivetin-oauth-state 1 2

# Escapes.
./olivetin-oauth-state 0 2
./olivetin-oauth-state 1 1
```

For symbolic or binary tools, build without the race detector:

```bash
go build -gcflags="all=-N -l" -o olivetin-oauth-state .
```
