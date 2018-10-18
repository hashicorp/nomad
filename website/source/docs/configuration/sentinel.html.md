---
layout: "docs"
page_title: "sentinel Stanza - Agent Configuration"
sidebar_current: "docs-configuration-sentinel"
description: |-
  The "sentinel" stanza configures the Nomad agent for Sentinel policies and tune various parameters.
---

# `sentinel` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>**sentinel**</code>
    </td>
  </tr>
</table>

The `sentinel` stanza configures the Sentinel policy engine and tunes various parameters.

```hcl
sentinel {
    import "custom-plugin" {
        path = "/usr/bin/sentinel-custom-plugin"
        args = ["-verbose", "foo"]
    }
}
```

## `sentinel` Parameters

- `import` <code>([Import](#import-parameters): nil)</code> -
  Specifies a plugin that should be made available for importing by Sentinel policies.
  The name of the import matches the name that can be imported.

### `import` Parameters

- `path` `(string: "")` - Specifies the path to the import plugin. Must be executable by Nomad.

- `args` `(array<string>: [])` - Specifies arguments to pass to the plugin when starting it.

