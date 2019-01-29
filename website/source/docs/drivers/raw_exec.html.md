---
layout: "docs"
page_title: "Drivers: Raw Exec"
sidebar_current: "docs-drivers-raw-exec"
description: |-
  The Raw Exec task driver simply fork/execs and provides no isolation.
---

# Raw Fork/Exec Driver

Name: `raw_exec`

The `raw_exec` driver is used to execute a command for a task without any
isolation. Further, the task is started as the same user as the Nomad process.
As such, it should be used with extreme care and is disabled by default.

## Task Configuration

```hcl
task "webservice" {
  driver = "raw_exec"

  config {
    command = "my-binary"
    args    = ["-flag", "1"]
  }
}  
```

The `raw_exec` driver supports the following configuration in the job spec:

* `command` - The command to execute. Must be provided. If executing a binary
  that exists on the host, the path must be absolute. If executing a binary that
  is downloaded from an [`artifact`](/docs/job-specification/artifact.html), the
  path can be relative from the allocation's root directory.

* `args` - (Optional) A list of arguments to the `command`. References
  to environment variables or any [interpretable Nomad
  variables](/docs/runtime/interpolation.html) will be interpreted before
  launching the task.

## Examples

To run a binary present on the Node:

```
task "example" {
  driver = "raw_exec"

  config {
    # When running a binary that exists on the host, the path must be absolute/
    command = "/bin/sleep"
    args    = ["1"]
  }
}
```

To execute a binary downloaded from an [`artifact`](/docs/job-specification/artifact.html):

```
task "example" {
  driver = "raw_exec"

  config {
    command = "name-of-my-binary"
  }

  artifact {
    source = "https://internal.file.server/name-of-my-binary"
    options {
      checksum = "sha256:abd123445ds4555555555"
    }
  }
}
```

## Client Requirements

The `raw_exec` driver can run on all supported operating systems. For security
reasons, it is disabled by default. To enable raw exec, the Nomad client
configuration must explicitly enable the `raw_exec` driver in the plugin's options:

```
plugin "raw_exec" {
  config {
    enabled = true
  }
}
```

Prior to Nomad 0.9, the client configuration would look like this (this syntax
will soon be deprecated):

```
client {
  options = {
    "driver.raw_exec.enable" = "1"
  }
}
```

## Plugin Options

* `enabled` - Specifies whether the driver should be enabled or disabled.
  Defaults to `false`.

* `no_cgroups` - Specifies whether the driver should not use
  cgroups to manage the process group launched by the driver. By default,
  cgroups are used to manage the process tree to ensure full cleanup of all
  processes started by the task. The driver only uses cgroups when Nomad is
  launched as root, on Linux and when cgroups are detected.

## Client Options

~> Note: client configuration options will soon be deprecated. Please use 
[plugin options][plugin-options] instead. See the [plugin stanza][plugin-stanza] documentation for more information.

* `driver.raw_exec.enable` - Specifies whether the driver should be enabled or
  disabled. Defaults to `false`.

* `driver.raw_exec.no_cgroups` - Specifies whether the driver should not use
  cgroups to manage the process group launched by the driver. By default,
  cgroups are used to manage the process tree to ensure full cleanup of all
  processes started by the task. The driver only uses cgroups when Nomad is
  launched as root, on Linux and when cgroups are detected.

## Client Attributes

The `raw_exec` driver will set the following client attributes:

* `driver.raw_exec` - This will be set to "1", indicating the driver is available.

## Resource Isolation

The `raw_exec` driver provides no isolation.

If the launched process creates a new process group, it is possible that Nomad
will leak processes on shutdown unless the application forwards signals
properly. Nomad will not leak any processes if cgroups are being used to manage
the process tree. Cgroups are used on Linux when Nomad is being run with
appropriate privileges, the cgroup system is mounted and the operator hasn't
disabled cgroups for the driver.

[plugin-options]: #plugin-options
[plugin-stanza]: /docs/configuration/plugin.html
