# free5GC BSF Subscription Map Race (CVE-2026-44318)

Race in free5GC's Binding Support Function (BSF). The PUT handler for
`/nbsf-management/v1/subscriptions/{subId}` first checks the global
`Subscriptions` map under `RLock()`. If `subId` is absent, the handler writes a
new subscription directly to the same map without taking the mutex.

**Project**: [free5gc/bsf](https://github.com/free5gc/bsf)  
**Affected product version**: free5GC before `4.2.2`  
**Vulnerable BSF tag**: `v1.0.1`  
**Fix commit**: [`2779085`](https://github.com/free5gc/bsf/commit/277908565fd628d974a13ef562b81a8b7b519ffa)  
**Advisory**: [GHSA-27ph-8q4f-h7m7](https://github.com/free5gc/free5gc/security/advisories/GHSA-27ph-8q4f-h7m7)

## Gate

The race needs the PUT request to take the create-if-absent branch:

1. concurrent PUT requests target subscription IDs that do not already exist;
2. the BSF handler parses a valid subscription body and writes the new entry.

When the subscription already exists, the handler uses the locked update helper
instead of the unlocked direct map write.

## Harness

The harness imports free5GC BSF internal packages, so `setup.sh` copies it into
an upstream checkout where those imports are legal.

```bash
./setup.sh v1.0.1 bsf-v1.0.1
cd bsf-v1.0.1
go build -race -o free5gc-bsf-put ./cmd/bombharness

# Trigger: fresh subIds take the unlocked create-if-absent branch.
./free5gc-bsf-put 0

# Escape: existing subIds take the locked update branch.
./free5gc-bsf-put 1
```

For symbolic or binary tools, build without the race detector:

```bash
go build -gcflags="all=-N -l" -o free5gc-bsf-put ./cmd/bombharness
```
