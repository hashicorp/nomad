---
layout: "docs"
page_title: "Vault Integration"
sidebar_current: "docs-vault-integration"
description: |-
  Learn how to integrate with HashiCorp Vault and retrieve Vault tokens for
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

## Vault Configuration

To use the Vault integration, Nomad servers must be provided a Vault token. This
token can either be a root token or a token with permissions to create from a
role. The root token is the easiest way to get started, but we recommend a
role-based token for production installations. Nomad servers will renew the
token automatically.

### Root Token Integration

If Nomad is given a [root
token](https://www.vaultproject.io/docs/concepts/tokens.html#root-tokens), no
further configuration is needed as Nomad can derive a token for jobs using any
Vault policies.

### Role based Integration

Vault's [Token Authentication Backend][auth] supports a concept called "roles".
Roles allow policies to be grouped together and token creation to be delegated
to a trusted service such as Nomad. By creating a role, the set of policies that
tasks managed by Nomad can access may be limited compared to giving Nomad a root
token. Roles allow both whitelist and blacklist management of polcies accessible
to the role.

To configure Nomad and Vault to create tokens against a role, the following must
occur:

  1. Create a set of Vault policies that can be used to generate a token for the
     Nomad Servers that allow them to create from a role and manage created
     tokens within the cluster. The required policies are described below.

  2. Create a Vault role with the configuration described below.

  3. Configure Nomad to use the created role.

  4. Give Nomad servers a token with the policies created from step 1. The token
     must also be periodic.

#### Required Vault Policies

The token Nomad receives must have the capabilities listed below. An explanation
for the use of each capability is given.

```
# Allow creating tokens under "nomad-cluster" role. The role name should be
# updated if "nomad-cluster" is not used.
path "auth/token/create/nomad-cluster" {
  capabilities = ["update"]
}

# Allow looking up "nomad-cluster" role. The role name should be updated if
# "nomad-cluster" is not used.
path "auth/token/roles/nomad-cluster" {
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
path "/sys/capabilities-self" {
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
$ vault policy-write nomad-server nomad-server-policy.hcl
```

#### Vault Role Configuration

A Vault role must be created for use by Nomad. The role can be used to manage
what Vault policies are accessible by jobs submitted to Nomad. The policies can
be managed as a whitelist by using `allowed_policies` in the role definition or
as a blacklist by using `disallowed_policies`.

If using `allowed_policies`, task's may only request Vault policies that are in
the list. If `disallowed_policies` is used, task may request any policy that is
not in the `disallowed_policies` list. There are tradeoffs to both approaches
but generally it is easier to use the blacklist approach and add policies that
you would not like tasks to have access to into the `disallowed_policies` list.

An example role definition is given below:

```json
{
  "disallowed_policies": "nomad-server",
  "explicit_max_ttl": 0,
  "name": "nomad-cluster",
  "orphan": false,
  "period": 259200,
  "renewable": true
}
```

##### Role Requirements

Nomad checks that role's have an appropriate configuration for use by the
cluster. Fields that are checked are documented below as well as descriptions of
the important fields. See Vault's [Token Authentication Backend][auth]
documentation for all possible fields and more complete documentation.

* `allowed_policies` - Specifies the list of allowed policies as a
  comma-seperated string. This list should contain all policies that jobs running
  under Nomad should have access to.

* `disallowed_policies` - Specifies the list of disallowed policies as a
  comma-seperated string. This list should contain all policies that jobs running
  under Nomad should **not** have access to. The policy created above that
  grants Nomad the ability to generate tokens from the role should be included
  in list of disallowed policies. This prevents tokens created by Nomad from
  generating new tokens with different policies than those granted by Nomad.

* `explicit_max_ttl` - Specifies the max TTL of a token. Must be set to `0` to
  allow periodic tokens.

* `name` - Specifies the name of the policy. We recommend using the name
  `nomad-cluster`. If a different name is chosen, replace the role in the above
  policy.

* `orphan` - Specifies whether tokens created againsts this role will be
  orphaned and have no parents. Must be set to `false`. This ensures that the
  token can be revoked when the task is no longer needed or a node dies.

* `period` - Specifies the length the TTL is extended by each renewal in
  seconds. It is suggested to set this value on the order of magnitude of 3 days
  (259200 seconds) to avoid a large renewal request rate to Vault. Must be set
  to a positive value.

* `renewable` - Specifies whether created tokens are renewable. Must be set to
  `true`. This allows Nomad to renew tokens for tasks.

The above [`nomad-cluster` role](/data/vault/nomad-cluster-role.hcl) is
available for download. Below is an example of writing this role to Vault:

```
# Download the role
$ curl https://nomadproject.io/data/vault/nomad-cluster-role.json -O -s -L

# Create the role with Vault
$ vault write /auth/token/roles/nomad-cluster @nomad-cluster-role.json
```


#### Example Configuration

To make getting started easy, the basic [`nomad-server`
policy](/data/vault/nomad-server-policy.hcl) and
[role](/data/vault/nomad-cluster-role.json) described above are available for
download.

The below example assumes Vault is accessible, unsealed and the the operator has
appropriate permissions.

```shell
# Download the policy and role
$ curl https://nomadproject.io/data/vault/nomad-server-policy.hcl -O -s -L
$ curl https://nomadproject.io/data/vault/nomad-cluster-role.json -O -s -L

# Write the policy to Vault
$ vault policy-write nomad-server nomad-server-policy.hcl

# Create the role with Vault
$ vault write /auth/token/roles/nomad-cluster @nomad-cluster-role.json
```

#### Retrieving the Role based Token

After the role is created, a token suitable for the Nomad servers may be
retrieved by issuing the following Vault command:

```
$ vault token-create -policy nomad-server -period 72h
Key             Value
---             -----
token           f02f01c2-c0d1-7cb7-6b88-8a14fada58c0
token_accessor  8cb7fcb3-9a4f-6fbf-0efc-83092bb0cb1c
token_duration  259200s
token_renewable true
token_policies  [default nomad-server]
```

The token can then be set in the server configuration's [vault block][config],
as a command-line flag, or via an environment variable.

```
$ VAULT_TOKEN=f02f01c2-c0d1-7cb7-6b88-8a14fada58c0 nomad agent -config /path/to/config
```

## Agent Configuration

To enable Vault integration, please see the [Nomad agent Vault
integration][config] configuration.

## Vault Definition Syntax

To configure a job to retrieve Vault tokens, please see the [`vault` job
specification documentation][vault-spec].

## Troubleshooting

Upon startup, Nomad will attempt to connect to the specified Vault server. Nomad
will lookup the passed token and if the token is from a role, the role will be
validated. Nomad will not shutdown if given an invalid Vault token, but will log
the reasons the token is invalid and disable Vault integration.

## Assumptions

- Vault 0.6.2 or later is needed.

# XXX
- Nomad is given either a root token or a token created from an approriate role.

[auth]: https://www.vaultproject.io/docs/auth/token.html "Vault Authentication Backend"
[config]: /docs/agent/configuration/vault.html "Nomad Vault Configuration Block"
[createfromrole]: /docs/agent/configuration/vault.html#create_from_role "Nomad vault create_from_role Configuration Flag"
[template]: /docs/job-specification/template.html "Nomad template Job Specification"
[vault]: https://www.vaultproject.io/ "Vault by HashiCorp"
[vault-spec]: /docs/job-specification/vault.html "Nomad Vault Job Specification"
