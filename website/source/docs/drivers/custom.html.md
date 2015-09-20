---
layout: "docs"
page_title: "Drivers: Custom"
sidebar_current: "docs-drivers-custom"
description: |-
  Create custom secret backends for Nomad.
---

# Custom Drivers

Nomad does not currently support the creation of custom secret backends.
The primary reason is because we want to ensure the core of Nomad is
secure before attempting any sort of plug-in system. We're interested
in supporting custom secret backends, but do not yet have a clear strategy
or timeline to do.

In the mean time, you can use the
[generic backend](/docs/secrets/generic/index.html) to support custom
data with custom leases.
