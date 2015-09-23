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
Default is `tcg`. If the host machine has `Qemu` installed with KVM support,
users can specify `kvm` for the `accelerator`
* `host_port` - **(Required)** The hosted location of the source Jar file. Must be accessible
from the Nomad client, via HTTP
* `guest_port` - **(Required)** The hosted location of the source Jar file. Must be accessible
from the Nomad client, via HTTP

## Client Requirements

The `java` driver requires Java to be installed and in your systems `$PATH`.
The `jar_source` must be accessible by the node running Nomad. This can be an 
internal source, private to your cluster, but it must be reachable by the client 
over HTTP. 

## Client Attributes

The `java` driver will set the following client attributes:

* `driver.java` - This will always be set to "1", indicating the
  driver is available.
* `driver.java.version` - Version of Java, ex: `1.6.0_65`
* `driver.java.runtime` - Runtime version, ex: `Java(TM) SE Runtime Environment (build 1.6.0_65-b14-466.1-11M4716)`
* `driver.java.vm` - Virtual Machine information, ex: `Java HotSpot(TM) 64-Bit Server VM (build 20.65-b04-466.1, mixed mode)`

## Resource Isolation

The resource isolation provided varies by the operating system of
the client and the configuration.

On Linux, Nomad will attempt to use cgroups, namespaces, and chroot
to isolate the resources of a process. If the Nomad agent is not
running as root many of these mechanisms cannot be used.

As a baseline, the Java jars will be ran inside a Java Virtual Machine,
providing a minimum amount of isolation.


