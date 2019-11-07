# Nomad Test Certificate

Using [cfssl 1.2.0](https://github.com/cloudflare/cfssl)

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
# Write defaults and update
cfssl print-defaults csr > ca-csr.json
cfssl print-defaults config > ca-config.json

# Generate CA certificate and key
cfssl gencert -config ca-config.json -initca ca-csr.json | cfssljson -bare ca -

# Generate Nomad certificate and key
cfssl gencert -ca ca.pem -ca-key ca-key.pem -config ca-config.json nomad-foo-csr.json | cfssljson -bare nomad-foo

# Generate bad region CA and certificate
cfssl gencert -config ca-config.json -initca ca-bad-csr.json | cfssljson -bare ca-bad -
cfssl gencert -ca ca-bad.pem -ca-key ca-bad-key.pem -config ca-config.json nomad-bad-csr.json | cfssljson -bare nomad-bad
```
