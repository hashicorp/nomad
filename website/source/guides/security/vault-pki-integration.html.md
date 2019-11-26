---
layout: "guides"
page_title: "Vault PKI Secrets Engine Integration"
sidebar_current: "guides-security-vault-pki"
description: |-
  Securing Nomad's cluster communication with TLS is important for both
  security and easing operations. Nomad can use mutual TLS (mTLS) for
  authenticating for all HTTP and RPC communication. This guide will leverage
  Vault's PKI secrets engine to accomplish this task.
---

# Vault PKI Secrets Engine Integration

You can use [Consul Template][consul-template] in your Nomad cluster to
integrate with Vault's [PKI Secrets Engine][pki-engine] to generate and renew
dynamic X.509 certificates. By using this method, you enable each node to have a
unique certificate with a relatively short ttl. This feature, along with
automatic certificate rotation, allows you to safely and securely scale your
cluster while using mutual TLS (mTLS).

## Reference Material

- [Vault PKI Secrets Engine][pki-engine]
- [Consul Template][consul-template-github]
- [Build Your Own Certificate Authority (CA)][vault-ca-learn]

## Estimated Time to Complete

25 minutes

## Challenge

Secure your existing Nomad cluster with mTLS. Configure a root and intermediate
CA in Vault and ensure (with the help of Consul Template) that you are
periodically renewing your X.509 certificates on all nodes to maintain a healthy
cluster state.

## Solution

Enable TLS in your Nomad cluster configuration. Additionally, configure Consul
Template on all nodes along with the appropriate templates to communicate with
Vault and ensure all nodes are dynamically generating/renewing their X.509
certificates.

## Prerequisites

To perform the tasks described in this guide, you need to have a Nomad
environment with Consul and Vault installed. You can use this [repo][repo] to
easily provision a sandbox environment. This guide will assume a cluster with
one server node and three client nodes.

~> **Please Note:** This guide is for demo purposes and is only using a single
Nomad server with a Vault server configured alongside it. In a production
cluster, 3 or 5 Nomad server nodes are recommended along with a separate Vault
cluster. Please see [Vault Reference Architecture][vault-ra] to learn how to
securely deploy a Vault cluster.

## Steps

### Step 1: Initialize Vault Server

Run the following command to initialize Vault server and receive an
[unseal][seal] key and initial root [token][token] (if you are running the
environment provided in this guide, the Vault server is co-located with the
Nomad server). Be sure to note the unseal key and initial root token as you will
need these two pieces of information.

```shell
$ vault operator init -key-shares=1 -key-threshold=1
```

The `vault operator init` command above creates a single Vault unseal key for
convenience. For a production environment, it is recommended that you create at
least five unseal key shares and securely distribute them to independent
operators. The `vault operator init` command defaults to five key shares and a
key threshold of three. If you provisioned more than one server, the others will
become standby nodes but should still be unsealed.

### Step 2: Unseal Vault

Run the following command and then provide your unseal key to Vault.

```shell
$ vault operator unseal
```
The output of unsealing Vault will look similar to the following:

```shell
Key                    Value
---                    -----
Seal Type              shamir
Initialized            true
Sealed                 false
Total Shares           1
Threshold              1
Version                1.0.3
Cluster Name           vault-cluster-d1b6513f
Cluster ID             87d6d13f-4b92-60ce-1f70-41a66412b0f1
HA Enabled             true
HA Cluster             n/a
HA Mode                standby
Active Node Address    <none>
```

### Step 3: Log in to Vault

Use the [login][login] command to authenticate yourself against Vault using the
initial root token you received earlier. You will need to authenticate to run
the necessary commands to write policies, create roles, and configure your root
and intermediate CAs.

```shell
$ vault login <your initial root token>
```
If your login is successful, you will see output similar to what is shown below:

```shell
Success! You are now authenticated. The token information displayed below
is already stored in the token helper. You do NOT need to run "vault login"
again. Future Vault requests will automatically use this token.
...
```

### Step 4: Generate the Root CA

Enable the [PKI secrets engine][pki-engine] at the `pki` path:

```shell
$ vault secrets enable pki
```

Tune the PKI secrets engine to issue certificates with a maximum time-to-live
(TTL) of 87600 hours:

```shell
$ vault secrets tune -max-lease-ttl=87600h pki
```
* Please note: we are using a common and recommended pattern which is to have
  one mount act as the root CA and to use this CA only to sign intermediate CA
  CSRs from other PKI secrets engines (which we will create in the next few
  steps). For tighter security, you can store your CA outside of Vault and use
  the PKI engine only as an intermediate CA.

Generate the root certificate and save the certificate as `CA_cert.crt`:

```shell
$ vault write -field=certificate pki/root/generate/internal \
    common_name="global.nomad" ttl=87600h > CA_cert.crt
```

### Step 5: Generate the Intermediate CA and CSR

Enable the PKI secrets engine at the `pki_int` path:

```shell
$ vault secrets enable -path=pki_int pki
```

Tune the PKI secrets engine at the `pki_int` path to issue certificates with a
maximum time-to-live (TTL) of 43800 hours:

```shell
$ vault secrets tune -max-lease-ttl=43800h pki_int
```
Generate a CSR from your intermediate CA and save it as `pki_intermediate.csr`:

```shell
$ vault write -format=json pki_int/intermediate/generate/internal \
    common_name="global.nomad Intermediate Authority" \
    ttl="43800h" | jq -r '.data.csr' > pki_intermediate.csr
```

### Step 6: Sign the CSR and Configure Intermediate CA Certificate

Sign the intermediate CA CSR with the root certificate and save the generated
certificate as `intermediate.cert.pem`:

```shell
$ vault write -format=json pki/root/sign-intermediate \
    csr=@pki_intermediate.csr format=pem_bundle \
    ttl="43800h" | jq -r '.data.certificate' > intermediate.cert.pem
```

Once the CSR is signed and the root CA returns a certificate, it can be imported
back into Vault:

```shell
vault write pki_int/intermediate/set-signed certificate=@intermediate.cert.pem
```

### Step 7: Create a Role

A role is a logical name that maps to a policy used to generate credentials. In
our example, it will allow you to use [configuration
parameters][config-parameters] that specify certificate common names, designate
alternate names, and enable subdomains along with a few other key settings.

Create a role named `nomad-cluster` that specifies the allowed domains, enables
you to create certificates for subdomains, and generates certificates with a TTL
of 86400 seconds (24 hours).

```
$ vault write pki_int/roles/nomad-cluster allowed_domains=global.nomad \
    allow_subdomains=true max_ttl=86400s require_cn=false generate_lease=true
```
You should see the following output if the command you issues was successful:

```
Success! Data written to: pki_int/roles/nomad-cluster
```

### Step 8: Create a Policy to Access the Role Endpoint

Recall from [Step 1](#step-1-initialize-vault-server) that we generated a root
token that we used to log in to Vault. Although we could use that token in our
next steps to generate our TLS certs, the recommended security approach is to
create a new token based on a specific policy with limited privileges.

Create a policy file named `tls-policy.hcl` and provide it the following
contents:

```
path "pki_int/issue/nomad-cluster" {
  capabilities = ["update"]
}
```
Note that we have are specifying the `update` [capability][capability] on the
path `pki_int/issue/nomad-cluster`. All other privileges will be denied. You can
read more about Vault policies [here][policies].

Write the policy we just created into Vault:

```
$ vault policy write tls-policy tls-policy.hcl
Success! Uploaded policy: tls-policy
```

### Step 9: Generate a Token based on `tls-policy`

Create a token based on `tls-policy` with the following command:

```shell
$ vault token create -policy="tls-policy" -period=24h -orphan
```

If the command is successful, you will see output similar to the following:

```shell
Key                  Value
---                  -----
token                s.m069Vpul3c4lfGnJ6unpxgxD
token_accessor       HiZALO25hDQzSgyaglkzty3M
token_duration       24h
token_renewable      true
token_policies       ["default" "tls-policy"]
identity_policies    []
policies             ["default" "tls-policy"]
```

Make a note of this token as you will need it in the upcoming steps.

### Step 10: Create and Populate the Templates Directory

We need to create templates that Consul Template can use to render the actual
certificates and keys on the nodes in our cluster. In this guide, we will place
these templates in `/opt/nomad/templates`. 

Create a directory called `templates` in `/opt/nomad`:

```shell
$ sudo mkdir /opt/nomad/templates
```

Below are the templates that the Consul Template configuration will use. We will
provide different templates to the nodes depending on whether they are server
nodes or client nodes. All of the nodes will get the CLI templates (since we
want to use the CLI on any of the nodes).

**For Nomad Servers**:

*agent.crt.tpl*:

```
{{ with secret "pki_int/issue/nomad-cluster" "common_name=server.global.nomad" "ttl=24h" "alt_names=localhost" "ip_sans=127.0.0.1"}}
{{ .Data.certificate }}
{{ end }}
```

*agent.key.tpl*:

```
{{ with secret "pki_int/issue/nomad-cluster" "common_name=server.global.nomad" "ttl=24h" "alt_names=localhost" "ip_sans=127.0.0.1"}}
{{ .Data.private_key }}
{{ end }}
```

*ca.crt.tpl*:

```
{{ with secret "pki_int/issue/nomad-cluster" "common_name=server.global.nomad" "ttl=24h"}}
{{ .Data.issuing_ca }}
{{ end }}
```

**For Nomad Clients**:

Replace the word `server` in the `common_name` option in each template with the
word `client`.

*agent.crt.tpl*:

```
{{ with secret "pki_int/issue/nomad-cluster" "common_name=client.global.nomad" "ttl=24h" "alt_names=localhost" "ip_sans=127.0.0.1"}}
{{ .Data.certificate }}
{{ end }}
```

*agent.key.tpl*:

```
{{ with secret "pki_int/issue/nomad-cluster" "common_name=client.global.nomad" "ttl=24h" "alt_names=localhost" "ip_sans=127.0.0.1"}}
{{ .Data.private_key }}
{{ end }}
```

*ca.crt.tpl*:

```
{{ with secret "pki_int/issue/nomad-cluster" "common_name=client.global.nomad" "ttl=24h"}}
{{ .Data.issuing_ca }}
{{ end }}
```

**For Nomad CLI (provide this on all nodes)**:

*cli.crt.tpl*:

```
{{ with secret "pki_int/issue/nomad-cluster" "ttl=24h" }}
{{ .Data.certificate }}
{{ end }}
```

*cli.key.tpl*:

```
{{ with secret "pki_int/issue/nomad-cluster" "ttl=24h" }}
{{ .Data.private_key }}
{{ end }}
```

### Step 11: Configure Consul Template on All Nodes

If you are using the AWS environment provided in this guide, you already have
[Consul Template][consul-template-github] installed on all nodes. If you are
using your own environment, please make sure Consul Template is installed. You
can download it [here][ct-download].

Provide the token you created in [Step
9](#step-9-generate-a-token-based-on-tls-policy) to the Consul Template
configuration file located at `/etc/consul-template.d/consul-template.hcl`. You
will also need to specify the [template stanza][ct-template-stanza] so you can
render each of the following on your nodes at the specified location from the
templates you created in the previous step:

* Node certificate
* Node private key
* CA public certificate

We will also specify the template stanza to create certs and keys from the
templates we previously created for the Nomad CLI (which defaults to HTTP but
will need to use HTTPS once once TLS is enabled in our cluster).

Your `consul-template.hcl` configuration file should look similar to the
following (you will need to provide this to each node in the cluster):

```
# This denotes the start of the configuration section for Vault. All values
# contained in this section pertain to Vault.
vault {
  # This is the address of the Vault leader. The protocol (http(s)) portion
  # of the address is required.
  address      = "http://active.vault.service.consul:8200"

  # This value can also be specified via the environment variable VAULT_TOKEN.
  token        = "s.m069Vpul3c4lfGnJ6unpxgxD"

  # This should also be less than or around 1/3 of your TTL for a predictable
  # behaviour. See https://github.com/hashicorp/vault/issues/3414
  grace        = "1s"

  # This tells Consul Template that the provided token is actually a wrapped
  # token that should be unwrapped using Vault's cubbyhole response wrapping
  # before being used. Please see Vault's cubbyhole response wrapping
  # documentation for more information.
  unwrap_token = false

  # This option tells Consul Template to automatically renew the Vault token
  # given. If you are unfamiliar with Vault's architecture, Vault requires
  # tokens be renewed at some regular interval or they will be revoked. Consul
  # Template will automatically renew the token at half the lease duration of
  # the token. The default value is true, but this option can be disabled if
  # you want to renew the Vault token using an out-of-band process.
  renew_token  = true
}

# This block defines the configuration for connecting to a syslog server for
# logging.
syslog {
  enabled  = true

  # This is the name of the syslog facility to log to.
  facility = "LOCAL5"
}

# This block defines the configuration for a template. Unlike other blocks,
# this block may be specified multiple times to configure multiple templates.
template {
  # This is the source file on disk to use as the input template. This is often
  # called the "Consul Template template". 
  source      = "/opt/nomad/templates/agent.crt.tpl"

  # This is the destination path on disk where the source template will render.
  # If the parent directories do not exist, Consul Template will attempt to
  # create them, unless create_dest_dirs is false.
  destination = "/opt/nomad/agent-certs/agent.crt"

  # This is the permission to render the file. If this option is left
  # unspecified, Consul Template will attempt to match the permissions of the
  # file that already exists at the destination path. If no file exists at that
  # path, the permissions are 0644.
  perms       = 0700

  # This is the optional command to run when the template is rendered. The
  # command will only run if the resulting template changes. 
  command     = "systemctl reload nomad"
}

template {
  source      = "/opt/nomad/templates/agent.key.tpl"
  destination = "/opt/nomad/agent-certs/agent.key"
  perms       = 0700
  command     = "systemctl reload nomad"
}

template {
  source      = "/opt/nomad/templates/ca.crt.tpl"
  destination = "/opt/nomad/agent-certs/ca.crt"
  command     = "systemctl reload nomad"
}

# The following template stanzas are for the CLI certs

template {
  source      = "/opt/nomad/templates/cli.crt.tpl"
  destination = "/opt/nomad/cli-certs/cli.crt"
}

template {
  source      = "/opt/nomad/templates/cli.key.tpl"
  destination = "/opt/nomad/cli-certs/cli.key"
}
```

!> Note: we have hard-coded the token we created into the Consul Template
configuration file. Although we can avoid this by assigning it to the
environment variable `VAULT_TOKEN`, this method can still pose a security
concern. The recommended approach is to securely introduce this token to Consul
Template. To learn how to accomplish this, see [Secure
Introduction][secure-introduction].

* Please also note we have applied file permissions `0700` to the `agent.crt`
  and `agent.key` since only the root user should be able to read those files.
  Any other user using the Nomad CLI will be able to read the CLI certs and key
  that we have created for them along with intermediate CA cert.


### Step 12: Start the Consul Template Service

Start the Consul Template service on each node:

```shell
$ sudo systemctl start consul-template
```
You can quickly confirm the appropriate certs and private keys were generated in
the `destination` directory you specified in your Consul Template configuration
by listing them out:

```
$ ls /opt/nomad/agent-certs/ /opt/nomad/cli-certs/
/opt/nomad/agent-certs/:
agent.crt  agent.key  ca.crt

/opt/nomad/cli-certs/:
cli.crt  cli.key
```

### Step 13: Configure Nomad to Use TLS

Add the following [tls stanza][nomad-tls-stanza] to the configuration of all
Nomad agents (servers and clients) in the cluster (configuration file located at
`/etc/nomad.d/nomad.hcl` in this example):

```hcl
tls {
  http = true
  rpc  = true

  ca_file   = "/opt/nomad/agent-certs/ca.crt"
  cert_file = "/opt/nomad/agent-certs/agent.crt"
  key_file  = "/opt/nomad/agent-certs/agent.key"

  verify_server_hostname = true
  verify_https_client    = true
}
```

Additionally, ensure the [`rpc_upgrade_mode`][rpc-upgrade-mode] option is set to
`true` on your server nodes (this is to ensure the Nomad servers will accept
both TLS and non-TLS connections during the upgrade):

```hcl
rpc_upgrade_mode       = true
```
Reload Nomad's configuration on all nodes:

```shell
$ systemctl reload nomad
```
Once Nomad has been reloaded on all nodes, go back to your server nodes and
change the `rpc_upgrade_mode` option to false (or remove the line since the
option defaults to false) so that your Nomad servers will only accept TLS
connections:

```hcl
rpc_upgrade_mode       = false
```
You will need to reload Nomad on your servers after changing this setting. You
can read more about RPC Upgrade Mode [here][rpc-upgrade].

If you run `nomad status`, you will now receive the following error:

```
Error querying jobs: Get http://172.31.52.215:4646/v1/jobs: net/http: HTTP/1.x transport connection broken: malformed HTTP response "\x15\x03\x01\x00\x02\x02"
```

This is because the Nomad CLI defaults to communicating via HTTP instead of
HTTPS. We can configure the local Nomad client to connect using TLS and specify
our custom key and certificates by setting the following environments variables:

```shell
export NOMAD_ADDR=https://localhost:4646
export NOMAD_CACERT="/opt/nomad/agent-certs/ca.crt"
export NOMAD_CLIENT_CERT="/opt/nomad/cli-certs/cli.crt"
export NOMAD_CLIENT_KEY="/opt/nomad/cli-certs/cli.key"
```

After these environment variables are correctly configured, the CLI will respond
as expected:

```shell
$ nomad status
No running jobs
```

## Encrypt Server Gossip

At this point all of Nomad's RPC and HTTP communication is secured with mTLS.
However, Nomad servers also communicate with a gossip protocol, Serf, that does
not use TLS:

* HTTP - Used to communicate between CLI and Nomad agents. Secured by mTLS.
* RPC - Used to communicate between Nomad agents. Secured by mTLS.
* Serf - Used to communicate between Nomad servers. Secured by a shared key.

Nomad server's gossip protocol use a shared key instead of TLS for encryption.
This encryption key must be added to every server's configuration using the
[`encrypt`](/docs/configuration/server.html#encrypt) parameter or with the
[`-encrypt` command line option](/docs/commands/agent.html).

The Nomad CLI includes a `operator keygen` command for generating a new secure
gossip encryption key:

```shell
$ nomad operator keygen
cg8StVXbQJ0gPvMd9o7yrg==
```

Alternatively, you can use any method that base64 encodes 16 random bytes:

```shell
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

Unlike with TLS, reloading Nomad will not be enough to initiate encryption of
gossip traffic. At this point, you may restart each Nomad server with `systemctl
restart nomad`.

[capability]: https://www.vaultproject.io/docs/concepts/policies.html#capabilities
[config-parameters]: https://www.vaultproject.io/api/secret/pki/index.html#parameters-8
[consul-template]: https://www.consul.io/docs/guides/consul-template.html
[consul-template-github]: https://github.com/hashicorp/consul-template
[ct-download]: https://releases.hashicorp.com/consul-template/
[ct-template-stanza]: https://github.com/hashicorp/consul-template#configuration-file-format
[login]: https://www.vaultproject.io/docs/commands/login.html
[nomad-tls-stanza]: https://www.nomadproject.io/docs/configuration/tls.html
[policies]: https://www.vaultproject.io/docs/concepts/policies.html#policies
[pki-engine]: https://www.vaultproject.io/docs/secrets/pki/index.html
[repo]: https://github.com/hashicorp/nomad/tree/master/terraform
[rpc-upgrade-mode]: /docs/configuration/tls.html#rpc_upgrade_mode
[rpc-upgrade]: /guides/security/securing-nomad.html#rpc-upgrade-mode-for-nomad-servers
[seal]: https://www.vaultproject.io/docs/concepts/seal.html
[secure-introduction]: https://learn.hashicorp.com/vault/identity-access-management/iam-secure-intro
[token]: https://www.vaultproject.io/docs/concepts/tokens.html
[vault-ca-learn]: https://learn.hashicorp.com/vault/secrets-management/sm-pki-engine
[vault-ra]: https://learn.hashicorp.com/vault/operations/ops-reference-architecture
