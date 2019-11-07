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
  environment variables for the task. ([See below](#environment-variables))

- `left_delimiter` `(string: "{{")` - Specifies the left delimiter to use in the
  template. The default is "{{" for some templates, it may be easier to use a
  different delimiter that does not conflict with the output file itself.

- `perms` `(string: "644")` - Specifies the rendered template's permissions.
  File permissions are given as octal of the Unix file permissions `rwxrwxrwx`.

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

-   `vault_grace` `(string: "15s")` - Specifies the grace period between lease
    renewal and secret re-acquisition. When renewing a secret, if the remaining
    lease is less than or equal to the configured grace, the template will request
    a new credential. This prevents Vault from revoking the secret at its
    expiration and the task having a stale secret.

    If the grace is set to a value that is higher than your default TTL or max
    TTL, the template will always read a new secret. **If secrets are being
    renewed constantly, decrease the `vault_grace`.**

    If the task defines several templates, the `vault_grace` will be set to the
    lowest value across all the templates.


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
Env templates work exactly like other templates except once the templates are
written, they are parsed as `KEY=value` pairs. Those key value pairs are
included in the task's environment.

For example the following template stanza:

```hcl
template {
  data = <<EOH
# Lines starting with a # are ignored

# Empty lines are also ignored
LOG_LEVEL="{{key "service/geo-api/log-verbosity"}}"
API_KEY="{{with secret "secret/geo-api-key"}}{{.Data.value}}{{end}}"
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

Secrets or certificates may contain a wide variety of characters such as
newlines, quotes, and backslashes which may be difficult to quote or escape
properly.

Whenever a templated variable may include special characters, use the `toJSON`
function to ensure special characters are properly parsed by Nomad:

```
CERT_PEM={{ file "path/to/cert.pem" | toJSON }}
```

The parser will read the JSON string, so the `$CERT_PEM` environment variable
will be identical to the contents of the file.

Likewise when evaluating a password that may contain quotes or `#`, use the
`toJSON` function to ensure Nomad passes the password to task unchanged:

```
# Passwords may contain any character including special characters like:
#   \"'#
# Use toJSON to ensure Nomad passes them to the environment unchanged.
{{ with secret "secrets/data/application/backend" }}
DB_PASSWD={{ .Data.data.DB_PASSWD | toJSON }}
{{ end }}
```

For more details see [go-envparser's README][go-envparse].

## Vault Integration

### PKI Certificate

Vault is a popular open source tool for managing secrets. In addition to acting
as an encrypted KV store, Vault can also generate dynamic secrets, like PKI/TLS
certificates.

When generating PKI certificates with Vault, the certificate, private key, and
any intermediate certs are all returned as part of the same API call. Most
software requires these files be placed in separate files on the system.

~> **Note**: `generate_lease` must be set to `true` (non-default) on the Vault PKI
role.<br /><br /> Failure to do so will cause the template to frequently render a new
certificate, approximately every minute. This creates a significant number of
certificates to be expired in Vault and could ultimately lead to Vault performance
impacts and failures.


#### As individual files

For templates, all dependencies are mapped into a single list. This means that
multiple templates watching the same path return the same data.

```hcl
template {
  data = <<EOH
{{ with secret "pki/issue/foo" "common_name=foo.service.consul" "ip_sans=127.0.0.1" }}
{{- .Data.certificate -}}
{{ end }}
EOH
  destination   = "${NOMAD_SECRETS_DIR}/certificate.crt"
  change_mode   = "restart"
}

template {
  data = <<EOH
{{ with secret "pki/issue/foo" "common_name=foo.service.consul" "ip_sans=127.0.0.1" }}
{{- .Data.issuing_ca -}}
{{ end }}
EOH
  destination   = "${NOMAD_SECRETS_DIR}/ca.crt"
  change_mode   = "restart"
}

template {
  data = <<EOH
{{ with secret "pki/issue/foo" "common_name=foo.service.consul" "ip_sans=127.0.0.1" }}
{{- .Data.private_key -}}
{{ end }}
EOH
  destination   = "${NOMAD_SECRETS_DIR}/private_key.key"
  change_mode   = "restart"
}
```

These are three different input templates, but when run under the Nomad job,
they are compressed into a single call, sharing the resulting data.

#### As a PEM formatted file
This example acquires a PKI certificate from Vault in PEM format, concatenates
the elements into a bundle, and stores it into your application's secret
directory.

```hcl
template {
  data = <<EOH
{{ with secret "pki/issue/foo" "common_name=foo.service.consul" "ip_sans=127.0.0.1" "format=pem" }}
{{ .Data.certificate }}
{{ .Data.issuing_ca }}
{{ .Data.private_key }}{{ end }}
EOH
  destination   = "${NOMAD_SECRETS_DIR}/bundle.pem"
  change_mode   = "restart"
}
```

### Vault KV API v1

Under Vault KV API v1, paths start with `secret/`, and the response returns the
raw key/value data. This secret was set using
`vault kv put secret/aws/s3 aws_access_key_id=somekeyid`.

```hcl
  template {
    data = <<EOF
      AWS_ACCESS_KEY_ID = "{{with secret "secret/aws/s3"}}{{.Data.aws_access_key_id}}{{end}}"
    EOF
  }
```

### Vault KV API v2

Under Vault KV API v2, paths start with `secret/data/`, and the response returns
metadata in addition to key/value data. This secret was set using
`vault kv put secret/aws/s3 aws_access_key_id=somekeyid`.

```hcl
  template {
    data = <<EOF
      AWS_ACCESS_KEY_ID = "{{with secret "secret/data/aws/s3"}}{{.Data.data.aws_access_key_id}}{{end}}"
    EOF
  }
```

Notice the addition of `data` in both the path and the field accessor string.
Additionally, when using the Vault v2 API, the Vault policies applied to your
Nomad jobs will need to grant permissions to `read` under `secret/data/...`
rather than `secret/...`.

## Client Configuration

The `template` block has the following [client configuration
options](/docs/configuration/client.html#options):

* `template.allow_host_source` - Allows templates to specify their source
  template as an absolute path referencing host directories. Defaults to `true`.

[ct]: https://github.com/hashicorp/consul-template "Consul Template by HashiCorp"
[artifact]: /docs/job-specification/artifact.html "Nomad artifact Job Specification"
[env]: /docs/runtime/environment.html "Nomad Runtime Environment"
[nodevars]: /docs/runtime/interpolation.html#interpreted_node_vars "Nomad Node Variables"
[go-envparse]: https://github.com/hashicorp/go-envparse#readme "The go-envparse Readme"
