---
layout: "docs"
page_title: "Drivers: Exec"
sidebar_current: "docs-drivers-exec"
description: |-
  The Exec task driver is used to run binaries using OS isolation primitives.
---

# Fork/Exec Driver

Name: `exec`

The `exec` driver is used to simply execute a particular command for a task.
This is the simplest driver and is extremely flexible. In particlar, because
it can invoke any command, it can be used to call scripts or other wrappers
which provide higher level features.

## Task Configuration

The `exec` driver supports the following configuration in the job spec:

* `command` - The command to execute. Must be provided.

* `args` - The argument list to the command, space seperated. Optional.

## Client Requirements

The `exec` driver has no special requirements and can run on all
supported operating systems. The resource isolation primitives vary
by OS.

## Client Attributes

The `exec` driver will set the following client attributes:

* `driver.exec` - This will always be set to "1", indicating the
  driver is available.

## Resource Isolation

The resource isolation provided varies by the operating system of
the client and the configuration.

On Linux, Nomad will attempt to use cgroups, namespaces, and chroot
to isolate the resources of a process. If the Nomad agent is not
running as root many of these mechanisms cannot be used.

As a baseline, the task driver will just execute the command
with no additional resource isolation if none are available.

