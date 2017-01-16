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
policies the tasks needs. Nomad clients make the token avaliable to the task and
handle the tokens renewal. Further, Nomad's [`template` block][template] can
retrieve secrets from Vault making it easier than ever to secure your
infrastructure.

Note that in order to use Vault with Nomad, you will need to configure and
install Vault separately from Nomad. Nomad does not run Vault for you.

## Vault Configuration

To use the Vault integration, Nomad servers must be provided a Vault token. This
token can either be a root token or a token from a role. The root token is the
easiest way to get started, but we recommend a role-based token for production
installations. Nomad servers will renew the token automatically.

### Root Token

If Nomad is given a [root
token](https://www.vaultproject.io/docs/concepts/tokens.html#root-tokens), no
further configuration is needed as Nomad can derive a token for jobs using any
Vault policies.

### Role based Token

Vault's [Token Authentication Backend][auth] supports a concept called "roles".
Roles allow policies to be grouped together and token creation to be delegated
to a trusted service such as Nomad. By creating a role, the set of policies that
tasks managed by Nomad can access may be limited compared to giving Nomad a root
token.

When given a non-root token, Nomad queries the token to determine the role it
was generated from. It will then derive tokens for jobs based on that role.
Nomad expects the role to be created with several properties described below
when creating the role with the Vault endpoint `/auth/token/roles/<role_name>`:

```json
{
  "allowed_policies": "<comma-seperated list of policies>",
  "explicit_max_ttl": 0,
  "name": "nomad",
  "orphan": false,
  "period": 259200,
  "renewable": true
}
```

#### Parameters:

* `allowed_policies` - Specifies the list of allowed policies as a
  comma-seperated string This list should contain all policies that jobs running
  under Nomad should have access to. Further, the list must contain one or more
  policies that gives Nomad the following permissions:

    ```hcl
    # Allow creating tokens under the role
    path "auth/token/create/nomad-server" {
        capabilities = ["create", "update"]
    }

    # Allow looking up the role
    path "auth/token/roles/nomad-server" {
        capabilities = ["read"]
    }

    # Allow looking up incoming tokens to validate they have permissions to
    # access the tokens they are requesting
    path "auth/token/lookup/*" {
        capabilities = ["read"]
    }

    # Allow revoking tokens that should no longer exist
    path "/auth/token/revoke-accessor/*" {
        capabilities = ["update"]
    }
    ```

* `explicit_max_ttl` - Specifies the max TTL of a token. Must be set to `0` to
  allow periodic tokens.

* `name` - Specifies the name of the policy. We recommend using the name
  `nomad-server`. If a different name is chosen, replace the role in the above
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

See Vault's [Token Authentication Backend][auth] documentation for all possible
fields and more complete documentation.

#### Example Configuration

To make getting started easy, the basic [`nomad-server`
policy](/data/vault/nomad-server-policy.hcl) and
[role](/data/vault/nomad-server-role.json) described above are available for
download.

The below example assumes Vault is accessible, unsealed and the the operator has
appropriate permissions.

```shell
# Download the policy and role
$ curl https://nomadproject.io/data/vault/nomad-server-policy.hcl -O -s -L
$ curl https://nomadproject.io/data/vault/nomad-server-role.json -O -s -L

# Write the policy to Vault
$ vault policy-write nomad-server nomad-server-policy.hcl

# Edit the role to add any policies that you would like to be accessible to
# Nomad jobs in the list of allowed_policies. Do not remove `nomad-server`.
$ editor nomad-server-role.json

# Create the role with Vault
$ vault write /auth/token/roles/nomad-server @nomad-server-role.json
```

#### Retrieving the Role based Token

After the role is created, a token suitable for the Nomad servers may be
retrieved by issuing the following Vault command:

```
$ vault token-create -role nomad-server
Key             Value
---             -----
token           f02f01c2-c0d1-7cb7-6b88-8a14fada58c0
token_accessor  8cb7fcb3-9a4f-6fbf-0efc-83092bb0cb1c
token_duration  259200s
token_renewable true
token_policies  [<policies>]
```

The token can then be set in the server configuration's [vault block][config],
as a command-line flag, or via an environment variable.

```
$ nomad agent -config /path/to/config -vault-token=f02f01c2-c0d1-7cb7-6b88-8a14fada58c0
```

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

- Nomad is given either a root token or a token created from an approriate role.

[auth]: https://www.vaultproject.io/docs/auth/token.html "Vault Authentication Backend"
[config]: /docs/agent/configuration/vault.html "Nomad Vault configuration block"
[template]: /docs/job-specification/template.html "Nomad template Job Specification"
[vault]: https://www.vaultproject.io/ "Vault by HashiCorp"
[vault-spec]: /docs/job-specification/vault.html "Nomad Vault Job Specification"
