---
layout: "docs"
page_title: "vault Stanza - Agent Configuration"
sidebar_current: "docs-agent-configuration-vault"
description: |-
  The "vault" stanza configures Nomad's integration with HashiCorp's Vault.
  When configured, Nomad can create and distribute secrets to tasks
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


The `vault` stanza configures Nomad's integration with
[HashiCorp's Vault][vault]. When configured, Nomad can create and distribute
secrets to tasks automatically. For more information on the architecture and
setup, please see the [Nomad and Vault integration documentation][nomad-vault].

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

- `allow_unauthenticated` `(bool: false)` - Specifies if users submitting jobs
  to the Nomad server should be required to provide their own Vault token,
  proving they have access to the policies listed in the job. If enabled, users
  could easily escalate privilege in a job.

- `enabled` `(bool: false)` - Specifies if the Vault integration should be
  activated.

- `task_token_ttl` `(string: "")` - Specifies the TTL of created tokens when
  using a root token. This is specified using a label suffix like "30s" or "1h".

- `tls_ca_file` `(string: "")` - Specifies an optional path to the CA
  certificate used for Vault communication. If unspecified, this will fallback
  to the default system CA bundle, which varies by OS and version.

- `tls_ca_path` `(string: "")` - Specifies an optional path to a folder
  containing CA certificates to be used for Vault communication. If unspecified,
  this will fallback to the default system CA bundle, which varies by OS and
  version.

- `tls_cert_file` `(string: "")` - Specifies the path to the certificate used
  for Vault communication. If this is set then you need to also set
  `tls_key_file`.

- `tls_key_file` `(string: "")` - Specifies the path to the private key used for
  Vault communication. If this is set then you need to also set `tls_cert_file`.

- `tls_server_name` `(string: "")` - Specifies an optional string used to set
  the SNI host when connecting to Vault via TLS.

- `tls_skip_verify` `(bool: false)` - Specifies if SSL peer validation should be
  enforced.

    !> It is **strongly discouraged** to disable SSL verification. Instead, you
    should install a custom CA bundle and validate against it. Disabling SSL
    verification can allow an attacker to easily compromise your cluster.

- `token` `(string: "")` - Specifies the parent Vault token to use to derive child tokens for jobs
  requesting tokens.
  Visit the [Vault Integration](/docs/vault-integration/index.html)
  documentation to see how to generate an appropriate token in Vault.

    !> It is **strongly discouraged** to place the token as a configuration
    parameter like this, since the token could be checked into source control
    accidentally. Users should set the `VAULT_TOKEN` environment variable when
    starting the agent instead.


## `vault` Examples

The following examples only show the `vault` stanzas. Remember that the
`vault` stanza is only valid in the placements listed above.

### Default Configuration

This example shows the most basic Vault integration configuration. If all
defaults are correct, simply include the Vault stanza to enable the integration:

```hcl
vault {
  enabled = true
}
```

### Custom Address

This example shows using a custom Vault address:

```hcl
vault {
  enabled = true
  address = "https://vault.company.internal:8200"
}
```

### TLS Configuration

This example shows utilizing a custom CA bundle and key to authenticate between
Nomad and Vault:

```hcl
vault {
  enabled         = true
  tls_ca_path     = "/etc/certs/ca"
  tls_cert_file   = "/var/certs/vault.crt"
  tls_key_file    = "/var/certs/vault.key"
  tls_server_name = "nomad.service.consul"
}
```

[vault]: https://www.vaultproject.io/ "Vault by HashiCorp"
[nomad-vault]: /docs/vault-integration/index.html "Nomad Vault Integration"
