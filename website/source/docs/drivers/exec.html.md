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
primitives of the operating system to limit the tasks access to resources. While
simple, since the `exec` driver  can invoke any command, it can be used to call
scripts or other wrappers which provide higher level features.

## Task Configuration

The `exec` driver supports the following configuration in the job spec:

* `command` - The command to execute. Must be provided.

* `artifact_source` – (Optional) Source location of an executable artifact. Must
  be accessible from the Nomad client. If you specify an `artifact_source` to be
  executed, you must reference it in the `command` as show in the examples below

* `checksum` - (Optional) The checksum type and value for the `artifact_source`
  image.  The format is `type:value`, where type is any of `md5`, `sha1`,
  `sha256`, or `sha512`, and the value is the computed checksum. If a checksum
  is supplied and does not match the downloaded artifact, the driver will fail
  to start

*   `args` - (Optional) A list of arguments to the optional `command`.
    References to environment variables or any [intepretable Nomad
    variables](/docs/jobspec/index.html#interpreted_vars) will be interpreted
    before launching the task. For example:

    ```
        args = ["$nomad.ip", "$MY_ENV", "$meta.foo"]
    ```

## Client Requirements

The `exec` driver can only be run when on Linux and running Nomad as root.
`exec` is limited to this configuration because currently isolation of resources
is only guaranteed on Linux. Further the host must have cgroups mounted properly
in order for the driver to work.

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
    checksum = "sha256:abd123445ds4555555555"
    command = "binary.bin"
  }
```

## Client Attributes

The `exec` driver will set the following client attributes:

* `driver.exec` - This will be set to "1", indicating the
  driver is available.

## Resource Isolation

The resource isolation provided varies by the operating system of
the client and the configuration.

On Linux, Nomad will use cgroups, and a chroot to isolate the
resources of a process and as such the Nomad agent must be run as root.

### Chroot
The chroot is populated with data in the following folders from the host
machine:

`["/bin", "/etc", "/lib", "/lib32", "/lib64", "/usr/bin", "/usr/lib", "/usr/share"]`
