---
layout: "docs"
page_title: "template Stanza - Job Specification"
sidebar_current: "docs-job-specification-template"
description: |-
  The "template" block instantiates an instance of a template renderer. This
  creates a convenient way to ship configuration files that are populated from
  Consul data, Vault secrets, or just general configurations within a Nomad
  task.
---

# `template` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>job -> group -> task -> **template**</code>
    </td>
  </tr>
</table>

The `template` block instantiates an instance of a template renderer. This
creates a convenient way to ship configuration files that are populated from
Consul data, Vault secrets, or just general configurations within a Nomad task.

```hcl
job "docs" {
  group "example" {
    task "server" {
      template {
        source        = "local/redis.conf.tpl"
        destination   = "local/redis.conf"
        change_mode   = "signal"
        change_signal = "SIGNINT"
      }
    }
  }
}
```

Nomad utilizes a tool called [Consul Template][ct]. For a full list of the
API template functions, please refer to the [Consul Template README][ct].

## `template` Parameters

- `source` `(string: "")` - Specifies the path to the template to be rendered.
  One of `source` or `data` must be specified, but not both. This source can
  optionally be fetched using an [`artifact`][artifact] resource. This template
  must exist on the machine prior to starting the task; it is not possible to
  reference a template inside of a Docker container, for example.

- `destination` `(string: <required>)` - Specifies the location where the
  resulting template should be rendered, relative to the task directory.

- `data` `(string: "")` - Specifies the raw template to execute. One of `source`
  or `data` must be specified, but not both. This is useful for smaller
  templates, but we recommend using `source` for larger templates.

- `change_mode` `(string: "restart")` - Specifies the behavior Nomad should take
  if the rendered template changes. The possible values are:

  - `"noop"` - take no action (continue running the task)
  - `"restart"` - restart the task
  - `"signal"` - send a configurable signal to the task

- `change_signal` `(string: "")` - Specifies the signal to send to the task as a
  string like `"SIGUSR1"` or `"SIGINT"`. This option is required if the
  `change_mode` is `signal`.

- `splay` `(string: "5s")` - Specifies a random amount of time to wait between
  0ms and the given splay value before invoking the change mode. This is
  specified using a label suffix like "30s" or "1h", and is often used to
  prevent a thundering herd problem where all task instances restart at the same
  time.

## `template` Examples

The following examples only show the `template` stanzas. Remember that the
`template` stanza is only valid in the placements listed above.

### Inline Template

This example uses an inline template to render a file to disk. This file watches
various keys in Consul for changes:

```hcl
template {
  data        = "---\nkey: {{ key \"service/my-key\" }}"
  destination = "local/file.yml"
}
```

It is also possible to use heredocs for multi-line templates, like:

```hcl
template {
  data = <<EOH
  ---
    key: {{ key "service/my-key" }}
  EOH

  destination = "local/file.yml"
}
```

### Remote Template

This example uses an [`artifact`][artifact] stanza to download an input template
before passing it to the template engine:

```hcl
artifact {
  source      = "https://example.com/file.yml.tpl"
  destination = "local/file.yml.tpl"
}

template {
  source      = "local/file.yml.tpl"
  destination = "local/file.yml"
}
```

### Client Configuration

The `template` block has the following [client configuration
options](/docs/agent/config.html#options):

* `template.allow_host_source` - Allows templates to specify their source
  template as an absolute path referencing host directories. Defaults to `true`.

[ct]: https://github.com/hashicorp/consul-template "Consul Template by HashiCorp"
[artifact]: /docs/job-specification/artifact.html "Nomad artifact Job Specification"
