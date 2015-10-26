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
However unlike [`raw_exec`](raw_exec.html) it uses the underlying isolation
primitives of the operating system to limit the tasks access to resources. While
simple, since the `exec` driver  can invoke any command, it can be used to call
scripts or other wrappers which provide higher level features.

## Task Configuration

The `exec` driver supports the following configuration in the job spec:

* `command` - (Required) The command to execute. Must be provided.
* `artifact_source` – (Optional) Source location of an executable artifact. Must be accessible
from the Nomad client. If you specify an `artifact_source` to be executed, you
must reference it in the `command` as show in the examples below
* `args` - The argument list to the command, space seperated. Optional.

## Client Requirements

The `exec` driver can run on all supported operating systems but to provide
proper isolation the client must be run as root on non-Windows operating systems.
Further, to support cgroups, `/sys/fs/cgroups/` must be mounted.

You must specify a `command` to be executed. Optionally you can specify an
`artifact_source` to be downloaded as well. Any `command` is assumed to be present on the 
running client, or a downloaded artifact.

## Examples

To run a binary present on the Node:

```
  config {
    command = "/bin/sleep"
    args = 1
  }
```

To execute a binary specified by `artifact_source`:

```
  config {
    artifact_source = "https://dl.dropboxusercontent.com/u/1234/binary.bin"
    command = "$NOMAD_TASK_DIR/binary.bin"
  }
```

## Client Attributes

The `exec` driver will set the following client attributes:

* `driver.exec` - This will be set to "1", indicating the
  driver is available.

## Resource Isolation

The resource isolation provided varies by the operating system of
the client and the configuration.

On Linux, Nomad will use cgroups, namespaces, and chroot to isolate the
resources of a process and as such the Nomad agent must be run as root.

On Windows, the task driver will just execute the command with no additional
resource isolation.
