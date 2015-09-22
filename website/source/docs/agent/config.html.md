---
layout: "docs"
page_title: "Configuration"
sidebar_current: "docs-agent-config"
description: |-
  Learn about the configuration options available for the Nomad agent.
---

# Configuration

Nomad agents are highly configurable and expose many configuration options
through the use of config files. Config files are written in
[HCL](https://github.com/hashicorp/hcl) or JSON syntax. Multiple configuration
files or directories of configuration files may be used jointly to configure the
Nomad agent.

When loading configuration files and directories, the Nomad agent parses each
file in lexical order. As each file is processed, its contents are merged into
the existing configuration, enabling a layered, additive configuration
mechanism. During a merge, configuration values are copied from
the next configuration file in the set if they have a non-empty value. An
empty value means `""` for strings, `0` for integer or float values, and
`false` for booleans. More complex data types like arrays or maps are usually
appended together. Any exceptions to these rules are documented alongside the
configuration options below.

A subset of the configuration options can also be specified using the
command-line interface. See the [CLI Options](#) section for further details.

Nomad's configuration is broken down into logical groupings. Because of the high
number of configuration options available, this page is also broken into
sections for easier reading.

## General Options

The following configuration options are available to both client and server
nodes, unless otherwise specified:

* `region`: Specifies the region the Nomad agent is a member of. A region
  typically maps to a geographic region, for example `us-east`, with potentially
  multiple zones, which map to [datacenters](#datacenter). Defaults to
  `global`.

* `datacenter`: Datacenter name the local agent is a member of. Members within a
  single datacenter should all share a local LAN connection. Defaults to `dc1`.

* `node`: The name of the local node. This value is used to identify individual
  nodes in a given datacenter and must be unique per-datacenter. By default this
  is set to the local host's name.

* `data_dir`: A local directory used to store agent state. Client nodes use this
  directory by default to store temporary allocation data as well as cluster
  information. Server nodes use this directory to store cluster state, including
  the replicated log and snapshot data. This option is required to start the
  Nomad agent.

* `log_level`: Controls the verbosity of logs the Nomad agent will output. Valid
  log levels include `WARN`, `INFO`, or `DEBUG` in increasing order of
  verbosity. Defaults to `INFO`.

* `bind_addr`: Used to indicate which address the Nomad agent should bind to for
  network services, including the HTTP interface as well as the internal gossip
  protocol and RPC mechanism. This should be specified in IP format, and can be
  used to easily bind all network services to the same address. It is also
  possible to bind the individual services to different addresses using the
  [addresses](#addresses) configuration option. Defaults to the local loopback
  address `127.0.0.1`.

* `enable_debug`: Enables the debugging HTTP endpoints. These endpoints can be
  used with profiling tools to dump diagnostic information about Nomad's
  internals. It is not recommended to leave this enabled in production
  environments. Defaults to `false`.

* `ports`: Controls the network ports used for different services required by
  the Nomad agent. The value is a key/value mapping of port numbers, and accepts
  the following keys:
  <br>
  * `http`: The port used to run the HTTP server. Applies to both client and
    server nodes. Defaults to `4646`.
  * `rpc`: The port used for internal RPC communication between agents and
    servers, and for inter-server traffic for the consensus algorithm (raft).
    Defaults to `4647`. Only used on server nodes.
  * `serf`: The port used for the gossip protocol for cluster membership. Both
    TCP and UDP should be routable between the server nodes on this port.
    Defaults to `4648`. Only used on server nodes.

* `addresses`: Controls the bind address for individual network services. Any
  values configured in this block take precedence over the default
  [bind_addr](#bind_addr). The value is a map of IP addresses and supports the
  following keys:
  <br>
  * `http`: The address the HTTP server is bound to. This is the most common
    bind address to change. Applies to both clients and servers.
  * `rpc`: The address to bind the internal RPC interfaces to. Should be exposed
    only to other cluster members if possible. Used only on server nodes, but
    must be accessible from all agents.
  * `serf`: The address used to bind the gossip layer to. Both a TCP and UDP
    listener will be exposed on this address. Should be restricted to only
    server nodes from the same datacenter if possible. Used only on server
    nodes.

* `telemetry`: Used to control how the Nomad agent exposes telemetry data to
  external metrics collection servers. This is a key/value mapping and supports
  the following keys:
  <br>
  * `statsite_address`: Address of a
    [statsite](https://github.com/armon/statsite) server to forward metrics data
    to.
  * `statsd_address`: Address of a [statsd](https://github.com/etsy/statsd)
    server to forward metrics to.
  * `disable_hostname`: A boolean indicating if gauge values should not be
    prefixed with the local hostname.
