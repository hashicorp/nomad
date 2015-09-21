---
layout: "docs"
page_title: "Drivers: Java"
sidebar_current: "docs-drivers-java"
description: |-
  The Java task driver is used to run Jars using the JVM.
---

# Java Driver

Name: `java`

The `Java` driver is used to execute Java applications packaged into a Java Jar 
file. The driver currently requires the Jar file be accessbile via
HTTP from the Nomad client. 

## Task Configuration

The `java` driver supports the following configuration in the job spec:

* `jar_source` - **(Required)** The hosted location of the source Jar file. Must be accessible
from the Nomad client, via HTTP

* `args` - (Optional) The argument list for the `java` command, space seperated. 

## Client Requirements

The `java` driver requires Java to be installed and in your systems `$PATH`.
The `jar_source` must be accessible by the node running Nomad. This can be an 
internal source, private to your cluster, but it must be reachable by the client 
over HTTP. 

The resource isolation primitives vary by OS.

## Client Attributes

The `java` driver will set the following client attributes:

* `driver.java` - This will always be set to "1", indicating the
  driver is available.

## Resource Isolation

The resource isolation provided varies by the operating system of
the client and the configuration.

On Linux, Nomad will attempt to use cgroups, namespaces, and chroot
to isolate the resources of a process. If the Nomad agent is not
running as root many of these mechanisms cannot be used.

As a baseline, the task driver will just execute the command
with no additional resource isolation if none are available.

