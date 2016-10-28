---
layout: "docs"
page_title: "Configuring Tasks - Operating a Job"
sidebar_current: "docs-operating-a-job-configuring-tasks"
description: |-
  Most applications require some kind of configuration. Whether this
  configuration is provided via the command line or via a configuration file,
  Nomad has built-in functionality for configuration. This section details two
  common patterns for configuring tasks.
---

# Configuring Tasks

Most applications require some kind of configuration. The simplest way is via
command-line arguments, but often times tasks consume complex configurations via
config files. This section explores how to configure Nomad jobs to support many
common configuration use cases.

## Command-line Arguments

Many tasks accept configuration via command-line arguments that do not change
over time.

For example, consider the [http-echo](https://github.com/hashicorp/http-echo)
server which is a small go binary that renders the provided text as a webpage. The binary accepts two parameters:

* `-listen` - the address:port to listen on
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

~> **This assumes** the <tt>http-echo</tt> binary is already installed and available in the system path. Nomad can also optionally fetch the binary using the <tt>artifact</tt> resource.

Nomad has many [drivers](/docs/drivers/index.html), and most support passing
arguments to their tasks via the `args` parameter. This option also optionally
accepts [Nomad interpolation](/docs/runtime/interpolation.html). For example, if
you wanted Nomad to dynamically allocate a high port to bind the service on
intead of relying on a static port for the previous job:

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

## Configuration Files

Not all applications accept their configuration via command-line flags.
Sometimes applications accept their configurations using files instead. Nomad
supports downloading [artifacts](/docs/job-specification/artifact.html) prior to
launching tasks. This allows shipping of configuration files and other assets
that the task needs to run properly.

Here is an example job which pulls down a configuration file as an artifact:

```hcl
job "docs" {
  datacenters = ["dc1"]

  group "example" {
    task "server" {
      driver = "exec"

      artifact {
        source      = "http://example.com/config.hcl"
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

For more information on the artifact resource, please see the [artifact documentation](/docs/job-specification/artifact.html).
