---
layout: "guides"
page_title: "Securing Nomad with TLS"
sidebar_current: "guides-tls"
description: |-
  Securing Nomad's cluster communication with TLS is XXX TODO XXX
---

# Securing Nomad with TLS

Securing Nomad's cluster communication is not only important for security but
can even ease operations by preventing mistakes and misconfigurations. Nomad
optionally uses mutual TLS (mTLS) for all HTTP and RPC communication. Nomad's
use of mTLS provides the following properties:

* Prevent unauthorized Nomad access
* Prevent observing or tampering with Nomad communication
* Prevent client/server role or region misconfigurations

The 3rd property is fairly unique to Nomad's use of TLS. While most uses of TLS
verify the identity of the server you're connecting to based on a domain name
such as `nomadproject.io`, Nomad verifies the node you're connecting to is in
the expected region and configured for the expected role (e.g.
`client.us-west.nomad`).

Configuring TLS can be unfortunately complex process, but if you used the
[Getting Started guide's Vagrantfile][Vagrantfile] or have [cfssl][] and Nomad
installed this guide will provide you with a production ready TLS
configuration.

~> Note that while Nomad's TLS configuration will be production ready, key
   management and rotation is a complex subject not covered by this guide.
   [Vault][] is the suggested solution for key generation and management.

XXX TODO XXX - serf encryption key

## Creating Certificates

The first step to configuring TLS for Nomad is generating certificates. In
order to prevent unauthorized cluster access, Nomad requires all certificates
be signed by the same Certificate Authority (CA). This should be a *private* CA
and not a public one like [Let's Encrypt][letsencrypt] as any certificate
signed by this CA will be allowed to communicate with the cluster.

### Certificate Authority

There are a variety of tools for managing your own CA, [like the PKI secret
backend in Vault][vault-pki], but for the sake of simplicity in this guide
we'll use [cfssl][]. You can generate a private CA certificate and key with
[cfssl][]:

```shell
# Generate the CA's private key and certificate
cfssl print-defaults csr | cfssl gencert -initca - | cfssljson -bare nomad-ca
```

The CA key (`nomad-ca-key.pem`) will be used to sign certificates for Nomad
nodes and must be kept private. The CA certificate (`nomad-ca.pem`) contains
the public key necessary to validate Nomad certificates and therefore must be
distributed to every node that requires access.

### Node Certificates

Once you have a CA certifacte and key you can generate and sign the
certificates Nomad will use directly. TLS certificates commonly use the
fully-qualified domain name of the system being identified as the certificate's
Common Name (CN). However, hosts (and therefore hostnames and IPs) are often
ephemeral in Nomad clusters. They come and go as clusters are scaled up and
down or outages occur. Not only would signing a new certificate per Nomad node
be difficult, but using a hostname provides no security or functional benefits
to Nomad. To fulfill the desired security properties (above) Nomad certificates
are signed with their region and role such as:

* `client.global.nomad` for a client node in the `global` region
* `server.us-west.nomad` for a server node in the `us-west` region

To create certificates for the client and server in the cluster from the
[Getting Started guide][guide-cluster] with [cfssl][] create ([or
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
# Generate a certificate for the Nomad server
echo '{}' | cfssl gencert -ca=nomad-ca.pem -ca-key=nomad-ca-key.pem -config=cfssl.json \
    -hostname="server.global.nomad,localhost" - | cfssljson -bare server

# Generate a certificate for the Nomad client
echo '{}' | cfssl gencert -ca=nomad-ca.pem -ca-key=nomad-ca-key.pem -config=cfssl.json \
    -hostname="client.global.nomad,localhost" - | cfssljson -bare client

# Generate a certificate for the CLI
echo '{}' | cfssl gencert -ca nomad-ca.pem -ca-key nomad-ca-key.pem -profile=client \
    - | cfssljson -bare cli
```

Using `localhost` as a subject alternate name (SAN) allows tools like `curl` to
be able to communicate with Nomad's HTTP API when run on the same host. Other
SANs may be added including a DNS resolvable hostname to allow remote HTTP
requests from third party tools.

You should now have the following files:

* `cfssl.json` - cfssl configuration.
* `nomad-ca.csr` - CA signing request.
* `nomad-ca-key.pem` - CA private key. Keep safe!
* `nomad-ca.pem` - CA public certificate.
* `cli.csr` - Nomad CLI certificate signing request.
* `cli.pem` - Nomad CLI certificate.
* `cli-key.pem` - Nomad CLI private key.
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

Once you have the appropriate key and certificates installed you're ready to
configure Nomad to use them for mTLS. Starting with the [server configuration
from the Getting Started guide][guide-server] add the following TLS
CONFIGUration options:

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

The new `tls` section is worth breaking down in more detail:

```hcl
  http = true
  rpc  = true
```

This enables TLS for the HTTP and RPC protocols. Unlike web servers, Nomad
doesn't use separate ports for TLS and non-TLS traffic: your cluster should
either use TLS or not.

```hcl
  ca_file   = "nomad-ca.pem"
  cert_file = "server.pem"
  key_file  = "server-key.pem"
```

The file lines should point to whereever you placed the certificate files on
the node. This guide assumes they are in Nomad's current directory.

```hcl
  verify_server_hostname = true
  verify_https_client    = true
```

These two settings are important for ensuring all of Nomad's mTLS security
properties are met. If `verify_server_hostname` is set to `false` the node's
cerificate will be checked to ensure it is signed by the same CA, but its role
and region will not be verified. This means any service with a certificate from
the same CA as Nomad can act as a client or server of any region.

`verify_https_client` requires HTTP API clients to present a certificate signed
by the same CA as Nomad's certificate. It may be disabled to allow HTTP API
clients (eg Nomad CLI, Consul, or curl) to communicate with the HTTPS API
without presenting a client-side certificate. If `verify_https_client` is
enabled ony HTTP API clients presenting a certificate signed by the same CA as
Nomad's certificate are allowed to access Nomad.

~> Enabling `verify_https_client` feature effectively protects Nomad from
   unauthorized network access at the cost of breaking compatibility with Consul
   HTTPS health checks.

### Client configuration

The Nomad client configuration is similar with the only difference being the
certificate and key used:

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
# In one terminal...
nomad agent -config server1.hcl

# ...and in another
nomad agent -config client1.hcl
```

Finally in a third terminal test out `nomad node-status`:

```text
vagrant@nomad:~$ nomad node-status
Error querying node status: Get http://127.0.0.1:4646/v1/nodes: malformed HTTP response "\x15\x03\x01\x00\x02\x02"
```

Oh no! That didn't work!

Don't worry, the Nomad CLI just defaults to `http://...` instead of
`https://...`. We can override this with an environment variable:

```shell
export NOMAD_ADDR=https://localhost:4646
export NOMAD_CACERT=nomad-ca.pem
export NOMAD_CLIENT_CERT=client.pem
export NOMAD_CLIENT_KEY=client-key.pem
```

The `NOMAD_CACERT` also needs to be set so the CLI can verify it's talking to
an actual Nomad node. Finally, `NOMAD_CLIENT_CERT` and `NOMAD_CLIENT_KEY` need
to be set since we enabled `verify_https_client` above which prevents any
access lacking a client certificate.

Now the CLI works as expected:

```text
vagrant@nomad:~$ nomad node-status
ID        DC   Name   Class   Drain  Status
237cd4c5  dc1  nomad  <none>  false  ready

vagrant@nomad:~$ nomad init
Example job file written to example.nomad
vagrant@nomad:~$ nomad run example.nomad
==> Monitoring evaluation "e9970e1d"
    Evaluation triggered by job "example"
    Allocation "a1f6c3e7" created: node "237cd4c5", group "cache"
    Evaluation within deployment: "080460ce"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "e9970e1d" finished with status "complete"
```

## Switching an existing cluster to TLS

XXX TODO XXX

[guide-server]: https://raw.githubusercontent.com/hashicorp/nomad/master/demo/vagrant/server.hcl
[guide-cluster]: https://www.nomadproject.io/intro/getting-started/cluster.html
[letsencrypt]: https://letsencrypt.org/
[cfssl]: https://cfssl.org/
[cfssl.json]: https://raw.githubusercontent.com/hashicorp/nomad/master/demo/vagrant/cfssl.json
[Vagrantfile]: https://raw.githubusercontent.com/hashicorp/nomad/master/demo/vagrant/Vagrantfile
[Vault]: https://www.vaultproject.io/
[vault-pki]: https://www.vaultproject.io/docs/secrets/pki/index.html
