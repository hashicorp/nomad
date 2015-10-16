---
layout: "docs"
page_title: "Drivers: Rkt"
sidebar_current: "docs-drivers-rkt"
description: |-
  The Rkt task driver is used to run application containers using Rkt.
---

# Rkt Driver

Name: `rkt`

The `Rkt` driver provides an interface for using CoreOS Rkt for running
application containers. Currently, the driver supports launching
containers.

## Task Configuration

The `Rkt` driver supports the following configuration in the job spec:

* `trust_prefix` - **(Optional)** The trust prefix to be passed to rkt. Must be reachable from
the box running the nomad agent. If not specified, the image is run without
verifying the image signature.
* `image` - **(Required)** The image to run which may be specified by name,
hash, ACI address or docker registry.
* `command` - **(Optional**) A command to execute on the ACI.
* `args` - **(Optional**) A string of args to pass into the image.

## Task Directories

The `Rkt` driver does not currently support mounting the `alloc/` and `local/`
directory. It is currently blocked by this [Rkt
issue](https://github.com/coreos/rkt/issues/761).

## Client Requirements

The `Rkt` driver requires rkt to be installed and in your systems `$PATH`.
The `trust_prefix` must be accessible by the node running Nomad. This can be an
internal source, private to your cluster, but it must be reachable by the client
over HTTP.

## Client Attributes

The `Rkt` driver will set the following client attributes:

* `driver.rkt` - Set to `1` if Rkt is found on the host node. Nomad determines
this by executing `rkt version` on the host and parsing the output
* `driver.rkt.version` - Version of `rkt` eg: `0.8.1`
* `driver.rkt.appc.version` - Version of `appc` that `rkt` is using eg: `0.8.1`

## Resource Isolation

This driver does not support any resource isolation as of now.
