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

* `command` - The command to execute. Must be provided.

* `args` - The argument list to the command, space seperated. Optional.

## Client Requirements

The `exec` driver can run on all supported operating systems but to provide
proper isolation the client must be run as root on non-Windows operating systems.
Further, to support cgroups, `/sys/fs/cgroups/` must be mounted.

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
