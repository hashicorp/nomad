---
layout: "docs"
page_title: "template Stanza - Job Specification"
sidebar_current: "docs-job-specification-template"
description: |-
  The "template" block instantiates an instance of a template renderer. This
  creates a convenient way to ship configuration files that are populated from
  environment variables, Consul data, Vault secrets, or just general
  configurations within a Nomad task.
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
environment variables, Consul data, Vault secrets, or just general
configurations within a Nomad task.

```hcl
job "docs" {
  group "example" {
    task "server" {
      template {
        source        = "local/redis.conf.tpl"
        destination   = "local/redis.conf"
        change_mode   = "signal"
        change_signal = "SIGINT"
      }
    }
  }
}
```

Nomad utilizes a tool called [Consul Template][ct]. Since Nomad v0.5.3, the
template can reference [Nomad's runtime environment variables][env]. Since Nomad
v0.5.6, the template can reference [Node attributes and metadata][nodevars]. For
a full list of the API template functions, please refer to the [Consul Template
README][ct]. Since Nomad v0.6.0, templates can be read as environment variables.

## `template` Parameters

- `change_mode` `(string: "restart")` - Specifies the behavior Nomad should take
  if the rendered template changes. Nomad will always write the new contents of
  the template to the specified destination. The possible values below describe
  Nomad's action after writing the template to disk.

  - `"noop"` - take no action (continue running the task)
  - `"restart"` - restart the task
  - `"signal"` - send a configurable signal to the task

- `change_signal` `(string: "")` - Specifies the signal to send to the task as a
  string like `"SIGUSR1"` or `"SIGINT"`. This option is required if the
  `change_mode` is `signal`.

- `data` `(string: "")` - Specifies the raw template to execute. One of `source`
  or `data` must be specified, but not both. This is useful for smaller
  templates, but we recommend using `source` for larger templates.

- `destination` `(string: <required>)` - Specifies the location where the
  resulting template should be rendered, relative to the task directory.

- `env` `(bool: false)` - Specifies the template should be read back in as
  environment variables for the task. (See below)

- `left_delimiter` `(string: "{{")` - Specifies the left delimiter to use in the
  template. The default is "{{" for some templates, it may be easier to use a
  different delimiter that does not conflict with the output file itself.

- `perms` `(string: "644")` - Specifies the rendered template's permissions.
  File permissions are given as octal of the Unix file permissions rwxrwxrwx.

- `right_delimiter` `(string: "}}")` - Specifies the right delimiter to use in the
  template. The default is "}}" for some templates, it may be easier to use a
  different delimiter that does not conflict with the output file itself.

- `source` `(string: "")` - Specifies the path to the template to be rendered.
  One of `source` or `data` must be specified, but not both. This source can
  optionally be fetched using an [`artifact`][artifact] resource. This template
  must exist on the machine prior to starting the task; it is not possible to
  reference a template inside of a Docker container, for example.

- `splay` `(string: "5s")` - Specifies a random amount of time to wait between
  0 ms and the given splay value before invoking the change mode. This is
  specified using a label suffix like "30s" or "1h", and is often used to
  prevent a thundering herd problem where all task instances restart at the same
  time.

- `vault_grace` `(string: "5m")` - Specifies the grace period between lease
  renewal and secret re-acquisition. When renewing a secret, if the remaining
  lease is less than or equal to the configured grace, the template will request
  a new credential. This prevents Vault from revoking the secret at its
  expiration and the task having a stale secret. If the grace is set to a value
  that is higher than your default TTL or max TTL, the template will always read
  a new secret. If the task defines several templates, the `vault_grace` will be
  set to the lowest value across all the templates.

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
    bind_port:   {{ env "NOMAD_PORT_db" }}
    scratch_dir: {{ env "NOMAD_TASK_DIR" }}
    node_id:     {{ env "node.unique.id" }}
    service_key: {{ key "service/my-key" }}
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

### Node Variables

As of Nomad v0.5.6 it is possible to access the Node's attributes and metadata.

```hcl
template {
  data = <<EOH
  ---
    node_dc:    {{ env "node.datacenter" }}
    node_cores: {{ env "attr.cpu.numcores" }}
    meta_key:   {{ env "meta.node_meta_key" }}
  EOH

  destination = "local/file.yml"
}
```

### Environment Variables

Since v0.6.0 templates may be used to create environment variables for tasks.
Env templates work exactly like other templates except once they're written,
they're read back in as `KEY=value` pairs. Those key value pairs are included
in the task's environment.

For example the following template stanza:

```hcl
template {
  data = <<EOH
# Lines starting with a # are ignored

# Empty lines are also ignored
LOG_LEVEL="{{key "service/geo-api/log-verbosity"}}"
API_KEY="{{with secret "secret/geo-api-key"}}{{.Data.key}}{{end}}"
EOH

  destination = "secrets/file.env"
  env         = true
}
```

The task's environment would then have environment variables like the
following:

```
LOG_LEVEL=DEBUG
API_KEY=12345678-1234-1234-1234-1234-123456789abc
```

This allows [12factor app](https://12factor.net/config) style environment
variable based configuration while keeping all of the familiar features and
semantics of Nomad templates.

If a value may include newlines you should JSON encode it:

```
CERT_PEM={{ file "path/to/cert.pem" | toJSON }}
```

The parser will read the JSON string, so the `$CERT_PEM` environment variable
will be identical to the contents of the file.

For more details see [go-envparser's
README](https://github.com/schmichael/go-envparse#readme).

## Client Configuration

The `template` block has the following [client configuration
options](/docs/agent/configuration/client.html#options):

* `template.allow_host_source` - Allows templates to specify their source
  template as an absolute path referencing host directories. Defaults to `true`.

[ct]: https://github.com/hashicorp/consul-template "Consul Template by HashiCorp"
[artifact]: /docs/job-specification/artifact.html "Nomad artifact Job Specification"
[env]: /docs/runtime/environment.html "Nomad Runtime Environment"
[nodevars]: /docs/runtime/interpolation.html#interpreted_node_vars "Nomad Node Variables"
