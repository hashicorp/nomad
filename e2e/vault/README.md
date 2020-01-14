# Vault Integration Test

Downloads, caches, and tests Nomad against open source Vault binaries. Runs
only when `NOMAD_E2E` is set.

Run with:

```
NOMAD_E2E=1 go test
```

**Warning: Downloads a lot of Vault versions!**
