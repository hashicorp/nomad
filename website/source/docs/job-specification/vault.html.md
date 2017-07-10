---
layout: "docs"
page_title: "vault Stanza - Job Specification"
sidebar_current: "docs-job-specification-vault"
description: |-
   The "vault" stanza allows the task to specify that it requires a token from a
   HashiCorp Vault server. Nomad will automatically retrieve a Vault token for
   the task and handle token renewal for the task.
---

# `vault` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>job -> **vault**</code>
      <br>
      <code>job -> group -> **vault**</code>
      <br>
      <code>job -> group -> task -> **vault**</code>
    </td>
  </tr>
</table>

The `vault` stanza allows a task to specify that it requires a token from a
[HashiCorp Vault][vault] server. Nomad will automatically retrieve a Vault token
for the task and handle token renewal for the task. If specified at the `group`
level, the configuration will apply to all tasks within the group. If specified
at the `job` level, the configuration will apply to all tasks within the job. If
multiple `vault` stanzas are specified, they are merged with the `task` stanza
taking the highest precedence, then the `group`, then the `job`.

```hcl
job "docs" {
  group "example" {
    task "server" {
      vault {
        policies = ["cdn", "frontend"]

        change_mode   = "signal"
        change_signal = "SIGUSR1"
      }
    }
  }
}
```

The Nomad client will make the Vault token available to the task by writing it
to the secret directory at `secret/vault_token` and by injecting a VAULT_TOKEN
environment variable.

If Vault token renewal fails due to a Vault outage, the Nomad client will
attempt to retrieve a new Vault token. When the new Vault token is retrieved,
the contents of the file will be replaced and action will be taken based on the
`change_mode`.

If Nomad is unable to renew the Vault token (perhaps due to a Vault outage or
network error), the client will retrieve a new Vault token. If successful, the
contents of the secrets file are updated on disk, and action will be taken
according to the value set in the `change_mode` parameter.

If a `vault` stanza is specified, the [`template`][template] stanza can interact
with Vault as well.

## `vault` Parameters

- `change_mode` `(string: "restart")` - Specifies the behavior Nomad should take
  if the Vault token changes. The possible values are:

  - `"noop"` - take no action (continue running the task)
  - `"restart"` - restart the task
  - `"signal"` - send a configurable signal to the task

- `change_signal` `(string: "")` - Specifies the signal to send to the task as a
  string like `"SIGUSR1"` or `"SIGINT"`. This option is required if the
  `change_mode` is `signal`.

- `env` `(bool: true)` - Specifies if the `VAULT_TOKEN` environment variable
  should be set when starting the task.

- `policies` `(array<string>: [])` - Specifies the set of Vault policies that
  the task requires. The Nomad client will generate a a Vault token that is
  limited to those policies.

## `vault` Examples

The following examples only show the `vault` stanzas. Remember that the
`vault` stanza is only valid in the placements listed above.

### Retrieve Token

This example tells the Nomad client to retrieve a Vault token. The token is
available to the task via the canonical environment variable `VAULT_TOKEN` and
written to disk at `secrets/vault_token`. The resulting token will have the
"frontend" Vault policy attached.

```hcl
vault {
  policies = ["frontend"]
}
```

### Signal Task

This example shows signaling the task instead of restarting it.

```hcl
vault {
  policies = ["frontend"]

  change_mode   = "signal"
  change_signal = "SIGINT"
}
```

[restart]: /docs/job-specification/restart.html "Nomad restart Job Specification"
[template]: /docs/job-specification/template.html "Nomad template Job Specification"
[vault]: https://www.vaultproject.io/ "Vault by HashiCorp"
