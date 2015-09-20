---
layout: "docs"
page_title: "Server Configuration"
sidebar_current: "docs-config"
description: |-
  Nomad server configuration reference.
---

# Nomad Configuration

Outside of development mode, Nomad servers are configured using a file.
The format of this file is [HCL](https://github.com/hashicorp/hcl) or JSON.
An example configuration is shown below:

TODO: Document Nomad configuration. Examples below stolen from Vault.

```javascript
backend "consul" {
  address = "127.0.0.1:8500"
  path = "vault"
}

listener "tcp" {
  address = "127.0.0.1:8200"
  tls_disable = 1
}

telemetry {
  statsite_address = "127.0.0.1:8125"
  disable_hostname = true
}
```

After the configuration is written, use the `-config` flag with `vault server`
to specify where the configuration is.

## Reference

* `backend` (required) - Configures the storage backend where Nomad data
  is stored. There are multiple options available for storage backends,
  and they're documented below.

* `listener` (required) - Configures how Nomad is listening for API requests.
  "tcp" is currently the only option available. A full reference for the
   inner syntax is below.

* `disable_mlock` (optional) - A boolean. If true, this will disable the
  server from executing the `mlock` syscall to prevent memory from being
  swapped to disk. This is not recommended in production (see below).

* `telemetry` (optional)  - Configures the telemetry reporting system
  (see below).

* `default_lease_duration` (optional) - Configures the default lease
  duration for tokens and secrets, specified in hours. Default value
  is 30 days. This value cannot be larger than `max_lease_duration`.

* `max_lease_duration` (optional) - Configures the maximum possible
  lease duration for tokens and secrets, specified in hours. Default
  value is 30 days.

In production, you should only consider setting the `disable_mlock` option
on Linux systems that only use encrypted swap or do not use swap at all.
Nomad does not currently support memory locking on Mac OS X and Windows
and so the feature is automatically disabled on those platforms.  To give
the Nomad executable access to the `mlock` syscall on Linux systems:

```shell
sudo setcap cap_ipc_lock=+ep $(readlink -f $(which vault))
```
