---
layout: "docs"
page_title: "plugin Stanza - Agent Configuration"
sidebar_current: "docs-configuration-plugin"
description: |-
  The "plugin" stanza is used to configure a Nomad plugin.
---

# `plugin` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>**plugin**</code>
    </td>
  </tr>
</table>

The `plugin` stanza is used to configure plugins.

```hcl
plugin "example-plugin" {
    args = ["-my-flag"]
    config {
       foo = "bar"
       bam {
         baz = 1
       }
    }
}
```

The name of the plugin is the plugin's executable name relative to to the
[plugin_dir](/docs/configuration/index.html#plugin_dir). If the plugin has a
suffix, such as `.exe`, this should be omitted.

## `plugin` Parameters

- `args` `(array<string>: [])` - Specifies a set of arguments to pass to the
  plugin binary when it is executed.

- `config` `(hcl/json: nil)` - Specifies configuration values for the plugin
  either as HCL or JSON. The accepted values are plugin specific. Please refer
  to the individual plugin's documentation.
