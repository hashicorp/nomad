---
layout: "docs"
page_title: "client Stanza - Agent Configuration"
sidebar_current: "docs-configuration-client"
description: |-
  The "client" stanza configures the Nomad agent to accept jobs as assigned by
  the Nomad server, join the cluster, and specify driver-specific configuration.
---

# `client` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>**client**</code>
    </td>
  </tr>
</table>

The `client` stanza configures the Nomad agent to accept jobs as assigned by
the Nomad server, join the cluster, and specify driver-specific configuration.

```hcl
client {
  enabled = true
  servers = ["1.2.3.4:4647", "5.6.7.8:4647"]
}
```

## `client` Parameters

- `alloc_dir` `(string: "[data_dir]/alloc")` - Specifies the directory to use
  for allocation data. By default, this is the top-level
  [data_dir](/docs/configuration/index.html#data_dir) suffixed with
  "alloc", like `"/opt/nomad/alloc"`. This must be an absolute path

- `chroot_env` <code>([ChrootEnv](#chroot_env-parameters): nil)</code> -
  Specifies a key-value mapping that defines the chroot environment for jobs
  using the Exec and Java drivers.

- `enabled` `(bool: false)` - Specifies if client mode is enabled. All other
  client configuration options depend on this value.

- `max_kill_timeout` `(string: "30s")` - Specifies the maximum amount of time a
  job is allowed to wait to exit. Individual jobs may customize their own kill
  timeout, but it may not exceed this value.

- `meta` `(map[string]string: nil)` - Specifies a key-value map that annotates
  with user-defined metadata.

- `network_interface` `(string: varied)` - Specifies the name of the interface
  to force network fingerprinting on. When run in dev mode, this defaults to the
  loopback interface. When not in dev mode, the interface attached to the
  default route is used. All IP addresses except those scoped local for IPV6 on
  the chosen interface are fingerprinted. The scheduler chooses from those IP
  addresses when allocating ports for tasks.

- `network_speed` `(int: 0)` - Specifies an override for the network link speed.
  This value, if set, overrides any detected or defaulted link speed. Most
  clients can determine their speed automatically, and thus in most cases this
  should be left unset.

- `cpu_total_compute` `(int: 0)` - Specifies an override for the total CPU
  compute. This value should be set to `# Cores * Core MHz`. For example, a
  quad-core running at 2 GHz would have a total compute of 8000 (4 * 2000). Most
  clients can determine their total CPU compute automatically, and thus in most
  cases this should be left unset.

- `memory_total_mb` `(int:0)` - Specifies an override for the total memory. If set,
  this value overrides any detected memory.

- `node_class` `(string: "")` - Specifies an arbitrary string used to logically
  group client nodes by user-defined class. This can be used during job
  placement as a filter.

- `options` <code>([Options](#options-parameters): nil)</code> - Specifies a
  key-value mapping of internal configuration for clients, such as for driver
  configuration.

- `reserved` <code>([Reserved](#reserved-parameters): nil)</code> - Specifies
  that Nomad should reserve a portion of the node's resources from receiving
  tasks. This can be used to target a certain capacity usage for the node. For
  example, 20% of the node's CPU could be reserved to target a CPU utilization
  of 80%.

- `servers` `(array<string>: [])` - Specifies an array of addresses to the Nomad
  servers this client should join. This list is used to register the client with
  the server nodes and advertise the available resources so that the agent can
  receive work. This may be specified as an IP address or DNS, with or without
  the port. If the port is omitted, the default port of `4647` is used.

- `server_join` <code>([server_join][server-join]: nil)</code> - Specifies
  how the Nomad client will connect to Nomad servers. The `start_join` field
  is not supported on the client. The retry_join fields may directly specify
  the server address or use go-discover syntax for auto-discovery. See the
  documentation for more detail.

- `state_dir` `(string: "[data_dir]/client")` - Specifies the directory to use
 to store client state. By default, this is - the top-level
 [data_dir](/docs/configuration/index.html#data_dir) suffixed with
 "client", like `"/opt/nomad/client"`. This must be an absolute path.

- `gc_interval` `(string: "1m")` - Specifies the interval at which Nomad
  attempts to garbage collect terminal allocation directories.

- `gc_disk_usage_threshold` `(float: 80)` - Specifies the disk usage percent which
  Nomad tries to maintain by garbage collecting terminal allocations.

- `gc_inode_usage_threshold` `(float: 70)` - Specifies the inode usage percent
  which Nomad tries to maintain by garbage collecting terminal allocations.

- `gc_max_allocs` `(int: 50)` - Specifies the maximum number of allocations
  which a client will track before triggering a garbage collection of terminal
  allocations. This will *not* limit the number of allocations a node can run at
  a time, however after `gc_max_allocs` every new allocation will cause terminal
  allocations to be GC'd.

- `gc_parallel_destroys` `(int: 2)` - Specifies the maximum number of
  parallel destroys allowed by the garbage collector. This value should be
  relatively low to avoid high resource usage during garbage collections.

- `no_host_uuid` `(bool: true)` - By default a random node UUID will be
  generated, but setting this to `false` will use the system's UUID. Before
  Nomad 0.6 the default was to use the system UUID.

### `chroot_env` Parameters

Drivers based on [isolated fork/exec](/docs/drivers/exec.html) implement file
system isolation using chroot on Linux. The `chroot_env` map allows the chroot
environment to be configured using source paths on the host operating system.
The mapping format is:

```text
source_path -> dest_path
```

The following example specifies a chroot which contains just enough to run the
`ls` utility:

```hcl
client {
  chroot_env {
    "/bin/ls"           = "/bin/ls"
    "/etc/ld.so.cache"  = "/etc/ld.so.cache"
    "/etc/ld.so.conf"   = "/etc/ld.so.conf"
    "/etc/ld.so.conf.d" = "/etc/ld.so.conf.d"
    "/lib"              = "/lib"
    "/lib64"            = "/lib64"
  }
}
```

When `chroot_env` is unspecified, the `exec` driver will use a default chroot
environment with the most commonly used parts of the operating system. Please
see the [Nomad `exec` driver documentation](/docs/drivers/exec.html#chroot) for
the full list.

### `options` Parameters

The following is not an exhaustive list of options for only the Nomad
client. To find the options supported by each individual Nomad driver, please
see the [drivers documentation](/docs/drivers/index.html).

- `"driver.whitelist"` `(string: "")` - Specifies a comma-separated list of
  whitelisted drivers . If specified, drivers not in the whitelist will be
  disabled. If the whitelist is empty, all drivers are fingerprinted and enabled
  where applicable.

    ```hcl
    client {
      options = {
        "driver.whitelist" = "docker,qemu"
      }
    }
    ```

- `"driver.blacklist"` `(string: "")` - Specifies a comma-separated list of
  blacklisted drivers . If specified, drivers in the blacklist will be
  disabled.

    ```hcl
    client {
      options = {
        "driver.blacklist" = "docker,qemu"
      }
    }
    ```

- `"env.blacklist"` `(string: see below)` - Specifies a comma-separated list of
  environment variable keys not to pass to these tasks. Nomad passes the host
  environment variables to `exec`, `raw_exec` and `java` tasks. If specified,
  the defaults are overridden. If a value is provided, **all** defaults are
  overridden (they are not merged).

    ```hcl
    client {
      options = {
        "env.blacklist" = "MY_CUSTOM_ENVVAR"
      }
    }
    ```

    The default list is:

    ```text
    CONSUL_TOKEN
    VAULT_TOKEN
    AWS_ACCESS_KEY_ID
    AWS_SECRET_ACCESS_KEY
    AWS_SESSION_TOKEN
    GOOGLE_APPLICATION_CREDENTIALS
    ```

- `"user.blacklist"` `(string: see below)` - Specifies a comma-separated
  blacklist of usernames for which a task is not allowed to run. This only
  applies if the driver is included in `"user.checked_drivers"`. If a value is
  provided, **all** defaults are overridden (they are not merged).

    ```hcl
    client {
      options = {
        "user.blacklist" = "root,ubuntu"
      }
    }
    ```

    The default list is:

    ```text
    root
    Administrator
    ```

- `"user.checked_drivers"` `(string: see below)` - Specifies a comma-separated
  list of drivers for which to enforce the `"user.blacklist"`. For drivers using
  containers, this enforcement is usually unnecessary. If a value is provided,
  **all** defaults are overridden (they are not merged).

    ```hcl
    client {
      options = {
        "user.checked_drivers" = "exec,raw_exec"
      }
    }
    ```

    The default list is:

    ```text
    exec
    qemu
    java
    ```

- `"fingerprint.whitelist"` `(string: "")` - Specifies a comma-separated list of
  whitelisted fingerprinters. If specified, any fingerprinters not in the
  whitelist will be disabled. If the whitelist is empty, all fingerprinters are
  used.

    ```hcl
    client {
      options = {
        "fingerprint.whitelist" = "network"
      }
    }
    ```

- `"fingerprint.blacklist"` `(string: "")` - Specifies a comma-separated list of
  blacklisted fingerprinters. If specified, any fingerprinters in the blacklist
  will be disabled.

    ```hcl
    client {
      options = {
        "fingerprint.blacklist" = "network"
      }
    }
    ```

- `"fingerprint.network.disallow_link_local"` `(string: "false")` - Specifies
  whether the network fingerprinter should ignore link-local addresses in the
  case that no globally routable address is found. The fingerprinter will always
  prefer globally routable addresses.

    ```hcl
    client {
      options = {
        "fingerprint.network.disallow_link_local" = "true"
      }
    }
    ```

### `reserved` Parameters

- `cpu` `(int: 0)` - Specifies the amount of CPU to reserve, in MHz.

- `memory` `(int: 0)` - Specifies the amount of memory to reserve, in MB.

- `disk` `(int: 0)` - Specifies the amount of disk to reserve, in MB.

- `reserved_ports` `(string: "")` - Specifies a comma-separated list of ports to
  reserve on all fingerprinted network devices. Ranges can be specified by using
  a hyphen separated the two inclusive ends.

## `client` Examples

### Common Setup

This example shows the most basic configuration for a Nomad client joined to a
cluster.

```hcl
client {
  enabled = true
  server_join {
    retry_join = [ "1.1.1.1", "2.2.2.2" ]
    retry_max = 3
    retry_interval = "15s"
  }
}
```

### Reserved Resources

This example shows a sample configuration for reserving resources to the client.
This is useful if you want to allocate only a portion of the client's resources
to jobs.

```hcl
client {
  enabled = true

  reserved {
    cpu            = 500
    memory         = 512
    disk           = 1024
    reserved_ports = "22,80,8500-8600"
  }
}
```

### Custom Metadata, Network Speed, and Node Class

This example shows a client configuration which customizes the metadata, network
speed, and node class.

```hcl
client {
  enabled       = true
  network_speed = 500
  node_class    = "prod"

  meta {
    "owner" = "ops"
  }
}
```
[server-join]: /docs/configuration/server_join.html "Server Join"
