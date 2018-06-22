---
layout: "docs"
page_title: "Drivers: Exec"
sidebar_current: "docs-drivers-exec"
description: |-
  The Exec task driver is used to run binaries using OS isolation primitives.
---

# Isolated Fork/Exec Driver

Name: `exec`

The `exec` driver is used to simply execute a particular command for a task.
However, unlike [`raw_exec`](raw_exec.html) it uses the underlying isolation
primitives of the operating system to limit the task's access to resources. While
simple, since the `exec` driver  can invoke any command, it can be used to call
scripts or other wrappers which provide higher level features.

## Task Configuration

```hcl
task "webservice" {
  driver = "exec"

  config {
    command = "my-binary"
    args    = ["-flag", "1"]
  }
}
```

The `exec` driver supports the following configuration in the job spec:

* `command` - The command to execute. Must be provided. If executing a binary
  that exists on the host, the path must be absolute. If executing a binary that
  is downloaded from an [`artifact`](/docs/job-specification/artifact.html), the
  path can be relative from the allocations's root directory.

* `args` - (Optional) A list of arguments to the `command`. References
  to environment variables or any [interpretable Nomad
  variables](/docs/runtime/interpolation.html) will be interpreted before
  launching the task.

## Examples

To run a binary present on the Node:

```hcl
task "example" {
  driver = "exec"

  config {
    # When running a binary that exists on the host, the path must be absolute.
    command = "/bin/sleep"
    args    = ["1"]
  }
}
```

To execute a binary downloaded from an
[`artifact`](/docs/job-specification/artifact.html):

```hcl
task "example" {
  driver = "exec"

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

The `exec` driver can only be run when on Linux and running Nomad as root.
`exec` is limited to this configuration because currently isolation of resources
is only guaranteed on Linux. Further, the host must have cgroups mounted properly
in order for the driver to work.

If you are receiving the error:

```
* Constraint "missing drivers" filtered <> nodes
```

and using the exec driver, check to ensure that you are running Nomad as root.
This also applies for running Nomad in -dev mode.


## Client Attributes

The `exec` driver will set the following client attributes:

* `driver.exec` - This will be set to "1", indicating the driver is available.

## Resource Isolation

The resource isolation provided varies by the operating system of
the client and the configuration.

On Linux, Nomad will use cgroups, and a chroot to isolate the
resources of a process and as such the Nomad agent must be run as root.

### <a id="chroot"></a>Chroot
The chroot is populated with data in the following directories from the host
machine:

```
[
  "/bin",
  "/etc",
  "/lib",
  "/lib32",
  "/lib64",
  "/run/resolvconf",
  "/sbin",
  "/usr",
]
```

The task's chroot is populated by linking or copying the data from the host into
the chroot. Note that this can take considerable disk space. Since Nomad v0.5.3,
the client manages garbage collection locally which mitigates any issue this may
create.

This list is configurable through the agent client
[configuration file](/docs/configuration/client.html#chroot_env).
