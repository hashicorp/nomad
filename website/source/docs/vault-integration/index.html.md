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
Nomad integrates with HashiCorp's [Vault][]. Nomad Servers and Clients
coordinate with Vault to derive a Vault token that has access to only the Vault
policies the tasks needs. Nomad Clients make the token avaliable to the task and
handle the tokens renewal. Further, Nomad's [`template` block][template] can
retrieve secrets from Vault making it easier than ever to secure your
infrastructure. 

Note that in order to use Vault with Nomad, you will need to configure and
install Vault separately from Nomad. Nomad does not run Vault for you.

## Vault Configuration

In order to use the Vault integration, Nomad Servers must be given a Vault
token. This Vault token can be either a root token or a token created from a
role. The root token provides an easy way to get started but it is recommended
to use the role based token described below. If the token is periodic, Nomad
Servers will renew the token.

### Root Token

If Nomad is given a root token, no further configuration is needed as Nomad can
derive a token for jobs using any Vault policies.

### Role based Token

Vault's [Token Authentication Backend][auth] supports a concept called "roles".
Roles allow policies to be grouped together and token creation to be delegated
to a trusted service such as Nomad. By creating a role, the set of policies that
task's managed by Nomad can acess may be limited compared to giving Nomad a root
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

* `allowed_policies`: The `allowed_policies` is a comma separated list of
  policies. This list should contain all policies that jobs running under Nomad
  should have access to. Further, the list must contain one or more policies
  that gives Nomad the following permissions:

    ```
    # Allow creating tokens under the role
    path "auth/token/create/<role_name>" {
        capabilities = ["create", "update"]
    }

    # Allow looking up the role
    path "auth/token/roles/<role_name>" {
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

* `explicit_max_ttl`: Must be set to `0` to allow periodic tokens.

* `name`: Any name is acceptable.

* `orphan`: Must be set to `false`. This ensures that the token can be revoked
  when the task is no longer needed or a node dies. This prohibits a leaked
  token being used past the lifetime of a task.

* `period`: Must be set to a positive value. The period specifies the length the
  TTL is extended by each renewal in seconds. It is suggested to set this value
  on the order of magniture of 3 days (259200 seconds) to avoid a large renewal
  request rate to Vault.

* `renewable`: Must be set to `true`. This is to allow Nomad to renew tokens for
  tasks.

See Vault's [Token Authentication Backend][auth] documentation for all possible
fields and more complete documentation.

#### Retrieving the Role based Token

After the role is created, a token suitable for the Nomad servers may be
retrieved by issuing the following Vault command:

```
$ vault token-create -role <role_name>
Key             Value
---             -----
token           f02f01c2-c0d1-7cb7-6b88-8a14fada58c0
token_accessor  8cb7fcb3-9a4f-6fbf-0efc-83092bb0cb1c
token_duration  259200s
token_renewable true
token_policies  [<policies>]
```

The token can then be set in the Server configuration's [vault block][config] or
as a command-line flag:

```
$ nomad agent -config /path/to/config -vault-token=f02f01c2-c0d1-7cb7-6b88-8a14fada58c0
```

## Agent Configuration

To enable Vault integration, please see the [Nomad agent Vault
integration][config] configuration.

## Vault Definition Syntax

To configure a job to retrieve Vault tokens, please see the [`vault` job
specification documentation][vault-spec].

## Assumptions

- Vault 0.6.2 or later is needed.

- Nomad is given either a root token or a token created from an approriate role.

[auth]: https://www.vaultproject.io/docs/auth/token.html "Vault Authentication Backend"
[config]: /docs/agent/config.html#vault_options "Nomad Vault configuration block"
[template]: /docs/job-specification/template.html "Nomad template Job Specification"
[vault]: https://www.vaultproject.io/ "Vault by HashiCorp"
[vault-spec]: /docs/job-specification/vault.html "Nomad Vault Job Specification"
