# Nomad Test Certificate
Nomad has a built in command to generate certificates for setting up tls encryption.
This will generate valid certificates with default settings if run without any configuration.
The command `nomad tls` is used to generate the test certificates in this directory.
The command is not created with the ability to generate bad certificates so we have to use
[cfssl 1.6.0](https://github.com/cloudflare/cfssl) for that task.

| File                             | Description               |
|----------------------------------|---------------------------|
| `nomad-agent-ca.pem`             | CA certificate            |
| `nomad-agent-ca-key.pem`         | CA Key                    |
| `regionFoo-client-nomad.pem`     | Nomad cert for foo region |
| `regionFoo-client-nomad-key.pem` | Nomad key for foo region  |
| `ca-bad.pem`                     | CA cert for bad region    |
| `ca-key-bad.pem`                 | CA key for bad region     |
| `nomad-bad.pem`                  | Nomad cert for bad region |
| `nomad-bad-key.pem`              | Nomad key for bad region  |
| `global-*.pem`                   | For global region         |

## Generating self-signed certs with nomad tls
```sh

# Generate CA certificate and key.
nomad tls ca create

# Generate certificates and keys with default values.
# 1. Generate server certificate with default values
# 2. Generate client certificate with default values
nomad tls cert create -server
nomad tls cert create -client

# Generate certificates and keys for region regionFoo.
# 1. Generate server certificate for region regionFoo
# 2. Generate client certificate for region regionFoo
nomad tls cert create -server -region regionFoo
nomad tls cert create -client -region regionFoo
```

## Generating bad self-signed certs with cfssl

```sh
# Write defaults and update.
# NOTE: this doesn't need to be run if regenerating old certificates and
# shouldn't as it overrides non-default values.
cfssl print-defaults csr > ca-csr.json
cfssl print-defaults csr > ca-bad-csr.json
cfssl print-defaults config > ca-config.json

# Generate CA certificate and key.
#
# 1. Generates ca-bad.csr, ca-bad.pem, and ca-bad-key.pem.
cfssl gencert -loglevel=5 -config ca-config.json -initca ca-bad-csr.json | cfssljson -bare ca-bad -

# Generate certificate and key.
#
# 1. Generates nomad-bad.csr, nomad-bad.pem, and nomad-bad-key.pem.
cfssl gencert -loglevel=5 -ca ca-bad.pem -ca-key ca-bad-key.pem -config ca-config.json nomad-bad-csr.json | cfssljson -bare nomad-bad
```