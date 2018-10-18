---
layout: "guides"
page_title: "Vault Integration"
sidebar_current: "guides-operations-vault-integration"
description: |-
  Learn how to integrate Nomad with HashiCorp Vault and retrieve Vault tokens for
  tasks.
---

# Vault Integration

Many workloads require access to tokens, passwords, certificates, API keys, and
other secrets. To enable secure, auditable and easy access to your secrets,
Nomad integrates with HashiCorp's [Vault][]. Nomad servers and clients
coordinate with Vault to derive a Vault token that has access to only the Vault
policies the tasks needs. Nomad clients make the token available to the task and
handle the tokens renewal. Further, Nomad's [`template` block][template] can
retrieve secrets from Vault making it easier than ever to secure your
infrastructure.

Note that in order to use Vault with Nomad, you will need to configure and
install Vault separately from Nomad. Nomad does not run Vault for you.

-> **Note:** Vault integration requires Vault version 0.6.2 or higher.

## Vault Configuration

To use the Vault integration, Nomad servers must be provided a Vault token. This
token can either be a root token or a periodic token with permissions to create
from a token role. The root token is the easiest way to get started, but we
recommend a token role based token for production installations. Nomad servers
will renew the token automatically. **Note that the Nomad clients do not need to
be provided with a Vault token.**

### Root Token Integration

If Nomad is given a [root
token](https://www.vaultproject.io/docs/concepts/tokens.html#root-tokens), no
further configuration is needed as Nomad can derive a token for jobs using any
Vault policies.

### Token Role based Integration

Vault's [Token Authentication Backend][auth] supports a concept called "roles".
Token roles allow policies to be grouped together and token creation to be
delegated to a trusted service such as Nomad. By creating a token role, the set
of policies that tasks managed by Nomad can access may be limited compared to
giving Nomad a root token. Token roles allow both white-list and blacklist
management of policies accessible to the role.

To configure Nomad and Vault to create tokens against a role, the following must
occur:

  1. Create a "nomad-server" policy used by Nomad to create and manage tokens.

  2. Create a Vault token role with the configuration described below.

  3. Configure Nomad to use the created token role.

  4. Give Nomad servers a periodic token with the "nomad-server" policy created
     above.

#### Required Vault Policies

The token Nomad receives must have the capabilities listed below. An explanation
for the use of each capability is given.

```hcl
# Allow creating tokens under "nomad-cluster" token role. The token role name
# should be updated if "nomad-cluster" is not used.
path "auth/token/create/nomad-cluster" {
  capabilities = ["update"]
}

# Allow looking up "nomad-cluster" token role. The token role name should be
# updated if "nomad-cluster" is not used.
path "auth/token/roles/nomad-cluster" {
  capabilities = ["read"]
}

# Allow looking up the token passed to Nomad to validate # the token has the
# proper capabilities. This is provided by the "default" policy.
path "auth/token/lookup-self" {
  capabilities = ["read"]
}

# Allow looking up incoming tokens to validate they have permissions to access
# the tokens they are requesting. This is only required if
# `allow_unauthenticated` is set to false.
path "auth/token/lookup" {
  capabilities = ["update"]
}

# Allow revoking tokens that should no longer exist. This allows revoking
# tokens for dead tasks.
path "auth/token/revoke-accessor" {
  capabilities = ["update"]
}

# Allow checking the capabilities of our own token. This is used to validate the
# token upon startup.
path "sys/capabilities-self" {
  capabilities = ["update"]
}

# Allow our own token to be renewed.
path "auth/token/renew-self" {
  capabilities = ["update"]
}
```

The above [`nomad-server` policy](/data/vault/nomad-server-policy.hcl) is
available for download. Below is an example of writing this policy to Vault:

```
# Download the policy
$ curl https://nomadproject.io/data/vault/nomad-server-policy.hcl -O -s -L

# Write the policy to Vault
$ vault policy write nomad-server nomad-server-policy.hcl
```

#### Vault Token Role Configuration

A Vault token role must be created for use by Nomad. The token role can be used
to manage what Vault policies are accessible by jobs submitted to Nomad. The
policies can be managed as a whitelist by using `allowed_policies` in the token
role definition or as a blacklist by using `disallowed_policies`.

If using `allowed_policies`, tasks may only request Vault policies that are in
the list. If `disallowed_policies` is used, task may request any policy that is
not in the `disallowed_policies` list. There are trade-offs to both approaches
but generally it is easier to use the blacklist approach and add policies that
you would not like tasks to have access to into the `disallowed_policies` list.

An example token role definition is given below:

```json
{
  "disallowed_policies": "nomad-server",
  "explicit_max_ttl": 0,
  "name": "nomad-cluster",
  "orphan": true,
  "period": 259200,
  "renewable": true
}
```


##### Token Role Requirements

Nomad checks that token role has an appropriate configuration for use by the
cluster. Fields that are checked are documented below as well as descriptions of
the important fields. See Vault's [Token Authentication Backend][auth]
documentation for all possible fields and more complete documentation.

* `allowed_policies` - Specifies the list of allowed policies as a
  comma-separated string. This list should contain all policies that jobs running
  under Nomad should have access to.

*    `disallowed_policies` - Specifies the list of disallowed policies as a
     comma-separated string. This list should contain all policies that jobs running
     under Nomad should **not** have access to. The policy created above that
     grants Nomad the ability to generate tokens from the token role should be
     included in list of disallowed policies. This prevents tokens created by
     Nomad from generating new tokens with different policies than those granted
     by Nomad.

     A regression occurred in Vault 0.6.4 when validating token creation using a
     token role with `disallowed_policies` such that it is not usable with
     Nomad. This will be remedied in 0.6.5 and does not effect earlier versions
     of Vault.

* `explicit_max_ttl` - Specifies the max TTL of a token. **Must be set to `0`** to
  allow periodic tokens.

* `name` - Specifies the name of the policy. We recommend using the name
  `nomad-cluster`. If a different name is chosen, replace the token role in the
  above policy.

*   `orphan` - Specifies whether tokens created against this token role will be
  orphaned and have no parents. Nomad does not enforce the value of this field
  but understanding the implications of each value is important.

    If set to false, all tokens will be revoked when the Vault token given to
  Nomad expires. This makes it easy to revoke all tokens generated by Nomad but
  forces all Nomad servers to use the same Vault token, even through upgrades of
  Nomad servers. If the Vault token that was given to Nomad and used to generate
  a tasks token expires, the token used by the task will also be revoked which
  is not ideal.

    When set to true, the tokens generated for tasks will not be revoked when
  Nomad's token is revoked. However Nomad will still revoke tokens when the
  allocation is no longer running, minimizing the lifetime of any task's token.
  With orphaned enabled, each Nomad server may also use a unique Vault token,
  making bootstrapping and upgrading simpler. As such, **setting `orphan = true`
  is the recommended setting**.

* `period` - Specifies the length the TTL is extended by each renewal in
  seconds. It is suggested to set this value on the order of magnitude of 3 days
  (259200 seconds) to avoid a large renewal request rate to Vault. **Must be set
  to a positive value**.

* `renewable` - Specifies whether created tokens are renewable. **Must be set to
  `true`**. This allows Nomad to renew tokens for tasks.

The above [`nomad-cluster` token role](/data/vault/nomad-cluster-role.json) is
available for download. Below is an example of writing this role to Vault:

```
# Download the token role
$ curl https://nomadproject.io/data/vault/nomad-cluster-role.json -O -s -L

# Create the token role with Vault
$ vault write /auth/token/roles/nomad-cluster @nomad-cluster-role.json
```


#### Example Configuration

To make getting started easy, the basic [`nomad-server`
policy](/data/vault/nomad-server-policy.hcl) and
[`nomad-cluster` role](/data/vault/nomad-cluster-role.json) described above are
available for download.

The below example assumes Vault is accessible, unsealed and the operator has
appropriate permissions.

```shell
# Download the policy and token role
$ curl https://nomadproject.io/data/vault/nomad-server-policy.hcl -O -s -L
$ curl https://nomadproject.io/data/vault/nomad-cluster-role.json -O -s -L

# Write the policy to Vault
$ vault policy write nomad-server nomad-server-policy.hcl

# Create the token role with Vault
$ vault write /auth/token/roles/nomad-cluster @nomad-cluster-role.json
```

#### Retrieving the Token Role based Token

After the token role is created, a token suitable for the Nomad servers may be
retrieved by issuing the following Vault command:

```
$ vault token create -policy nomad-server -period 72h -orphan
Key             Value
---             -----
token           f02f01c2-c0d1-7cb7-6b88-8a14fada58c0
token_accessor  8cb7fcb3-9a4f-6fbf-0efc-83092bb0cb1c
token_duration  259200s
token_renewable true
token_policies  [default nomad-server]
```

The `-orphan` flag is included when generating the Nomad server token above to
prevent revocation of the token when its parent expires. Vault typically
creates tokens with a parent-child relationship. When an ancestor token is
revoked, all of its descendant tokens and their associated leases are revoked
as well.

When generating Nomad's Vault token, we need to ensure that revocation of the
parent token does not revoke Nomad's token. To prevent this behavior we
specify the `-orphan` flag when we create the Nomad's Vault token. All
other tokens generated by Nomad for jobs will be generated using the policy
default of `orphan = false`.

More information about creating orphan tokens can be found in
[Vault's Token Hierarchies and Orphan Tokens documentation][tokenhierarchy].

The token can then be set in the server configuration's
[`vault` stanza][config], as a command-line flag, or via an environment
variable.

```
$ VAULT_TOKEN=f02f01c2-c0d1-7cb7-6b88-8a14fada58c0 nomad agent -config /path/to/config
```

An example of what may be contained in the configuration is shown below. For
complete documentation please see the [Nomad agent Vault integration][config]
configuration.

```hcl
vault {
  enabled          = true
  ca_path          = "/etc/certs/ca"
  cert_file        = "/var/certs/vault.crt"
  key_file         = "/var/certs/vault.key"
  address          = "https://vault.service.consul:8200"
  create_from_role = "nomad-cluster"
}
```

## Agent Configuration

To enable Vault integration, please see the [Nomad agent Vault
integration][config] configuration.

## Vault Definition Syntax

To configure a job to retrieve Vault tokens, please see the [`vault` job
specification documentation][vault-spec].

## Troubleshooting

### Invalid Vault token

Upon startup, Nomad will attempt to connect to the specified Vault server. Nomad
will lookup the passed token and if the token is from a token role, the token
role will be validated. Nomad will not shutdown if given an invalid Vault token,
but will log the reasons the token is invalid and disable Vault integration.

### Permission Denied errors

If you are using a Vault version less than 0.7.1 with a Nomad version greater than or equal to 0.6.1, you will need to update your task's policy (listed in [the `vault` stanza of the job specification][vault-spec]) to add the following:

```
path "sys/leases/renew" {
    capabilities = ["update"]
}
```

This is included in Vault's "default" policy beginning with Vault 0.7.1 and is relied upon by Nomad's Vault integration beginning with Nomad 0.6.1. If you're using a newer Nomad version with an older Vault version, your default policy may not automatically include this and you will see "permission denied" errors in your Nomad logs similar to the following:

```
Code: 403. Errors:
URL: PUT https://vault:8200/v1/sys/leases/renew
* permission denied
```

### No Secret Exists

Vault has two APIs for secrets, [`v1` and `v2`][vault-secrets-version]. Each version
has different paths, and Nomad does not abstract this for you. As such you will
need to specify the path as reflected by Vault's HTTP API, rather than the path
used in the `vault kv` command.

You can see examples of `v1` and `v2` syntax in the
[template documentation][vault-kv-templates].


[auth]: https://www.vaultproject.io/docs/auth/token.html "Vault Authentication Backend"
[config]: /docs/configuration/vault.html "Nomad Vault Configuration Block"
[createfromrole]: /docs/configuration/vault.html#create_from_role "Nomad vault create_from_role Configuration Flag"
[template]: /docs/job-specification/template.html "Nomad template Job Specification"
[vault]: https://www.vaultproject.io/ "Vault by HashiCorp"
[vault-spec]: /docs/job-specification/vault.html "Nomad Vault Job Specification"
[tokenhierarchy]: https://www.vaultproject.io/docs/concepts/tokens.html#token-hierarchies-and-orphan-tokens "Vault Tokens - Token Hierarchies and Orphan Tokens"
[vault-secrets-version]: https://www.vaultproject.io/docs/secrets/kv/index.html "KV Secrets Engine"
[vault-kv-templates]: /docs/job-specification/template.html#vault-kv-api-v1 "Vault KV API v1"
