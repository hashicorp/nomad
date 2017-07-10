---
layout: "docs"
page_title: "Agent Configuration"
sidebar_current: "docs-agent-configuration"
description: |-
  Learn about the configuration options available for the Nomad agent.
---

# Agent Configuration

Nomad agents have a variety of parameters that can be specified via
configuration files or command-line flags. Configuration files are written in
[HCL][hcl]. Nomad can read and combine parameters from multiple configuration
files or directories to configure the Nomad agent.

## Load Order and Merging

The Nomad agent supports multiple configuration files, which can be provided
using the `-config` CLI flag. The flag can accept either a file or folder:

```shell
$ nomad agent -config=server.conf -config=/etc/nomad -config=extra.json
```

This will load configuration from `server.conf`, from `.hcl` and `.json` files
under `/etc/nomad`, and finally from `extra.json`. Files are loaded and merged
in lexicographical order. Directories are not loaded recursively.

As each file is processed, its contents are merged into the existing
configuration. When merging, any non-empty values from the latest config file
will append or replace parameters in the current configuration. An empty value
means `""` for strings, `0` for integer or float values, and `false` for
booleans. Since empty values are ignored you cannot disable an parameter like
`server` mode once you've enabled it.

Here is an example Nomad agent configuration that runs in both client and server
mode.

```hcl
bind_addr = "0.0.0.0" # the default

data_dir  = "/var/lib/nomad"

advertise {
  # Defaults to the node's hostname. If the hostname resolves to a loopback
  # address you must manually configure advertise addresses.
  http = "1.2.3.4"
  rpc  = "1.2.3.4"
  serf = "1.2.3.4:5648" # non-default ports may be specified
}

server {
  enabled          = true
  bootstrap_expect = 3
}

client {
  enabled       = true
  network_speed = 10
  options {
    "driver.raw_exec.enable" = "1"
  }
}

consul {
  address = "1.2.3.4:8500"
}

```

~> Note that it is strongly recommended **not** to operate a node as both
`client` and `server`, although this is supported to simplify development and
testing.

## General Parameters

- `addresses` `(Addresses: see below)` - Specifies the bind address for
  individual network services. Any values configured in this stanza take
  precedence over the default [bind_addr](#bind_addr).
  The values support [go-sockaddr/template format][go-sockaddr/template].

  - `http` - The address the HTTP server is bound to. This is the most common
    bind address to change.

  - `rpc` - The address to bind the internal RPC interfaces to. Should be
    exposed only to other cluster members if possible.

  - `serf` - The address used to bind the gossip layer to. Both a TCP and UDP
    listener will be exposed on this address. Should be exposed only to other
    cluster members if possible.

- `advertise` `(Advertise: see below)` - Specifies the advertise address for
  individual network services. This can be used to advertise a different address
  to the peers of a server or a client node to support more complex network
  configurations such as NAT. This configuration is optional, and defaults to
  the bind address of the specific network service if it is not provided. Any
  values configured in this stanza take precedence over the default
  [bind_addr](#bind_addr). If the bind address is `0.0.0.0` then the first
  private IP found is advertised. You may advertise an alternate port as well.
  The values support [go-sockaddr/template format][go-sockaddr/template].

  - `http` - The address to advertise for the HTTP interface. This should be
    reachable by all the nodes from which end users are going to use the Nomad
    CLI tools.

  - `rpc` - The address to advertise for the RPC interface. This address should
    be reachable by all of the agents in the cluster.

  - `serf` - The address advertised for the gossip layer. This address must be
    reachable from all server nodes. It is not required that clients can reach
    this address.

- `bind_addr` `(string: "0.0.0.0")` - Specifies which address the Nomad
  agent should bind to for network services, including the HTTP interface as
  well as the internal gossip protocol and RPC mechanism. This should be
  specified in IP format, and can be used to easily bind all network services to
  the same address. It is also possible to bind the individual services to
  different addresses using the [addresses](#addresses) configuration option.
  Dev mode (`-dev`) defaults to localhost.
  The value supports [go-sockaddr/template format][go-sockaddr/template].

- `client` <code>([Client][client]: nil)</code> - Specifies configuration which is specific to the Nomad client.

- `consul` <code>([Consul][consul]: nil)</code> - Specifies configuration for
  connecting to Consul.

- `datacenter` `(string: "dc1")` - Specifies the data center of the local agent.
  All members of a datacenter should share a local LAN connection.

- `data_dir` `(string: required)` - Specifies a local directory used to store
  agent state. Client nodes use this directory by default to store temporary
  allocation data as well as cluster information. Server nodes use this
  directory to store cluster state, including the replicated log and snapshot
  data. This must be specified as an absolute path.

- `disable_anonymous_signature` `(bool: false)` - Specifies if Nomad should
  provide an anonymous signature for de-duplication with the update check.

- `disable_update_check` `(bool: false)` - Specifies if Nomad should not check for updates and security bulletins.

- `enable_debug` `(bool: false)` - Specifies if the debugging HTTP endpoints
  should be enabled. These endpoints can be used with profiling tools to dump
  diagnostic information about Nomad's internals.

- `enable_syslog` `(bool: false)` - Specifies if the agent should log to syslog.
  This option only works on Unix based systems.

- `http_api_response_headers` `(map<string|string>: nil)` - Specifies
  user-defined headers to add to the HTTP API responses.

- `leave_on_interrupt` `(bool: false)` - Specifies if the agent should
  gracefully leave when receiving the interrupt signal. By default, the agent
  will exit forcefully on any signal.

- `leave_on_terminate` `(bool: false)` - Specifies if the agent should
  gracefully leave when receiving the terminate signal. By default, the agent
  will exit forcefully on any signal.

- `log_level` `(string: "INFO")` - Specifies  the verbosity of logs the Nomad
  agent will output. Valid log levels include `WARN`, `INFO`, or `DEBUG` in
  increasing order of verbosity.

- `name` `(string: [hostname])` - Specifies the name of the local node. This
  value is used to identify individual nodes in a given datacenter and must be
  unique per-datacenter.

- `ports` `(Port: see below)` - Specifies the network ports used for different
  services required by the Nomad agent.

  - `http` - The port used to run the HTTP server.

  - `rpc` - The port used for internal RPC communication between
    agents and servers, and for inter-server traffic for the consensus algorithm
    (raft).

  - `serf` - The port used for the gossip protocol for cluster
    membership. Both TCP and UDP should be routable between the server nodes on
    this port.

    The default values are:

    ```hcl
    ports {
      http = 4646
      rpc  = 4647
      serf = 4648
    }
    ```

- `region` `(string: "global")` - Specifies the region the Nomad agent is a
  member of. A region typically maps to a geographic region, for example `us`,
  with potentially multiple zones, which map to [datacenters](#datacenter) such
  as `us-west` and `us-east`.

- `server` <code>([Server][server]: nil)</code> - Specifies configuration which is specific to the Nomad server.

- `syslog_facility` `(string: "LOCAL0")` - Specifies the syslog facility to write to. This has no effect unless `enable_syslog` is true.

- `tls` <code>([TLS][tls]: nil)</code> - Specifies configuration for TLS.

- `vault` <code>([Vault][vault]: nil)</code> - Specifies configuration for
  connecting to Vault.

## Examples

### Custom Region and Datacenter

This example shows configuring a custom region and data center for the Nomad
agent:

```hcl
region     = "europe"
datacenter = "ams"
```

### Enable CORS

This example shows how to enable CORS on the HTTP API endpoints:

```hcl
http_api_response_headers {
  "Access-Control-Allow-Origin" = "*"
}
```

[hcl]: https://github.com/hashicorp/hcl "HashiCorp Configuration Language"
[go-sockaddr/template]: https://godoc.org/github.com/hashicorp/go-sockaddr/template
[consul]: /docs/agent/configuration/consul.html "Nomad Agent consul Configuration"
[vault]: /docs/agent/configuration/vault.html "Nomad Agent vault Configuration"
[tls]: /docs/agent/configuration/tls.html "Nomad Agent tls Configuration"
[client]: /docs/agent/configuration/client.html "Nomad Agent client Configuration"
[server]: /docs/agent/configuration/server.html "Nomad Agent server Configuration"
