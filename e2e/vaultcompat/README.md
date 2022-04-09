# Vault Integration Test

Not run as part of nightly e2e suite at this point.

Downloads, caches, and tests Nomad against open source Vault binaries. Runs
only when `-integration` is set.

Run with:

```
cd e2e/vault/
go test -integration
```

**Warning: Downloads a lot of Vault versions!**
