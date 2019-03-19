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
unique certificate, eliminating sharing and the accompanying pain of revocation
and rollover. You can also keep certificate TTLs relatively short which makes
situations where you have to revoke certificates less likely. This in turn
allows you to safely and securely scale your cluster while using mutual TLS
(mTLS).

## Reference Material

- [Vault PKI Secrets Engine][pki-engine]
- [Consul Template][consul-template-github]
- [Build Your Own Certificate Authority (CA)][vault-ca-learn]

## Estimated Time to Complete

25 minutes

## Challenge

Secure your Nomad cluster with mTLS. Configure a root and intermediate CA in
Vault and ensure (with the help of Consul Template) that you are periodically
renewing your X.509 certificates on all nodes to maintain a healthy cluster
state.

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

Enable the pki secrets engine at the pki path:

```shell
$ vault secrets enable pki
```

Tune the pki secrets engine to issue certificates with a maximum time-to-live
(TTL) of 87600 hours:

```shell
$ vault secrets tune -max-lease-ttl=87600h pki
```
* Please note: we are using a common and recommended pattern which is to have
  one mount act as the root CA and to use this CA only to sign intermediate CA
  CSRs from other PKI secrets engines (which we will create in the next few
  steps).

Generate the root certificate and save the certificate as `CA_cert.crt`:

```shell
$ vault write -field=certificate pki/root/generate/internal \
    common_name="global.nomad" ttl=87600h > CA_cert.crt
```

### Step 4: Generate the Intermediate CA and CSR

Enable the pki secrets engine at the pki_int path:

```shell
$ vault secrets enable -path=pki_int pki
```

Tune the pki_int secrets engine to issue certificates with a maximum
time-to-live (TTL) of 43800 hours:

```shell
$ vault secrets tune -max-lease-ttl=43800h pki_int
```
Generate a CSR from your intermediate CA and save it as `pki_intermediate.csr`:

```shell
$ vault write -format=json pki_int/intermediate/generate/internal \
    common_name="global.nomad Intermediate Authority" \
    ttl="43800h" | jq -r '.data.csr' > pki_intermediate.csr
```

### Step 5: Sign the CSR and Configure Intermediate CA Certificate

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

### Step 6: Create a Role

A role is a logical name that maps to a policy used to generate credentials. In
our example, it will allow you to use [configuration
parameters][config-parameters] that specify certificate common names, designate
alternate names, and enable subdomains along with a few other key settings.

Create a role named `nomad-cluster` that specifies the allowed domains, enables
you to create certificates for subdomains, and generates certificates with a TTL
of 86400 seconds (24 hours).

```
$ vault write pki_int/roles/nomad-cluster allowed_domains=global.nomad \
    allow_subdomains=true max_ttl=86400s generate_lease=true
```
You should see the following output if the command you issues was successful:

```
Success! Data written to: pki_int/roles/nomad-cluster
```

### Step 7: Create a Policy to Access the Role Endpoint

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

### Step 8: Generate a Token based on `tls-policy`

Create a token based on `tls-policy` with the following command:

```
$ vault token create -policy="tls-policy" -ttl=24h
```

If the command is successful, you will see output similar to the following:

```shell
Key                  Value
---                  -----
token                s.xafiYzh7MCMotHLu2d35hepR
token_accessor       9vj7q5nnF53JAcTyxvccpAZK
token_duration       24h
token_renewable      true
token_policies       ["default" "tls-policy"]
identity_policies    []
policies             ["default" "tls-policy"]
```

### Step 9: Configure Consul Template on All Nodes

If you are using the AWS environment provided in this guide, you already have
[Consul Template][consul-template-github] installed on all nodes. If you are
using your own environment, please make sure Consul Template is installed. You
can download it [here][ct-download].

Provide the token you created in [Step
8](#step-8-generate-a-token-based-on-tls-policy) to the Consul Template
configuration file located at `/etc/consul-template.d/consul-template.hcl`. You
will also need to specify the [template stanza][ct-template-stanza] so you can
render each of the following on the node at the specified location from the
provided templates (the actual templates will be provided in the next step):

* Node certificate
* Node private key
* CA public certificate

Your `consul-template.hcl` configuration file should look similar to the
following (you will need to do this on each node in the cluster):

```
vault {
  address      = "http://active.vault.service.consul:8200"
  token        = "s.xafiYzh7MCMotHLu2d35hepR"
  grace        = "1s"
  unwrap_token = false
  renew_token  = true
}

syslog {
  enabled  = true
  facility = "LOCAL5"
}

template {
  source      = "/opt/nomad/templates/agent.crt.tpl"
  destination = "/opt/nomad/certs/agent.crt"
  command     = "pkill -SIGHUP nomad"
}

template {
  source      = "/opt/nomad/templates/agent.key.tpl"
  destination = "/opt/nomad/certs/agent.key"
  command     = "pkill -SIGHUP nomad"
}

template {
  source      = "/opt/nomad/templates/ca.crt.tpl"
  destination = "/opt/nomad/certs/ca.crt"
  command     = "pkill -SIGHUP nomad"
}
```
You may change the `source` and `destination` options in your
configuration to a location you prefer. This environment assumes you have
created a `templates` and `certs` directory in `/opt/nomad`.

!> Note: we have hard coded the token we created into the Consul Template
configuration file. Although we can avoid this by assigning it to the
environment variable `VAULT_TOKEN`, this approach is still a security concern.
We need to securely introduce this token to Consul Template. To learn how to
accomplish this, see [Secure Introduction][secure-introduction].

### Step 10: Create Templates

The following are the templates used by the configuration file we specified in
the previous step:

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
word `client`:

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

### Step 10: Start the Consul Template Service

Once you have written the individual templates to the `source` locations you
specified in each of the template stanzas in [Step
9](#step-9-configure-consul-template-on-all-nodes), you may start the Consul
Template service on each node:

```shell
$ sudo systemctl start consul-template
```
You can quickly confirm the appropriate certs and private keys were generated in
the `destination` directory you specified in your Consul Template configuration
by listing them out:

```
$ ls /opt/nomad/certs/
agent.crt  agent.key  ca.crt
```

[capability]: https://www.vaultproject.io/docs/concepts/policies.html#capabilities
[config-parameters]: https://www.vaultproject.io/api/secret/pki/index.html#parameters-8
[consul-template]: https://www.consul.io/docs/guides/consul-template.html
[consul-template-github]: https://github.com/hashicorp/consul-template
[ct-download]: https://releases.hashicorp.com/consul-template/
[ct-template-stanza]: https://github.com/hashicorp/consul-template#configuration-file-format
[login]: https://www.vaultproject.io/docs/commands/login.html
[policies]: https://www.vaultproject.io/docs/concepts/policies.html#policies
[pki-engine]: https://www.vaultproject.io/docs/secrets/pki/index.html
[repo]: https://github.com/hashicorp/nomad/tree/master/terraform
[seal]: https://www.vaultproject.io/docs/concepts/seal.html
[secure-introduction]: https://learn.hashicorp.com/vault/identity-access-management/iam-secure-intro
[token]: https://www.vaultproject.io/docs/concepts/tokens.html
[vault-ca-learn]: https://learn.hashicorp.com/vault/secrets-management/sm-pki-engine
[vault-ra]: https://learn.hashicorp.com/vault/operations/ops-reference-architecture