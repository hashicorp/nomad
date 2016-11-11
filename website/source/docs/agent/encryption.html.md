---
layout: "docs"
page_title: "Gossip and RPC Encryption"
sidebar_current: "docs-agent-encryption"
description: |-
  Learn how to configure Nomad to encrypt all of its traffic.
---

# Encryption

The Nomad agent supports encrypting all of its network traffic. There are
two separate encryption systems, one for gossip traffic, and one for HTTP and
RPC.

## Gossip

Enabling gossip encryption only requires that you set an encryption key when
starting the Nomad server. The key can be set via the
[`encrypt`](/docs/agent/configuration/server.html#encrypt) parameter: the value
of this setting is a server configuration file containing the encryption key.

The key must be 16-bytes, base64 encoded. As a convenience, Nomad provides the
[`nomad keygen`](/docs/commands/keygen.html) command to generate a cryptographically suitable key:

```shell
$ nomad keygen
cg8StVXbQJ0gPvMd9o7yrg==
```

With that key, you can enable gossip encryption on the agent.


## HTTP, RPC, and Raft Encryption with TLS

Nomad supports using TLS to verify the authenticity of servers and clients. To
enable this, Nomad requires that all clients and servers have key pairs that are
generated and signed by a Certificate Authority. This can be a private CA.

TLS can be used to verify the authenticity of the servers and clients. The
configuration option [`verify_server_hostname`][tls] causes Nomad to verify that
a certificate is provided that is signed by the Certificate Authority from the
[`ca_file`][tls] for TLS connections.

If `verify_server_hostname` is set, then outgoing connections perform
hostname verification. Unlike traditional HTTPS browser validation, all servers
must have a certificate valid for `server.<region>.nomad` or the client will
reject the handshake. It is also recommended for the certificate to sign
`localhost` such that the CLI can validate the server name.

TLS is used to secure the RPC calls between agents, but gossip between nodes is
done over UDP and is secured using a symmetric key. See above for enabling
gossip encryption.

[tls]: /docs/agent/configuration/tls.html "Nomad TLS Configuration"

### Example TLS Configuration using cfssl

While [Vault's PKI backend][vault] is an ideal solution for managing
certificates and other secrets in a production environment, it's useful to use
simpler command line tools when learning how to configure TLS and your [PKI].

[`cfssl`][cfssl] is a tool for working with TLS certificates and certificate
authorities similar to [OpenSSL's][openssl] `x509` command line tool.

Once you have the `cfssl` command line tool installed create, the first step to
setting up TLS is to create a Certificate Authority (CA) certificate.  The
following command will generate a suitable example CA CSR, certificate, and
key:

```sh
# Run in the directory where you want to store certificates

cfssl print-defaults csr | cfssl gencert -initca - | cfssljson -bare ca
```

Next create a `nomad-csr.json` which contains the configuration for the actual
certificate you'll be using in Nomad:

```json
{
    "CN": "regionglobal.nomad",
    "hosts": [
	"server.regionglobal.nomad",
	"client.regionglobal.nomad"
    ]
}
```

This will create a certificate suitable for both clients and servers in the
`global` (default) region.

In production Nomad agents should have a certificate valid for the name
`${ROLE}.region${REGION}.nomad` where role is either `client` or `server`
depending on the node's role.

Create a certificate signed by your CA:

```sh
cfssl gencert -ca ca.pem -ca-key ca-key.pem nomad-csr.json | cfssljson -bare nomad
```

You've now successfully generated self-signed certificates! You should see the
following files:

| File            | Description                  | Usage                     |
|-----------------|------------------------------|---------------------------|
| `ca.pem`        | CA certificate               | `ca_file` setting         |
| `ca-key.pem`    | CA private key               | Signing CSRs              |
| `nomad.pem`     | Nomad cert for global region | `cert_file` setting       |
| `nomad-key.pem` | Nomad key for foo region     | `key_file` setting        |
| `*.csr`      | Certificate Signing Requests | Generating certs (temporary) |

In your Nomad configuration add the `tls` stanza:

```hcl
tls {
  http = true
  rpc  = true
  verify_server_hostname = true
  ca_file   = "ca.pem"
  cert_file = "nomad.pem"
  key_file  = "nomad-key.pem"
}
```

[vault]: https://www.vaultproject.io/docs/secrets/pki/
[PKI]: https://en.wikipedia.org/wiki/Public_key_infrastructure
[cfssl]: https://cfssl.org/
[openssl]: https://www.openssl.org/
