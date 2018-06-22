---
layout: "docs"
page_title: "vault Stanza - Agent Configuration"
sidebar_current: "docs-configuration-vault"
description: |-
  The "vault" stanza configures Nomad's integration with HashiCorp's Vault.
  When configured, Nomad can create and distribute Vault tokens to tasks
  automatically.
---

# `vault` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>**vault**</code>
    </td>
  </tr>
</table>


The `vault` stanza configures Nomad's integration with [HashiCorp's
Vault][vault]. When configured, Nomad can create and distribute Vault tokens to
tasks automatically. For more information on the architecture and setup, please
see the [Nomad and Vault integration documentation][nomad-vault].

```hcl
vault {
  enabled = true
  address = "https://vault.company.internal:8200"
}
```

## `vault` Parameters

- `address` - `(string: "https://vault.service.consul:8200")` - Specifies the
  address to the Vault server. This must include the protocol, host/ip, and port
  given in the format `protocol://host:port`. If your Vault installation is
  behind a load balancer, this should be the address of the load balancer.

- `allow_unauthenticated` `(bool: true)` - Specifies if users submitting jobs to
  the Nomad server should be required to provide their own Vault token, proving
  they have access to the policies listed in the job. This option should be
  disabled in an untrusted environment.

- `enabled` `(bool: false)` - Specifies if the Vault integration should be
  activated.

- `create_from_role` `(string: "")` - Specifies the role to create tokens from.
  The token given to Nomad does not have to be created from this role but must
  have "update" capability on "auth/token/create/<create_from_role>" path in
  Vault. If this value is unset and the token is created from a role, the value
  is defaulted to the role the token is from. This is largely for backwards
  compatibility. It is recommended to set the `create_from_role` field if Nomad
  is deriving child tokens from a role.

- `task_token_ttl` `(string: "")` - Specifies the TTL of created tokens when
  using a root token. This is specified using a label suffix like "30s" or "1h".

- `ca_file` `(string: "")` - Specifies an optional path to the CA
  certificate used for Vault communication. If unspecified, this will fallback
  to the default system CA bundle, which varies by OS and version.

- `ca_path` `(string: "")` - Specifies an optional path to a folder
  containing CA certificates to be used for Vault communication. If unspecified,
  this will fallback to the default system CA bundle, which varies by OS and
  version.

- `cert_file` `(string: "")` - Specifies the path to the certificate used
  for Vault communication. If this is set then you need to also set
  `tls_key_file`.

- `key_file` `(string: "")` - Specifies the path to the private key used for
  Vault communication. If this is set then you need to also set `cert_file`.

- `tls_server_name` `(string: "")` - Specifies an optional string used to set
  the SNI host when connecting to Vault via TLS.

- `tls_skip_verify` `(bool: false)` - Specifies if SSL peer validation should be
  enforced.

    !> It is **strongly discouraged** to disable SSL verification. Instead, you
    should install a custom CA bundle and validate against it. Disabling SSL
    verification can allow an attacker to easily compromise your cluster.

- `token` `(string: "")` - Specifies the parent Vault token to use to derive child tokens for jobs
  requesting tokens.
  Visit the [Vault Integration Guide](/guides/operations/vault-integration/index.html)
  to see how to generate an appropriate token in Vault.

    !> It is **strongly discouraged** to place the token as a configuration
    parameter like this, since the token could be checked into source control
    accidentally. Users should set the `VAULT_TOKEN` environment variable when
    starting the agent instead.


## `vault` Examples

The following examples only show the `vault` stanzas. Remember that the
`vault` stanza is only valid in the placements listed above.

### Nomad Server

This example shows an example Vault configuration for a Nomad server:

```hcl
vault {
  enabled     = true
  ca_path     = "/etc/certs/ca"
  cert_file   = "/var/certs/vault.crt"
  key_file    = "/var/certs/vault.key"

  # Address to communicate with Vault. The below is the default address if
  # unspecified.
  address     = "https://vault.service.consul:8200"

  # Embedding the token in the configuration is discouraged. Instead users
  # should set the VAULT_TOKEN environment variable when starting the Nomad
  # agent 
  token       = "debecfdc-9ed7-ea22-c6ee-948f22cdd474"

  # Setting the create_from_role option causes Nomad to create tokens for tasks
  # via the provided role. This allows the role to manage what policies are
  # allowed and disallowed for use by tasks.
  create_from_role = "nomad-cluster"
}
```

### Nomad Client

This example shows an example Vault configuration for a Nomad client:

```hcl
vault {
  enabled     = true
  address     = "https://vault.service.consul:8200"
  ca_path     = "/etc/certs/ca"
  cert_file   = "/var/certs/vault.crt"
  key_file    = "/var/certs/vault.key"
}
```

The key difference is that the token is not necessary on the client.

## `vault` Configuration Reloads

The Vault configuration can be reloaded on servers. This can be useful if a new
token needs to be given to the servers without having to restart them. A reload
can be accomplished by sending the process a `SIGHUP` signal.

[vault]: https://www.vaultproject.io/ "Vault by HashiCorp"
[nomad-vault]: /guides/operations/vault-integration/index.html "Nomad Vault Integration"
