---
layout: "guides"
page_title: "Encryption Overview"
sidebar_current: "guides-security-encryption"
description: |-
  Learn how to configure Nomad to encrypt HTTP, RPC, and Serf traffic.
---

# Encryption Overview

The Nomad agent supports encrypting all of its network traffic. There are
two separate encryption systems, one for gossip traffic, and one for HTTP and
RPC.

## Gossip

Enabling gossip encryption only requires that you set an encryption key when
starting the Nomad server. The key can be set via the
[`encrypt`](/docs/configuration/server.html#encrypt) parameter: the value
of this setting is a server configuration file containing the encryption key.

The key must be 16 bytes, base64 encoded. As a convenience, Nomad provides the
[`nomad operator keygen`](/docs/commands/operator/keygen.html) command to
generate a cryptographically suitable key:

```sh
$ nomad operator keygen
cg8StVXbQJ0gPvMd9o7yrg==
```

With that key, you can enable gossip encryption on the agent.


## HTTP, RPC, and Raft Encryption with TLS

Nomad supports using TLS to verify the authenticity of servers and clients. To
enable this, Nomad requires that all clients and servers have key pairs that are
generated and signed by a private Certificate Authority (CA).

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

### Configuring the command line tool

If you have HTTPS enabled for your Nomad agent, you must export environment
variables for the command line tool to also use HTTPS:

```sh
# NOMAD_ADDR defaults to http://, so set it to https
# Alternatively you can use the -address flag
export NOMAD_ADDR=https://127.0.0.1:4646

# Set the location of your CA certificate
# Alternatively you can use the -ca-cert flag
export NOMAD_CACERT=/path/to/ca.pem
```

Run any command except `agent` with `-h` to see all environment variables and
flags. For example: `nomad status -h`

By default HTTPS does not validate client certificates, so you do not need to
give the command line tool access to any private keys.

### Network Isolation with TLS

If you want to isolate Nomad agents on a network with TLS you need to enable
both [`verify_https_client`][tls] and [`verify_server_hostname`][tls]. This
will cause agents to require client certificates for all incoming HTTPS
connections as well as verify proper names on all other certificates.

Consul will not attempt to health check agents with `verify_https_client` set
as it is unable to use client certificates.

# Configuring Nomad with TLS

Read the [Securing Nomad with TLS Guide][guide] for details on how to configure
encryption for Nomad.

[guide]: /guides/security/securing-nomad.html "Securing Nomad with TLS"
[tls]: /docs/configuration/tls.html "Nomad TLS Configuration"
