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
command-line interface. See the [CLI Options](#cli) section for further details.

Nomad's configuration is broken down into logical groupings. Because of the high
number of configuration options available, this page is also broken into
sections for easier reading.

## General Options

The following configuration options are available to both client and server
nodes, unless otherwise specified:

* <a id="region">`region`</a>: Specifies the region the Nomad agent is a
  member of. A region typically maps to a geographic region, for example `us`,
  with potentially multiple zones, which map to [datacenters](#datacenter) such
  as `us-west` and `us-east`. Defaults to `global`.

* `datacenter`: Datacenter of the local agent. All members of a datacenter
  should all share a local LAN connection. Defaults to `dc1`.

* <a id="name">`name`</a>: The name of the local node. This value is used to
  identify individual nodes in a given datacenter and must be unique
  per-datacenter. By default this is set to the local host's name.

* `data_dir`: A local directory used to store agent state. Client nodes use this
  directory by default to store temporary allocation data as well as cluster
  information. Server nodes use this directory to store cluster state, including
  the replicated log and snapshot data. This option is required to start the
  Nomad agent.

* `log_level`: Controls the verbosity of logs the Nomad agent will output. Valid
  log levels include `WARN`, `INFO`, or `DEBUG` in increasing order of
  verbosity. Defaults to `INFO`.

* <a id="bind_addr">`bind_addr`</a>: Used to indicate which address the Nomad
  agent should bind to for network services, including the HTTP interface as
  well as the internal gossip protocol and RPC mechanism. This should be
  specified in IP format, and can be used to easily bind all network services to
  the same address. It is also possible to bind the individual services to
  different addresses using the [addresses](#addresses) configuration option.
  Defaults to the local loopback address `127.0.0.1`.

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

* <a id="addresses">`addresses`</a>: Controls the bind address for individual
  network services. Any values configured in this block take precedence over the
  default [bind_addr](#bind_addr). The value is a map of IP addresses and
  supports the following keys:
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

* `advertise`: Controls the advertise address for individual network services.
  This can be used to advertise a different address to the peers of a server
  node to support more complex network configurations such as NAT. This
  configuration is optional, and defaults to the bind address of the specific
  network service if it is not provided. This configuration is only appicable
  on server nodes. The value is a map of IP addresses and supports the
  following keys:
  <br>
  * `rpc`: The address to advertise for the RPC interface. This address should
    be reachable by all of the agents in the cluster.
  * `serf`: The address advertised for the gossip layer. This address must be
    reachable from all server nodes. It is not required that clients can reach
    this address.

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

* `leave_on_interrupt`: Enables gracefully leave when receiving the
  interrupt signal. By default, the agent will exit forcefully on any signal.

* `leave_on_terminate`: Enables gracefully leave when receiving the
  terminate signal. By default, the agent will exit forcefully on any signal.

* `enable_syslog`: Enables logging to syslog. This option only work on
  Unix based systems.

* `syslog_facility`: Controls the syslog facility that is used. By default,
  `LOCAL0` will be used. This should be used with `enable_syslog`.

* `disable_update_check`: Disables automatic checking for security bulletins
  and new version releases.

* `disable_anonymous_signature`: Disables providing an anonymous signature
  for de-duplication with the update check. See `disable_update_check`.

## Server-specific Options

The following options are applicable to server agents only and need not be
configured on client nodes.

* `server`: This is the top-level key used to define the Nomad server
  configuration. It is a key/value mapping which supports the following keys:
  <br>
  * `enabled`: A boolean indicating if server mode should be enabled for the
    local agent. All other server options depend on this value being set.
    Defaults to `false`.
  * <a id="bootstrap_expect">`bootstrap_expect`</a>: This is an integer
    representing the number of server nodes to wait for before bootstrapping. It
    is most common to use the odd-numbered integers `3` or `5` for this value,
    depending on the cluster size. A value of `1` does not provide any fault
    tolerance and is not recommended for production use cases.
  * `data_dir`: This is the data directory used for server-specific data,
    including the replicated log. By default, this directory lives inside of the
    [data_dir](#data_dir) in the "server" sub-path.
  * `protocol_version`: The Nomad protocol version spoken when communicating
    with other Nomad servers. This value is typically not required as the agent
    internally knows the latest version, but may be useful in some upgrade
    scenarios.
  * `num_schedulers`: The number of parallel scheduler threads to run. This
    can be as many as one per core, or `0` to disallow this server from making
    any scheduling decisions. This defaults to the number of CPU cores.
  * `enabled_schedulers`: This is an array of strings indicating which
    sub-schedulers this server will handle. This can be used to restrict the
    evaluations that worker threads will dequeue for processing. This
    defaults to all available schedulers.

## Client-specific Options

The following options are applicable to client agents only and need not be
configured on server nodes.

* `client`: This is the top-level key used to define the Nomad client
  configuration. Like the server configuration, it is a key/value mapping which
  supports the following keys:
  <br>
  * `enabled`: A boolean indicating if client mode is enabled. All other client
    configuration options depend on this value. Defaults to `false`.
  * <a id="state_dir">`state_dir`</a>: This is the state dir used to store
    client state. By default, it lives inside of the [data_dir](#data_dir), in
    the "client" sub-path.
  * <a id="alloc_dir">`alloc_dir`</a>: A directory used to store allocation data.
    Depending on the workload, the size of this directory can grow arbitrarily
    large as it is used to store downloaded artifacts for drivers (QEMU images,
    JAR files, etc.). It is therefore important to ensure this directory is
    placed some place on the filesystem with adequate storage capacity. By
    default, this directory lives under the [data_dir](#data_dir) at the
    "alloc" sub-path.
  * <a id="servers">`servers`</a>: An array of server addresses. This list is
    used to register the client with the server nodes and advertise the
    available resources so that the agent can receive work.
  * <a id="node_id">`node_id`</a>: This is the value used to uniquely identify
    the local agent's node registration with the servers. This can be any
    arbitrary string but must be unique to the cluster. By default, if not
    specified, a randomly- generate UUID will be used.
  * <a id="node_class">`node_class`</a>: A string used to logically group client
    nodes by class. This can be used during job placement as a filter. This
    option is not required and has no default.
  * <a id="meta">`meta`</a>: This is a key/value mapping of metadata pairs. This
    is a free-form map and can contain any string values.
  * `network_interface`: This is a string to force network fingerprinting to use
    a specific network interface
  * `options`: This is a key/value mapping of internal configuration for clients,
    such as for driver configuration.

## Atlas Options

The following options are used to configure [Atlas](https://atlas.hashicorp.com)
integration and are entirely optional.

* `atlas`: The top-level config key used to contain all Atlas-related
  configuration options. The value is a key/value map which supports the
  following keys:
  <br>
  * <a id="infrastructure">`infrastructure`</a>: The Atlas infrastructure name to
    connect this agent to. This value should be of the form
    `<org>/<infrastructure>`, and requires a valid [token](#token) authorized on
    the infrastructure.
  * <a id="token">`token`</a>: The Atlas token to use for authentication. This
    token should have access to the provided [infrastructure](#infrastructure).
  * <a id="join">`join`</a>: A boolean indicating if the auto-join feature of
    Atlas should be enabled. Defaults to `false`.
  * `endpoint`: The address of the Atlas instance to connect to. Defaults to the
    public Atlas endpoint and is only used if both
    [infrastructure](#infrastructure) and [token](#token) are provided.

## Command-line Options <a id="cli"></a>

A subset of the available Nomad agent configuration can optionally be passed in
via CLI arguments. The `agent` command accepts the following arguments:

* `alloc-dir=<path>`: Equivalent to the Client [alloc_dir](#alloc_dir) config
   option.
* `-atlas=<infrastructure>`: Equivalent to the Atlas
  [infrastructure](#infrastructure) config option.
* `-atlas-join`: Equivalent to the Atlas [join](#join) config option.
* `-atlas-token=<token>`: Equivalent to the Atlas [token](#token) config option.
* `-bind=<address>`: Equivalent to the [bind_addr](#bind_addr) config option.
* `-bootstrap-expect=<num>`: Equivalent to the
  [bootstrap_expect](#bootstrap_expect) config option.
* `-client`: Enable client mode on the local agent.
* `-config=<path>`: Specifies the path to a configuration file or a directory of
  configuration files to load. Can be specified multiple times.
* `-data-dir=<path>`: Equivalent to the [data_dir](#data_dir) config option.
* `-dc=<datacenter>`: Equivalent to the [datacenter](#datacenter) config option.
* `-dev`: Start the agent in development mode. This enables a pre-configured
  dual-role agent (client + server) which is useful for developing or testing
  Nomad. No other configuration is required to start the agent in this mode.
* `-log-level=<level>`: Equivalent to the [log_level](#log_level) config option.
* `-meta=<key=value>`: Equivalent to the Client [meta](#meta) config option.
* `-node=<name>`: Equivalent to the [name](#name) config option.
* `-node-class=<class>`: Equivalent to the Client [node_class](#node_class)
  config option.
* `-node-id=<uuid>`: Equivalent to the Client [node_id](#node_id) config option.
* `-region=<region>`: Equivalent to the [region](#region) config option.
* `-server`: Enable server mode on the local agent.
* `-servers=<host:port>`: Equivalent to the Client [servers](#servers) config
  option.
* `-state-dir=<path>`: Equivalent to the Client [state_dir](#state_dir) config
  option.
