---
layout: "docs"
page_title: "Drivers: Qemu"
sidebar_current: "docs-drivers-qemu"
description: |-
  The Qemu task driver is used to run virtual machines using Qemu/KVM.
---

# Qemu Driver

Name: `qemu`

The `Qemu` driver provides a generic virtual machine runner. Qemu can utilize
the KVM kernel module to utilize hardware virtualization features and provide
great performance. Currently the `Qemu` driver can map a set of ports from the
host machine to the guest virtual machine, and provides configuration for
resource allocation. 

The `Qemu` driver can execute any regular `qemu` image (e.g. `qcow`, `img`,
`iso`), and is currently invoked with `qemu-system-x86_64`. 

## Task Configuration

The `Qemu` driver supports the following configuration in the job spec:

* `image_source` - **(Required)** The hosted location of the source Qemu image. Must be accessible
from the Nomad client, via HTTP. 
* `checksum` - **(Required)** The MD5 checksum of the `qemu` image. If the
checksums do not match, the `Qemu` diver will fail to start the image
* `accelerator` - (Optional) The type of accelerator to use in the invocation.
 If the host machine has `Qemu` installed with KVM support, users can specify `kvm` for the `accelerator`. Default is `tcg`
* `host_port` - **(Required)** Port on the host machine to forward to the guest
VM
* `guest_ports` - **(Optional)** Ports on the guest machine that are listening for
traffic from the host. These ports match up with any `ReservedPorts` requested
in the `Task` specification

## Client Requirements

The `Qemu` driver requires Qemu to be installed and in your systems `$PATH`.
The `image_source` must be accessible by the node running Nomad. This can be an 
internal source, private to your cluster, but it must be reachable by the client 
over HTTP. 

## Client Attributes

The `Qemu` driver will set the following client attributes:

* `driver.qemu` - Set to `1` if Qemu is found on the host node. Nomad determines
this by executing `qemu-system-x86_64 -version` on the host and parsing the output
* `driver.qemu.version` - Version of `qemu-system-x86_64, ex: `2.4.0`

## Resource Isolation

The resource isolation provided varies by the operating system of
the client and the configuration.

On Linux, Nomad will attempt to use cgroups, namespaces, and chroot
to isolate the resources of a process. If the Nomad agent is not
running as root many of these mechanisms cannot be used.

As a baseline, the Qemu images will be ran inside a virtual machine operated by
Qemu, providing a minimum amount of isolation.

