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
[Getting Started guide's Vagrantfile][Vagrantfile] or have [OpenSSL][] and Nomad
installed this guide will provide you with a production ready TLS
configuration.

~> Note that while Nomad's TLS configuration will be production ready, key
   management and rotation is a complex subject not covered by this guide.
   [Vault][] is the suggested solution for key generation and management.

XXX TODO XXX - serf encryption key

## Creating Certificates

The first step to configuring TLS for Nomad is generating certificates. In
order to prevent unauthorized cluster access, Nomad requires all certificates
are signed by the sign Certificate Authority (CA). This should be a *private*
CA and not a public like [Let's Encrypt][letsencrypt] as any certificate signed
by this CA will be allowed to communicate with the cluster.

### Certificate Authority

You can generate a private CA certificate and key with OpenSSL:

```shell
# Generate the CA's private key
# This file (nomad-ca.key) must be kept *secret*
openssl genrsa -out nomad-ca.key 4096

# Generate the CA's self-signed certicate
# This file (nomad-ca.crt) will be distributed to all nodes
openssl req -new -x509 -key nomad-ca.key -out nomad-ca.crt
You are about to be asked to enter information that will be incorporated
into your certificate request.
What you are about to enter is what is called a Distinguished Name or a DN.
There are quite a few fields but you can leave some blank
For some fields there will be a default value,
If you enter '.', the field will be left blank.
-----
Country Name (2 letter code) [AU]:.
State or Province Name (full name) [Some-State]:.
Locality Name (eg, city) []:.
Organization Name (eg, company) [Internet Widgits Pty Ltd]:.
Organizational Unit Name (eg, section) []:.
Common Name (e.g. server FQDN or YOUR name) []:Nomad CA
Email Address []:.
```

Your answers to OpenSSL's prompts are purely informational and not used by
Nomad.

The CA key (`nomad-ca.key`) will be used to sign certificates for Nomad nodes
and must be kept private. The CA certificate (`nomad-ca.crt`) contains the
public key necessary to validate Nomad certificates and therefore must be
distributed to every node that requires access.

### Node Certificates

Once you have a CA certifacte and key you can generate and sign the
certificates Nomad will use directly. Traditionally TLS certificates use the
fully-qualified domain name of the system being identified as the certificate's
Common Name (CN). However, hosts (and therefore hostnames and IPs) are often
ephemeral in Nomad clusters. They come and go as clusters are scaled up and
down or outages occur. Not only would signing a new certificate per Nomad node
be difficult, but using a hostname provides no security or functional benefits
to Nomad. To fulfill the desired security properties (see above) Nomad
certificates are signed with their region and role such as:

* `client.global.nomad` for a client node in the `global` region
* `server.us-west.nomad` for a server node in the `us-west` region

To create certificates for the client and server in the cluster from the
[Getting Started guide][guide-cluster] with OpenSSL create the following
configuration file `nomad.conf`:

```ini
basicConstraints = CA:FALSE
subjectAltName = @alt_names

[alt_names]
DNS.1 = ${commonName}
DNS.2 = localhost
```

Using `localhost` as a subject alternate name (SAN) allows tools like `curl` to
be able to communicate with Nomad's HTTP API when run on the same host. Other
SANs may be added including a DNS resolvable hostname to allow remote HTTP
requests from third party tools.

Then create client and server certificate and key pairs:

```shell
# Client key and certificate
openssl genrsa -out client.global.nomad.key 4096
openssl req -new -sha256 \
            -out client.global.nomad.csr \
            -key client.global.nomad.key \
            -subj /CN=client.global.nomad/
openssl x509 -req \
             -in client.global.nomad.csr \
             -CA nomad-ca.crt \
             -CAkey nomad-ca.key \
             -days 3650 \
             -set_serial $(hexdump -e '"0x%x%x%x%x"' -n 16 /dev/urandom) \
             -extfile nomad.conf \
             -out client.global.nomad.crt

# Server key and certificate
openssl genrsa -out server.global.nomad.key 4096
openssl req -new -sha256 \
            -out server.global.nomad.csr \
            -key server.global.nomad.key \
            -subj /CN=server.global.nomad/
openssl x509 -req \
             -in server.global.nomad.csr \
             -CA nomad-ca.crt \
             -CAkey nomad-ca.key \
             -days 3650 \
             -set_serial $(hexdump -e '"0x%x%x%x%x"' -n 16 /dev/urandom) \
             -extfile nomad.conf \
             -out server.global.nomad.crt
```

You should now have the following files:

* `nomad-ca.key` - CA private key. Keep safe!
* `nomad-ca.crt` - CA public certificate.
* `client.global.nomad.key` - Nomad client node private key for the `global` region.
* `client.global.nomad.csr` - Nomad client node certificate signing request for the `global` region.
* `client.global.nomad.crt` - Nomad client node public certificate for the `global` region.
* `server.global.nomad.key` - Nomad server node private key for the `global` region.
* `server.global.nomad.csr` - Nomad server node certificate signing request for the `global` region.
* `server.global.nomad.crt` - Nomad server node public certificate for the `global` region.

Each Nomad node should have the appropriate key (`.key`) and certificate
(`.crt`) file for its region and role. In addition each node needs the CA's
public certificate (`nomad-ca.crt`).

## Configuring Nomad

Once you have the appropriate key and certificates installed you're ready to
configure Nomad to use them for mTLS. Starting with the [server configuration
from the Getting Started guide][guide-server] add the following TLS specific
configuration options:

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

  ca_file   = "nomad-ca.crt"
  cert_file = "server.global.nomad.crt"
  key_file  = "server.global.nomad.key"

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
  ca_file   = "nomad-ca.crt"
  cert_file = "server.global.nomad.crt"
  key_file  = "server.global.nomad.key"
```

The file lines should point to whereever you placed the certificate files on
the node. This guide assumes they're in Nomad's current directory.

```hcl
  verify_server_hostname = true
  verify_https_client    = true
```

These two settings are important for ensuring all of Nomad's mTLS security
properties are met. If `verify_server_hostname` is set to `false` the node's
cerificate will be checked to ensure it is signed by the same CA, but its role
and region will not be verified. This means any service with a certificate from
the same CA as Nomad can act as a client or server of any region.

`verify_https_client` may be disabled to allow HTTP API clients (eg Nomad CLI, Consul, or
curl) to communicate with the HTTPS API without presenting a client-side
certificate. If `verify_https_client` is enabled ony HTTP API clients
presenting a certificate signed by the same CA as Nomad's certificate are
allowed to access Nomad.

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

  ca_file   = "nomad-ca.crt"
  cert_file = "client.global.nomad.crt"
  key_file  = "client.global.nomad.key"

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
export NOMAD_CACERT=nomad-ca.crt
export NOMAD_CLIENT_CERT=client.global.nomad.crt
```

The `NOMAD_CACERT` also needs to be set so the CLI can verify it's talking to
an actual Nomad node. Finally, the `NOMAD_CLIENT_CERT` needs to be set since we
enabled `verify_https_client` above which prevents any access lacking a client
certificate. Operators may wish to generate a certificate specifically for the
CLI as any certificate signed by Nomad's CA will work.

XXX TODO XXX - an example of everything working

## Switching an existing cluster to TLS

XXX TODO XXX

[guide-server]: https://raw.githubusercontent.com/hashicorp/nomad/master/demo/vagrant/server.hcl
[guide-cluster]: https://www.nomadproject.io/intro/getting-started/cluster.html
[letsencrypt]: https://letsencrypt.org/
[OpenSSL]: https://www.openssl.org/
[Vagrantfile]: https://raw.githubusercontent.com/hashicorp/nomad/master/demo/vagrant/Vagrantfile
[Vault]: https://www.vaultproject.io/docs/secrets/pki/index.html
