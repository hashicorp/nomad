# Nomad Test Certificate

Nomad has a built in command to generate certificates for setting up tls encryption.
This will generate valid certificates with default settings if run without any configuration.
The command `nomad tls` is used to generate the test certificates in this directory.

| File                             | Description               |
|----------------------------------|---------------------------|
| `nomad-agent-ca.pem`             | CA certificate            |
| `nomad-agent-ca-key.pem`         | CA Key                    |
| `regionFoo-client-nomad.pem`     | Nomad cert for foo region |
| `regionFoo-client-nomad-key.pem` | Nomad key for foo region  |
| `bad-agent-ca.pem`               | CA cert for bad region    |
| `bad-agent-ca-key.pem`           | CA key for bad region     |
| `badRegion-client-bad.pem`       | Nomad cert for bad region |
| `badRegion-client-bad-key.pem`   | Nomad key for bad region  |
| `global-*.pem`                   | For global region         |
| `whitespace-agent-ca.pem`        | For whitespace test       |

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


## Generating additional self-signed certs for testing tls misconfiguration 

These certificates are used to test incorrect tls configuration.
They are valid certificates but issued from a different CA

```sh

# Generate CA certificate and key.
nomad tls ca create -name-constraint=true -domain bad

# Generate certificates and keys for region badRegion.
# 1. Generate server certificate for region badRegion
# 2. Generate client certificate for region badRegion
nomad tls cert create -server -region badRegion -domain=bad
nomad tls cert create -client -region badRegion -domain=bad
```

## Generate CA for whitespace test

You will need to edit the pem file to add some whitespace after the 
-----END CERTIFICATE----- line 

```sh

# Generate CA certificate and key.
nomad tls ca create -name-constraint=true -domain whitespace
```