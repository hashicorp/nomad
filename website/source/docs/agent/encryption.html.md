---
layout: "docs"
page_title: "Gossip and RPC Encryption"
sidebar_current: "docs-agent-encryption"
description: |-
  Learn how to configure Nomad to encrypt both its gossip traffic and its RPC
  traffic.
---

# Encryption

The Nomad agent supports encrypting all of its network traffic. There are
two separate encryption systems, one for gossip traffic, and one for RPC.

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


## RPC and Raft Encryption with TLS

Nomad supports using TLS to verify the authenticity of servers and clients. To
enable this, Nomad requires that all clients and servers have key pairs that are
generated and signed by a Certificate Authority. This can be a private CA.

TLS can be used to verify the authenticity of the servers and clients. The
configuration option [`verify_server_hostname`][tls] causes Nomad to verify that
a certificate is provided that is signed by the Certificate Authority from the
[`ca_file`][tls] for TLS connections.

If `verify_server_hostname` is set, then outgoing connections perform
hostname verification. All servers must have a certificate valid for
`server.<region>.nomad` or the client will reject the handshake. It is also
recommended for the certificate to sign `localhost` such that the CLI can
validate the server name.

TLS is used to secure the RPC calls between agents, but gossip between nodes is
done over UDP and is secured using a symmetric key. See above for enabling
gossip encryption.

[tls]: /docs/agent/configuration/tls.html "Nomad TLS Configuration"
