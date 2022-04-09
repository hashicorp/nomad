# Nomad Test Certificate

Using [cfssl 1.6.0](https://github.com/cloudflare/cfssl)

| File                | Description               |
|---------------------|---------------------------|
| `ca.pem`            | CA certificate            |
| `ca-key.pem`        | CA Key                    |
| `nomad-foo.pem`     | Nomad cert for foo region |
| `nomad-foo-key.pem` | Nomad key for foo region  |
| `ca-bad.pem`        | CA cert for bad region    |
| `ca-key-bad.pem`    | CA key for bad region     |
| `nomad-bad.pem`     | Nomad cert for bad region |
| `nomad-bad-key.pem` | Nomad key for bad region  |
| `global-*.pem`      | For global region         |

## Generating self-signed certs
```sh
# Write defaults and update.
# NOTE: this doesn't need to be run if regenerating old certificates and
# shouldn't as it overrides non-default values.
cfssl print-defaults csr > ca-csr.json
cfssl print-defaults csr > ca-bad-csr.json
cfssl print-defaults config > ca-config.json

# Generate CA certificates and keys.
#
# 1. Generates ca.csr, ca.pem, and ca-key.pem.
# 2. Generates ca-bad.csr, ca-bad.pem, and ca-bad-key.pem.
cfssl gencert -loglevel=5 -config ca-config.json -initca ca-csr.json | cfssljson -bare ca -
cfssl gencert -loglevel=5 -config ca-config.json -initca ca-bad-csr.json | cfssljson -bare ca-bad -

# Generate certificates and keys.
#
# 1. Generates nomad-foo.csr, nomad-foo.pem, and nomad-foo-key.pem.
# 1. Generates nomad-bad.csr, nomad-bad.pem, and nomad-bad-key.pem.
cfssl gencert -loglevel=5 -ca ca.pem -ca-key ca-key.pem -config ca-config.json nomad-foo-csr.json | cfssljson -bare nomad-foo
cfssl gencert -loglevel=5 -ca ca-bad.pem -ca-key ca-bad-key.pem -config ca-config.json nomad-bad-csr.json | cfssljson -bare nomad-bad
```
