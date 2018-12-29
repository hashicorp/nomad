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

- `enabled_schedulers` `(array<string>: [all])` - Specifies which sub-schedulers
  this server will handle. This can be used to restrict the evaluations that
  worker threads will dequeue for processing.

- `encrypt` `(string: "")` - Specifies the secret key to use for encryption of
  Nomad server's gossip network traffic. This key must be 16 bytes that are
  base64-encoded. The provided key is automatically persisted to the data
  directory and loaded automatically whenever the agent is restarted. This means
  that to encrypt Nomad server's gossip protocol, this option only needs to be
  provided once on each agent's initial startup sequence. If it is provided
  after Nomad has been initialized with an encryption key, then the provided key
  is ignored and a warning will be displayed. See the
  [Nomad encryption documentation][encryption] for more details on this option
  and its impact on the cluster.

- `node_gc_threshold` `(string: "24h")` - Specifies how long a node must be in a
  terminal state before it is garbage collected and purged from the system. This
  is specified using a label suffix like "30s" or "1h".

- `job_gc_threshold` `(string: "4h")` - Specifies the minimum time a job must be
  in the terminal state before it is eligible for garbage collection. This is
  specified using a label suffix like "30s" or "1h".

- `eval_gc_threshold` `(string: "1h")` - Specifies the minimum time an
  evaluation must be in the terminal state before it is eligible for garbage
  collection. This is specified using a label suffix like "30s" or "1h".

- `deployment_gc_threshold` `(string: "1h")` - Specifies the minimum time a
  deployment must be in the terminal state before it is eligible for garbage
  collection. This is specified using a label suffix like "30s" or "1h".

- `heartbeat_grace` `(string: "10s")` - Specifies the additional time given as a
  grace period beyond the heartbeat TTL of nodes to account for network and
  processing delays as well as clock skew. This is specified using a label
  suffix like "30s" or "1h".

- `min_heartbeat_ttl` `(string: "10s")` - Specifies the minimum time between
  node heartbeats. This is used as a floor to prevent excessive updates. This is
  specified using a label suffix like "30s" or "1h". Lowering the minimum TTL is
  a tradeoff as it lowers failure detection time of nodes at the tradeoff of
  false positives and increased load on the leader.

- `max_heartbeats_per_second` `(float: 50.0)` - Specifies the maximum target
  rate of heartbeats being processed per second. This allows the TTL to be
  increased to meet the target rate. Increasing the maximum heartbeats per
  second is a tradeoff as it lowers failure detection time of nodes at the
  tradeoff of false positives and increased load on the leader.

- `num_schedulers` `(int: [num-cores])` - Specifies the number of parallel
  scheduler threads to run. This can be as many as one per core, or `0` to
  disallow this server from making any scheduling decisions. This defaults to
  the number of CPU cores.

- `protocol_version` `(int: 1)` - Specifies the Nomad protocol version to use
  when communicating with other Nomad servers. This value is typically not
  required as the agent internally knows the latest version, but may be useful
  in some upgrade scenarios.

- `rejoin_after_leave` `(bool: false)` - Specifies if Nomad will ignore a
  previous leave and attempt to rejoin the cluster when starting. By default,
  Nomad treats leave as a permanent intent and does not attempt to join the
  cluster again when starting. This flag allows the previous state to be used to
  rejoin the cluster.

- `retry_join` `(array<string>: [])` - Specifies a list of server addresses to
  retry joining if the first attempt fails. This is similar to
  [`start_join`](#start_join), but only invokes if the initial join attempt
  fails. The list of addresses will be tried in the order specified, until one
  succeeds. After one succeeds, no further addresses will be contacted. This is
  useful for cases where we know the address will become available eventually.
  Use `retry_join` with an array as a replacement for `start_join`, **do not use
  both options**. See the [server address format](#server-address-format)
  section for more information on the format of the string.

- `retry_interval` `(string: "30s")` - Specifies the time to wait between retry
  join attempts.

- `retry_max` `(int: 0)` - Specifies the maximum number of join attempts to be
  made before exiting with a return code of 1. By default, this is set to 0
  which is interpreted as infinite retries.

- `start_join` `(array<string>: [])` - Specifies a list of server addresses to
  join on startup. If Nomad is unable to join with any of the specified
  addresses, agent startup will fail. See the
  [server address format](#server-address-format) section for more information
  on the format of the string.

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
