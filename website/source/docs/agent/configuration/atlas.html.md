---
layout: "docs"
page_title: "atlas Stanza - Agent Configuration"
sidebar_current: "docs-agent-configuration-atlas"
description: |-
  The `atlas` stanza configures Nomad's integration with HashiCorp's Atlas and
  Nomad Enterprise.
---

# `atlas` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>**atlas**</code>
    </td>
  </tr>
</table>


The `atlas` stanza configures Nomad's integration with
[HashiCorp's Atlas][atlas] and Nomad Enterprise.

```hcl
atlas {
  infrastructure = "hashicorp/example"
  join           = true
}
```

~> Nomad integration with Atlas is **currently in private beta** and only
available to select users. As the functionality becomes more widely available,
additional examples and documented will be listed here.

## `atlas` Parameters

- `endpoint` `(string: "https://atlas.hashicorp.com")` - Specifies the address
  of the Atlas service to connect.

- `infrastructure` `(string: <required>)` - Specifies the name of the Atlas
  infrastructure to connect the agent. This should be of the form
  `<organization>/<infrastructure>`, and requires a valid `token`

- `join` `(bool: false)` - Specifies if the auto-join functionality should be
  enabled.

- `token` `(string: <required>)` - Specifies the Atlas token to use for
  authentication. This token must have access to the provided `infrastructure`.
  This can also optionally be specified using the `ATLAS_TOKEN` environment
  variable.

## `atlas` Examples

The following examples only show the `atlas` stanzas. Remember that the
`atlas` stanza is only valid in the placements listed above.

### Nomad Enterprise SaaS

This example connects to the public Nomad Enterprise service to the
infrastructure named "hashicorp/example". The provided token must have
permissions to manage the infrastructure or access will be denied.

```hcl
atlas {
  infrastructure = "hashicorp/example"
  token          = "abcd.atlasv1.efghi...."
  join           = true
}
```

### On-Premise Nomad Enterprise

This example connects to a custom Nomad Enterprise server, such as an on-premise
installation.

```hcl
atlas {
  endpoint       = "https://corp.atlas.local/"
  infrastructure = "acme/example"
  join           = true
}
```

[atlas]: https://atlas.hashicorp.com/ "Atlas by HashiCorp"
