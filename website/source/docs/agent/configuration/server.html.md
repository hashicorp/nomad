---
layout: "docs"
page_title: "server Stanza - Agent Configuration"
sidebar_current: "docs-agent-configuration-server"
description: |-
  The "server" stanza configures the Nomad agent to operate in server mode to
  participate in scheduling decisions, register with service discovery, handle
  join failures, and more.
---

# `server` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>**server**</code>
    </td>
  </tr>
</table>


The `server` stanza configures the Nomad agent to operate in server mode to
participate in scheduling decisions, register with service discovery, handle
join failures, and more.

```hcl
server {
  enabled          = true
  bootstrap_expect = 3
  retry_join       = ["1.2.3.4", "5.6.7.8"]
}
```

## `server` Parameters

- `authoritative_region` `(string: "")` - Specifies the authoritative region, which
  provides a single source of truth for global configurations such as ACL Policies and
  global ACL tokens. Non-authoritative regions will replicate from the authoritative
  to act as a mirror. By default, the local region is assumed to be authoritative.

- `bootstrap_expect` `(int: required)` - Specifies the number of server nodes to
  wait for before bootstrapping. It is most common to use the odd-numbered
  integers `3` or `5` for this value, depending on the cluster size. A value of
  `1` does not provide any fault tolerance and is not recommended for production
  use cases.

- `data_dir` `(string: "[data_dir]/server")` - Specifies the directory to use -
  for server-specific data, including the replicated log. By default, this is -
  the top-level [data_dir](/docs/agent/configuration/index.html#data_dir)
  suffixed with "server", like `"/opt/nomad/server"`. This must be an absolute
  path.

- `enabled` `(bool: false)` - Specifies if this agent should run in server mode.
  All other server options depend on this value being set.

-
- `retry_join` `(array<string>: [])` - Specifies a list of server
  addresses to retry joining if the first attempt fails. This is similar to
  [`start_join`](#start_join), but only invokes if the initial join attempt
  fails. The list of addresses will be tried in the order specified, until one
  succeeds. After one succeeds, no further addresses will be contacted. This is
  useful for cases where we know the address will become available eventually.
  Use `retry_join` with an array as a replacement for `start_join`, **do not use
  both options**. See the [server address format](#server-address-format)
  section for more information on the format of the string. This field is
  deprecated in favor of [server_join](#server_join).

- `retry_interval` `(string: "30s")` - Specifies the time to wait between retry
  join attempts. This field is  deprecated in favor of [server_join](#server_join).

- `retry_max` `(int: 0)` - Specifies the maximum number of join attempts to be
  made before exiting with a return code of 1. By default, this is set to 0
  which is interpreted as infinite retries. This field is  deprecated in favor
  of [server_join](#server_join).

- `server_join` <code>([ServerJoin][server_join]: nil)</code> - Specifies
  configuration which is specific to retry joining Nomad servers.

- `start_join` `(array<string>: [])` - Specifies a list of server addresses to
  join on startup. If Nomad is unable to join with any of the specified
  addresses, agent startup will fail. See the
  [server address format](#server-address-format) section for more information
  on the format of the string. This field is  deprecated in favor of
  [server_join](#server_join).

- `upgrade_version` `(string: "")` - A custom version of the format X.Y.Z to use
  in place of the Nomad version when custom upgrades are enabled in Autopilot.
  For more information, see the [Autopilot Guide](/guides/cluster/autopilot.html).

### Server Address Format

This section describes the acceptable syntax and format for describing the
location of a Nomad server. There are many ways to reference a Nomad server,
including directly by IP address and resolving through DNS.

#### Directly via IP Address

It is possible to address another Nomad server using its IP address. This is
done in the `ip:port` format, such as:

```
1.2.3.4:5678
```

If the port option is omitted, it defaults to the Serf port, which is 4648
unless configured otherwise:

```
1.2.3.4 => 1.2.3.4:4648
```

#### Via Domains or DNS

It is possible to address another Nomad server using its DNS address. This is
done in the `address:port` format, such as:

```
nomad-01.company.local:5678
```

If the port option is omitted, it defaults to the Serf port, which is 4648
unless configured otherwise:

```
nomad-01.company.local => nomad-01.company.local:4648
```

## `server` Examples

### Common Setup

This example shows a common Nomad agent `server` configuration stanza. The two
IP addresses could also be DNS, and should point to the other Nomad servers in
the cluster

```hcl
server {
  enabled          = true
  bootstrap_expect = 3
  retry_join       = ["1.2.3.4", "5.6.7.8"]
}
```

### Configuring Data Directory

This example shows configuring a custom data directory for the server data.

```hcl
server {
  data_dir = "/opt/nomad/server"
}
```

### Automatic Bootstrapping

The Nomad servers can automatically bootstrap if Consul is configured. For a
more detailed explanation, please see the
[automatic Nomad bootstrapping documentation](/guides/cluster/automatic.html).

### Restricting Schedulers

This example shows restricting the schedulers that are enabled as well as the
maximum number of cores to utilize when participating in scheduling decisions:

```hcl
server {
  enabled            = true
  enabled_schedulers = ["batch", "service"]
  num_schedulers     = 7
}
```

[encryption]: /docs/agent/encryption.html "Nomad Agent Encryption"
