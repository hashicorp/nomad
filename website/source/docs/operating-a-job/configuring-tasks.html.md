---
layout: "docs"
page_title: "Configuring Tasks - Operating a Job"
sidebar_current: "docs-operating-a-job-configuring-tasks"
description: |-
  Most applications require some kind of configuration. Whether the
  configuration is provided via the command line, environment variables, or a
  configuration file, Nomad has built-in functionality for configuration. This
  section details three common patterns for configuring tasks.
---

# Configuring Tasks

Most applications require some kind of local configuration. While command line
arguments are the simplest method, many applications require more complex
configurations provided via environment variables or configuration files. This
section explores how to configure Nomad jobs to support many common
configuration use cases.

## Command-line Arguments

Many tasks accept configuration via command-line arguments.  For example,
consider the [http-echo](https://github.com/hashicorp/http-echo) server which
is a small go binary that renders the provided text as a webpage. The binary
accepts two parameters:

* `-listen` - the `address:port` to listen on
* `-text` - the text to render as the HTML page

Outside of Nomad, the server is started like this:

```shell
$ http-echo -listen=":5678" -text="hello world"
```

The Nomad equivalent job file might look something like this:

```hcl
job "docs" {
  datacenters = ["dc1"]

  group "example" {
    task "server" {
      driver = "exec"

      config {
        command = "/bin/http-echo"
        args = [
          "-listen", ":5678",
          "-text", "hello world",
        ]
      }

      resources {
        network {
          mbits = 10
          port "http" {
            static = "5678"
          }
        }
      }
    }
  }
}
```

~> **This assumes** the <tt>http-echo</tt> binary is already installed and
   available in the system path. Nomad can also optionally fetch the binary
   using the <tt>artifact</tt> resource.

Nomad has many [drivers](/docs/drivers/index.html), and most support passing
arguments to their tasks via the `args` parameter. This parameter also supports
[Nomad interpolation](/docs/runtime/interpolation.html). For example, if you
wanted Nomad to dynamically allocate a high port to bind the service on instead
of relying on a static port for the previous job:

```hcl
job "docs" {
  datacenters = ["dc1"]

  group "example" {
    task "server" {
      driver = "exec"

      config {
        command = "/bin/http-echo"
        args = [
          "-listen", ":${NOMAD_PORT_http}",
          "-text", "hello world",
        ]
      }

      resources {
        network {
          mbits = 10
          port "http" {}
        }
      }
    }
  }
}
```

## Environment Variables

Some applications can be configured via environment variables. [The
Twelve-Factor App](https://12factor.net/config) document suggests configuring
applications through environment variables. Nomad supports custom environment
variables in two ways:

* Interpolation in an `env` stanza
* Templated in the a `template` stanza

### `env` stanza

Each task may have an `env` stanza which specifies environment variables:

```hcl
task "server" {
  env {
    my_key = "my-value"
  }
}
```

The `env` stanza also supports
[interpolation](/docs/runtime/interpolation.html):

```hcl
task "server" {
  env {
    LISTEN_PORT = "${NOMAD_PORT_http}"
  }
}
```

See the [`env`](/docs/job-specification/env.html.md) docs for details.


### Environment Templates

Nomad's [`template`][template] stanza can be used
to generate environment variables. Environment variables may be templated with
[Node attributes and metadata][nodevars], the contents of files on disk, Consul
keys, or secrets from Vault:

```hcl
template {
  data = <<EOH
LOG_LEVEL="{{key "service/geo-api/log-verbosity"}}"
API_KEY="{{with secret "secret/geo-api-key"}}{{.Data.key}}{{end}}"
CERT={{ file "path/to/cert.pem" | to JSON }}
NODE_ID="{{ env "node.unique.id" }}"
EOH

  destination = "secrets/config.env"
  env         = true
}
```

The template will be written to disk and then read as environment variables
before your task is launched.

## Configuration Files

Sometimes applications accept their configurations using files to support
complex data structures. Nomad supports downloading
[artifacts][artifact] and
[templating][template] them prior to launching
tasks.
This allows shipping of configuration files and other assets that the task
needs to run properly.

Here is an example job which pulls down a configuration file as an artifact and
templates it:

```hcl
job "docs" {
  datacenters = ["dc1"]

  group "example" {
    task "server" {
      driver = "exec"

      artifact {
        source      = "http://example.com/config.hcl.tmpl"
        destination = "local/config.hcl.tmpl"
      }

      template {
        source      = "local/config.hcl.tmpl"
        destination = "local/config.hcl"
      }

      config {
        command = "my-app"
        args = [
          "-config", "local/config.hcl",
        ]
      }
    }
  }
}
```

For more information on the artifact resource, please see the [artifact
documentation](/docs/job-specification/artifact.html).

[artifact]: /docs/job-specification/artifact.html "Nomad artifact Job Specification"
[nodevars]: /docs/runtime/interpolation.html#interpreted_node_vars "Nomad Node Variables"
[template]: /docs/job-specification/template.html "Nomad template Job Specification"
