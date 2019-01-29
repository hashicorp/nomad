---
layout: "docs"
page_title: "Device Plugins"
sidebar_current: "docs-devices"
description: |-
  Device Plugins are used to expose devices to tasks in Nomad.
---

# Device Plugins

Device plugins are used to detect and make devices available to tasks in Nomad.
Devices are physical hardware that exists on a node such as a GPU or an FPGA. By
having extensible device plugins, Nomad has the flexibility to support a broad
set of devices and allows the community to build additional device plugins as
needed.

The list of supported device plugins is provided on the left of this page.
Each device plugin documents its configuration and installation requirements,
the attributes it fingerprints, and the environment variables it exposes to
tasks.
