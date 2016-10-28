---
layout: "docs"
page_title: "Drivers: Qemu"
sidebar_current: "docs-drivers-qemu"
description: |-
  The Qemu task driver is used to run virtual machines using Qemu/KVM.
---

# Qemu Driver

Name: `qemu`

The `qemu` driver provides a generic virtual machine runner. Qemu can utilize
the KVM kernel module to utilize hardware virtualization features and provide
great performance. Currently the `qemu` driver can map a set of ports from the
host machine to the guest virtual machine, and provides configuration for
resource allocation.

The `qemu` driver can execute any regular `qemu` image (e.g. `qcow`, `img`,
`iso`), and is currently invoked with `qemu-system-x86_64`.

The driver requires the image to be accessible from the Nomad client via the
[`artifact` downloader](/docs/job-specification/artifact.html).

## Task Configuration

```hcl
task "webservice" {
  driver = "qemu"

  config {
    image_path  = "/path/to/my/linux.img"
    accelerator = "kvm"
    args        = ["-nodefaults", "-nodefconfig"]
  }
}  
```

The `qemu` driver supports the following configuration in the job spec:

* `image_path` - The path to the downloaded image. In most cases this will just
  be the name of the image. However, if the supplied artifact is an archive that
  contains the image in a subfolder, the path will need to be the relative path
  (`subdir/from_archive/my.img`).

* `accelerator` - (Optional) The type of accelerator to use in the invocation.
  If the host machine has `qemu` installed with KVM support, users can specify
  `kvm` for the `accelerator`. Default is `tcg`.

* `port_map` - (Optional) A key-value map of port labels.

    ```hcl
    config {
      # Forward the host port with the label "db" to the guest VM's port 6539.
      port_map {
        db = 6539
      }
    }
    ```

* `args` - (Optional) A list of strings that is passed to qemu as command line
  options.

## Examples

A simple config block to run a `qemu` image:

```
task "virtual" {
  driver = "qemu"

  config {
    image_path  = "local/linux.img"
    accelerator = "kvm"
    args        = ["-nodefaults", "-nodefconfig"]
  }

  # Specifying an artifact is required with the "qemu"
  # driver. This is the # mechanism to ship the image to be run.
  artifact {
    source = "https://internal.file.server/linux.img"

    options {
      checksum = "md5:123445555555555"
    }
  }
```

## Client Requirements

The `qemu` driver requires Qemu to be installed and in your system's `$PATH`.
The task must also specify at least one artifact to download, as this is the only
way to retrieve the image being run.

## Client Attributes

The `qemu` driver will set the following client attributes:

* `driver.qemu` - Set to `1` if Qemu is found on the host node. Nomad determines
this by executing `qemu-system-x86_64 -version` on the host and parsing the output
* `driver.qemu.version` - Version of `qemu-system-x86_64`, ex: `2.4.0`

Here is an example of using these properties in a job file:

```hcl
job "docs" {
  # Only run this job where the qemu version is higher than 1.2.3.
  constraint {
    attribute = "${driver.qemu.version}"
    operator  = ">"
    value     = "1.2.3"
  }
}
```

## Resource Isolation

Nomad uses Qemu to provide full software virtualization for virtual machine
workloads. Nomad can use Qemu KVM's hardware-assisted virtualization to deliver
better performance.

Virtualization provides the highest level of isolation for workloads that
require additional security, and resource use is constrained by the Qemu
hypervisor rather than the host kernel. VM network traffic still flows through
the host's interface(s).
