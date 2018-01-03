---
layout: "guides"
page_title: "Securing Nomad with TLS"
sidebar_current: "guides-securing-nomad"
description: |-
  Securing Nomad's cluster communication with TLS is important for both
  security and easing operations. Nomad can use mutual TLS (mTLS) for
  authenticating for all HTTP and RPC communication.
---

# Securing Nomad with TLS

Securing Nomad's cluster communication is not only important for security but
can even ease operations by preventing mistakes and misconfigurations. Nomad
optionally uses mutual [TLS][tls] (mTLS) for all HTTP and RPC communication.
Nomad's use of mTLS provides the following properties:

* Prevent unauthorized Nomad access
* Prevent observing or tampering with Nomad communication
* Prevent client/server role or region misconfigurations
* Prevent other services from masquerading as Nomad agents

Preventing region misconfigurations is a property of Nomad's mTLS not commonly
found in the TLS implementations on the public Internet.  While most uses of
TLS verify the identity of the server you are connecting to based on a domain
name such as `example.com`, Nomad verifies the node you are connecting to is in
the expected region and configured for the expected role (e.g.
`client.us-west.nomad`). This also prevents other services who may have access
to certificates signed by the same private CA from masquerading as Nomad
agents. If certificates were identified based on hostname/IP then any other
service on a host could masquerade as a Nomad agent.

Correctly configuring TLS can be a complex process, especially given the wide
range of deployment methodologies. If you use the sample
[Vagrantfile][vagrantfile] from the [Getting Started Guide][guide-install] - or
have [cfssl][cfssl] and Nomad installed - this guide will provide you with a
production ready TLS configuration.

~> Note that while Nomad's TLS configuration will be production ready, key
   management and rotation is a complex subject not covered by this guide.
   [Vault][vault] is the suggested solution for key generation and management.

## Creating Certificates

The first step to configuring TLS for Nomad is generating certificates. In
order to prevent unauthorized cluster access, Nomad requires all certificates
be signed by the same Certificate Authority (CA). This should be a _private_ CA
and not a public one like [Let's Encrypt][letsencrypt] as any certificate
signed by this CA will be allowed to communicate with the cluster.

~> Nomad certificates may be signed by intermediate CAs as long as the root CA
   is the same. Append all intermediate CAs to the `cert_file`.

### Certificate Authority

There are a variety of tools for managing your own CA, [like the PKI secret
backend in Vault][vault-pki], but for the sake of simplicity this guide will
use [cfssl][cfssl]. You can generate a private CA certificate and key with
[cfssl][cfssl]:

```shell
$ # Generate the CA's private key and certificate
$ cfssl print-defaults csr | cfssl gencert -initca - | cfssljson -bare nomad-ca
```

The CA key (`nomad-ca-key.pem`) will be used to sign certificates for Nomad
nodes and must be kept private. The CA certificate (`nomad-ca.pem`) contains
the public key necessary to validate Nomad certificates and therefore must be
distributed to every node that requires access.

### Node Certificates

Once you have a CA certificate and key you can generate and sign the
certificates Nomad will use directly. TLS certificates commonly use the
fully-qualified domain name of the system being identified as the certificate's
Common Name (CN). However, hosts (and therefore hostnames and IPs) are often
ephemeral in Nomad clusters.  Not only would signing a new certificate per
Nomad node be difficult, but using a hostname provides no security or
functional benefits to Nomad. To fulfill the desired security properties
(above) Nomad certificates are signed with their region and role such as:

* `client.global.nomad` for a client node in the `global` region
* `server.us-west.nomad` for a server node in the `us-west` region

To create certificates for the client and server in the cluster from the
[Getting Started guide][guide-cluster] with [cfssl][cfssl] create ([or
download][cfssl.json]) the following configuration file as `cfssl.json` to
increase the default certificate expiration time:

```json
{
  "signing": {
    "default": {
      "expiry": "87600h",
      "usages": [
        "signing",
        "key encipherment",
        "server auth",
        "client auth"
      ]
    }
  }
}
```

```shell
$ # Generate a certificate for the Nomad server
$ echo '{}' | cfssl gencert -ca=nomad-ca.pem -ca-key=nomad-ca-key.pem -config=cfssl.json \
    -hostname="server.global.nomad,localhost,127.0.0.1" - | cfssljson -bare server

# Generate a certificate for the Nomad client
$ echo '{}' | cfssl gencert -ca=nomad-ca.pem -ca-key=nomad-ca-key.pem -config=cfssl.json \
    -hostname="client.global.nomad,localhost,127.0.0.1" - | cfssljson -bare client

# Generate a certificate for the CLI
$ echo '{}' | cfssl gencert -ca=nomad-ca.pem -ca-key=nomad-ca-key.pem -profile=client \
    - | cfssljson -bare cli
```

Using `localhost` and `127.0.0.1` as subject alternate names (SANs) allows
tools like `curl` to be able to communicate with Nomad's HTTP API when run on
the same host. Other SANs may be added including a DNS resolvable hostname to
allow remote HTTP requests from third party tools.

You should now have the following files:

* `cfssl.json` - cfssl configuration.
* `nomad-ca.csr` - CA signing request.
* `nomad-ca-key.pem` - CA private key. Keep safe!
* `nomad-ca.pem` - CA public certificate.
* `cli.csr` - Nomad CLI certificate signing request.
* `cli-key.pem` - Nomad CLI private key.
* `cli.pem` - Nomad CLI certificate.
* `client.csr` - Nomad client node certificate signing request for the `global` region.
* `client-key.pem` - Nomad client node private key for the `global` region.
* `client.pem` - Nomad client node public certificate for the `global` region.
* `server.csr` - Nomad server node certificate signing request for the `global` region.
* `server-key.pem` - Nomad server node private key for the `global` region.
* `server.pem` - Nomad server node public certificate for the `global` region.

Each Nomad node should have the appropriate key (`-key.pem`) and certificate
(`.pem`) file for its region and role. In addition each node needs the CA's
public certificate (`nomad-ca.pem`).

## Configuring Nomad

Next Nomad must be configured to use the newly-created key and certificates for
mTLS. Starting with the [server configuration from the Getting Started
guide][guide-server] add the following TLS configuration options:

```hcl
# Increase log verbosity
log_level = "DEBUG"

# Setup data dir
data_dir = "/tmp/server1"

# Enable the server
server {
  enabled = true

  # Self-elect, should be 3 or 5 for production
  bootstrap_expect = 1
}

# Require TLS
tls {
  http = true
  rpc  = true

  ca_file   = "nomad-ca.pem"
  cert_file = "server.pem"
  key_file  = "server-key.pem"

  verify_server_hostname = true
  verify_https_client    = true
}
```

The new [`tls`][tls_block] section is worth breaking down in more detail:

```hcl
tls {
  http = true
  rpc  = true
  # ...
}
```

This enables TLS for the HTTP and RPC protocols. Unlike web servers, Nomad
doesn't use separate ports for TLS and non-TLS traffic: your cluster should
either use TLS or not.

```hcl
tls {
  # ...

  ca_file   = "nomad-ca.pem"
  cert_file = "server.pem"
  key_file  = "server-key.pem"

  # ...
}
```

The file lines should point to wherever you placed the certificate files on
the node. This guide assumes they are in Nomad's current directory.

```hcl
tls {
  # ...

  verify_server_hostname = true
  verify_https_client    = true
}
```

These two settings are important for ensuring all of Nomad's mTLS security
properties are met. If [`verify_server_hostname`][verify_server_hostname] is
set to `false` the node's certificate will be checked to ensure it is signed by
the same CA, but its role and region will not be verified. This means any
service with a certificate signed by same CA as Nomad can act as a client or
server of any region.

[`verify_https_client`][verify_https_client] requires HTTP API clients to
present a certificate signed by the same CA as Nomad's certificate. It may be
disabled to allow HTTP API clients (e.g. Nomad CLI, Consul, or curl) to
communicate with the HTTPS API without presenting a client-side certificate. If
`verify_https_client` is enabled only HTTP API clients presenting a certificate
signed by the same CA as Nomad's certificate are allowed to access Nomad.

~> Enabling `verify_https_client` effectively protects Nomad from unauthorized
   network access at the cost of losing Consul HTTPS health checks for agents.

### Client Configuration

The Nomad client configuration is similar to the server configuration. The
biggest difference is in the certificate and key used for configuration.

```hcl
# Increase log verbosity
log_level = "DEBUG"

# Setup data dir
data_dir = "/tmp/client1"

# Enable the client
client {
  enabled = true

  # For demo assume we are talking to server1. For production,
  # this should be like "nomad.service.consul:4647" and a system
  # like Consul used for service discovery.
  servers = ["127.0.0.1:4647"]
}

# Modify our port to avoid a collision with server1
ports {
  http = 5656
}

# Require TLS
tls {
  http = true
  rpc  = true

  ca_file   = "nomad-ca.pem"
  cert_file = "client.pem"
  key_file  = "client-key.pem"

  verify_server_hostname = true
  verify_https_client    = true
}
```

### Running with TLS

Now that we have certificates generated and configuration for a client and
server we can test our TLS-enabled cluster!

In separate terminals start a server and client agent:

```shell
$ # In one terminal...
$ nomad agent -config server1.hcl

$ # ...and in another
$ nomad agent -config client1.hcl
```

If you run `nomad node-status` now, you'll get an error, like:

```text
Error querying node status: Get http://127.0.0.1:4646/v1/nodes: malformed HTTP response "\x15\x03\x01\x00\x02\x02"
```

This is because the Nomad CLI defaults to communicating via HTTP instead of
HTTPS. We can configure the local Nomad client to connect using TLS and specify
our custom keys and certificates using the command line:

```shell
$ nomad node-status -ca-cert=nomad-ca.pem -client-cert=cli.pem -client-key=cli-key.pem -address=https://127.0.0.1:4646
```

This process can be cumbersome to type each time, so the Nomad CLI also
searches environment variables for default values. Set the following
environment variables in your shell:

```shell
$ export NOMAD_ADDR=https://localhost:4646
$ export NOMAD_CACERT=nomad-ca.pem
$ export NOMAD_CLIENT_CERT=cli.pem
$ export NOMAD_CLIENT_KEY=cli-key.pem
```

* `NOMAD_ADDR` is the URL of the Nomad agent and sets the default for `-addr`.
* `NOMAD_CACERT` is the location of your CA certificate and sets the default
  for `-ca-cert`.
* `NOMAD_CLIENT_CERT` is the location of your CLI certificate and sets the
  default for `-client-cert`.
* `NOMAD_CLIENT_KEY` is the location of your CLI key and sets the default for
  `-client-key`.

After these environment variables are correctly configured, the CLI will
respond as expected:

```text
$ nomad node-status
ID        DC   Name   Class   Drain  Status
237cd4c5  dc1  nomad  <none>  false  ready

$ nomad init
Example job file written to example.nomad
vagrant@nomad:~$ nomad run example.nomad
==> Monitoring evaluation "e9970e1d"
    Evaluation triggered by job "example"
    Allocation "a1f6c3e7" created: node "237cd4c5", group "cache"
    Evaluation within deployment: "080460ce"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "e9970e1d" finished with status "complete"
```

## Server Gossip

At this point all of Nomad's RPC and HTTP communication is secured with mTLS.
However, Nomad servers also communicate with a gossip protocol, Serf, that does
not use TLS:

* HTTP - Used to communicate between CLI and Nomad agents. Secured by mTLS.
* RPC - Used to communicate between Nomad agents. Secured by mTLS.
* Serf - Used to communicate between Nomad servers. Secured by a shared key.

Nomad server's gossip protocol use a shared key instead of TLS for encryption.
This encryption key must be added to every server's configuration using the
[`encrypt`](/docs/agent/configuration/server.html#encrypt) parameter or with
the [`-encrypt` command line option](/docs/commands/agent.html).

The Nomad CLI includes a `keygen` command for generating a new secure gossip
encryption key:

```text
$ nomad keygen
cg8StVXbQJ0gPvMd9o7yrg==
```

Alternatively, you can use any method that base64 encodes 16 random bytes:

```text
$ openssl rand -base64 16
raZjciP8vikXng2S5X0m9w==
$ dd if=/dev/urandom bs=16 count=1 status=none | base64
LsuYyj93KVfT3pAJPMMCgA==
```

Put the same generated key into every server's configuration file or command
line arguments:

```hcl
server {
  enabled = true

  # Self-elect, should be 3 or 5 for production
  bootstrap_expect = 1

  # Encrypt gossip communication
  encrypt = "cg8StVXbQJ0gPvMd9o7yrg=="
}
```

## Switching an existing cluster to TLS

Since Nomad does _not_ use different ports for TLS and non-TLS communication,
the use of TLS must be consistent across the cluster. Switching an existing
cluster to use TLS everywhere is operationally similar to upgrading between
versions of Nomad, but requires additional steps to preventing needlessly
rescheduling allocations.

1. Add the appropriate key and certificates to all nodes.
  * Ensure the private key file is only readable by the Nomad user.
1. Add the environment variables to all nodes where the CLI is used.
1. Add the appropriate [`tls`][tls_block] block to the configuration file on
   all nodes.
1. Generate a gossip key and add it the Nomad server configuration.

~> Once a quorum of servers are TLS-enabled, clients will no longer be able to
   communicate with the servers until their client configuration is updated and
   reloaded.

At this point a rolling restart of the cluster will enable TLS everywhere.
However, once servers are restarted clients will be unable to heartbeat. This
means any client unable to restart with TLS enabled before their heartbeat TTL
expires will have their allocations marked as `lost` and rescheduled.

While the default heartbeat settings may be sufficient for concurrently
restarting a small number of nodes without any allocations being marked as
`lost`, most operators should raise the [`heartbeat_grace`][heartbeat_grace]
configuration setting before restarting their servers:

1. Set `heartbeat_grace = "1h"` or an appropriate duration on servers.
1. Restart servers, one at a time.
1. Restart clients, one or more at a time.
1. Set [`heartbeat_grace`][heartbeat_grace] back to its previous value (or
   remove to accept the default).
1. Restart servers, one at a time.

~> In a future release Nomad will allow upgrading a cluster to use TLS by
   allowing servers to accept TLS and non-TLS connections from clients during
   the migration.

Jobs running in the cluster will _not_ be affected and will continue running
throughout the switch as long as all clients can restart within their heartbeat
TTL.

## Changing Nomad certificates on the fly

As of 0.7.1, Nomad supports dynamic certificate reloading via SIGHUP.

Given a prior TLS configuration as follows:

```hcl
tls {
  http = true
  rpc  = true

  ca_file   = "nomad-ca.pem"
  cert_file = "server.pem"
  key_file  = "server-key.pem"

  verify_server_hostname = true
  verify_https_client    = true
}
```

Nomad's cert_file and key_file can be reloaded via SIGHUP simply by
updating the TLS stanza to:

```hcl
tls {
  http = true
  rpc  = true

  ca_file   = "nomad-ca.pem"
  cert_file = "new_server.pem"
  key_file  = "new_server_key.pem"

  verify_server_hostname = true
  verify_https_client    = true
}
```

NOTE: Dynamically reloading certificates will _not_ close existing connections.
If you need to rotate certificates due to a security incident, you will still
need to completely shutdown and restart the Nomad agent.


[cfssl]: https://cfssl.org/
[cfssl.json]: https://raw.githubusercontent.com/hashicorp/nomad/master/demo/vagrant/cfssl.json
[guide-install]: https://www.nomadproject.io/intro/getting-started/install.html
[guide-cluster]: https://www.nomadproject.io/intro/getting-started/cluster.html
[guide-server]: https://raw.githubusercontent.com/hashicorp/nomad/master/demo/vagrant/server.hcl
[heartbeat_grace]: /docs/agent/configuration/server.html#heartbeat_grace
[letsencrypt]: https://letsencrypt.org/
[tls]: https://en.wikipedia.org/wiki/Transport_Layer_Security
[tls_block]: /docs/agent/configuration/tls.html
[vagrantfile]: https://raw.githubusercontent.com/hashicorp/nomad/master/demo/vagrant/Vagrantfile
[vault]: https://www.vaultproject.io/
[vault-pki]: https://www.vaultproject.io/docs/secrets/pki/index.html
[verify_https_client]: /docs/agent/configuration/tls.html#verify_https_client
[verify_server_hostname]: /docs/agent/configuration/tls.html#verify_server_hostname
