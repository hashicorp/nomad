---
layout: "docs"
page_title: "Configuration"
sidebar_current: "docs-agent-config"
description: |-
  Learn about the configuration options available for the Nomad agent.
---

# Configuration

Nomad agents have a variety of options that can be specified via configuration
files or command-line flags. Config files must be written in
[HCL](https://github.com/hashicorp/hcl) or JSON syntax. Nomad can read and
combine options from multiple configuration files or directories to configure
the Nomad agent.

## Loading Configuration Files

When specifying multiple config file options on the command-line, the files are
loaded in the order they are specified. For example:

    nomad agent -config server.conf /etc/nomad extra.json

Will load configuration from `server.conf`, from `.hcl` and `.json` files under
`/etc/nomad`, and finally from `extra.json`.

Configuration files in directories are loaded alphabetically. With the
directory option, only files ending with the `.hcl` or `.json` extensions are
used. Directories are not loaded recursively.

As each file is processed, its contents are merged into the existing
configuration. When merging, any non-empty values from the latest config file
will append or replace options in the current configuration. An empty value
means `""` for strings, `0` for integer or float values, and `false` for
booleans. Since empty values are ignored you cannot disable an option like
server mode once you've enabled it.

Complex data types like arrays or maps are usually merged. [Some configuration
options](#cli) can also be specified using the command-line interface. Please
refer to the sections below for the details of each option.

## Configuration Syntax

The preferred configuration syntax is HCL, which supports comments, but you can
also use JSON. Below is an example configuration file in HCL syntax.

```
bind_addr = "0.0.0.0"
data_dir = "/var/lib/nomad"

advertise {
  # We need to specify our host's IP because we can't
  # advertise 0.0.0.0 to other nodes in our cluster.
  rpc = "1.2.3.4:4647"
}

server {
  enabled = true
  bootstrap_expect = 3
}

client {
  enabled = true
  network_speed = 10
}

atlas {
  infrastructure = "hashicorp/mars"
  token = "atlas.v1.AFE84330943"
}
```

Note that it is strongly recommended _not_ to operate a node as both `client`
and `server`, although this is supported to simplify development and testing.

## General Options

The following configuration options are available to both client and server
nodes, unless otherwise specified:

* <a id="region">`region`</a>: Specifies the region the Nomad agent is a
  member of. A region typically maps to a geographic region, for example `us`,
  with potentially multiple zones, which map to [datacenters](#datacenter) such
  as `us-west` and `us-east`. Defaults to `global`.

* `datacenter`: Datacenter of the local agent. All members of a datacenter
  should share a local LAN connection. Defaults to `dc1`.

* <a id="name">`name`</a>: The name of the local node. This value is used to
  identify individual nodes in a given datacenter and must be unique
  per-datacenter. By default this is set to the local host's name.

* `data_dir`: A local directory used to store agent state. Client nodes use this
  directory by default to store temporary allocation data as well as cluster
  information. Server nodes use this directory to store cluster state, including
  the replicated log and snapshot data. This option is required to start the
  Nomad agent and must be specified as an absolute path.

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
  network service if it is not provided. This configuration is only applicable
  on server nodes. The value is a map of IP addresses and ports and supports
  the following keys:
  <br>
  * `rpc`: The address to advertise for the RPC interface. This address should
    be reachable by all of the agents in the cluster. For example:
    ```
    advertise {
      rpc = "1.2.3.4:4647"
    }
    ```
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

* `leave_on_interrupt`: Enables gracefully leaving when receiving the
  interrupt signal. By default, the agent will exit forcefully on any signal.

* `leave_on_terminate`: Enables gracefully leaving when receiving the
  terminate signal. By default, the agent will exit forcefully on any signal.

* `enable_syslog`: Enables logging to syslog. This option only works on
  Unix based systems.

* `syslog_facility`: Controls the syslog facility that is used. By default,
  `LOCAL0` will be used. This should be used with `enable_syslog`.

* `disable_update_check`: Disables automatic checking for security bulletins
  and new version releases.

* `disable_anonymous_signature`: Disables providing an anonymous signature
  for de-duplication with the update check. See `disable_update_check`.

* `http_api_response_headers`: This object allows adding headers to the 
  HTTP API responses. For example, the following config can be used to enable
  CORS on the HTTP API endpoints:
  ```
  http_api_response_headers {
      Access-Control-Allow-Origin = "*"
  }
  ```

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
  * `node_gc_threshold` This is a string with a unit suffix, such as "300ms",
    "1.5h" or "25m". Valid time units are "ns", "us" (or "µs"), "ms", "s",
    "m", "h". Controls how long a node must be in a terminal state before it is
    garbage collected and purged from the system.
  * <a id="rejoin_after_leave">`rejoin_after_leave`</a> When provided, Nomad will ignore a previous leave and
    attempt to rejoin the cluster when starting. By default, Nomad treats leave
    as a permanent intent and does not attempt to join the cluster again when
    starting. This flag allows the previous state to be used to rejoin the
    cluster.
  * <a id="retry_join">`retry_join`</a> Similar to [`start_join`](#start_join) but allows retrying a join
    if the first attempt fails. This is useful for cases where we know the
    address will become available eventually.
  * <a id="retry_interval">`retry_interval`</a> The time to wait between join attempts. Defaults to 30s.
  * <a id="retry_max">`retry_max`</a> The maximum number of join attempts to be made before exiting
    with a return code of 1. By default, this is set to 0 which is interpreted
    as infinite retries.
  * <a id="start_join">`start_join`</a> An array of strings specifying addresses of nodes to join upon startup.
    If Nomad is unable to join with any of the specified addresses, agent startup will
    fail. By default, the agent won't join any nodes when it starts up.

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
    the "client" sub-path. It must be specified as an absolute path.
  * <a id="alloc_dir">`alloc_dir`</a>: A directory used to store allocation data.
    Depending on the workload, the size of this directory can grow arbitrarily
    large as it is used to store downloaded artifacts for drivers (QEMU images,
    JAR files, etc.). It is therefore important to ensure this directory is
    placed some place on the filesystem with adequate storage capacity. By
    default, this directory lives under the [data_dir](#data_dir) at the
    "alloc" sub-path. It must be specified as an absolute path.
  * <a id="servers">`servers`</a>: An array of server addresses. This list is
    used to register the client with the server nodes and advertise the
    available resources so that the agent can receive work. If a port is not specified
    in the array of server addresses, the default port `4647` will be used.
  * <a id="node_id">`node_id`</a>: This is the value used to uniquely identify
    the local agent's node registration with the servers. This can be any
    arbitrary string but must be unique to the cluster. By default, if not
    specified, a randomly- generate UUID will be used.
  * <a id="node_class">`node_class`</a>: A string used to logically group client
    nodes by class. This can be used during job placement as a filter. This
    option is not required and has no default.
  * <a id="meta">`meta`</a>: This is a key/value mapping of metadata pairs. This
    is a free-form map and can contain any string values.
  * <a id="options">`options`</a>: This is a key/value mapping of internal
    configuration for clients, such as for driver configuration. Please see
    [here](#options_map) for a description of available options.
  * <a id="network_interface">`network_interface`</a>: This is a string to force
    network fingerprinting to use a specific network interface
  * <a id="network_speed">`network_speed`</a>: This is an int that sets the
    default link speed of network interfaces, in megabits, if their speed can
    not be determined dynamically.
  * `max_kill_timeout`: `max_kill_timeout` is a time duration that can be
    specified using the `s`, `m`, and `h` suffixes, such as `30s`. If a job's
    task specifies a `kill_timeout` greater than `max_kill_timeout`,
    `max_kill_timeout` is used. This is to prevent a user being able to set an
    unreasonable timeout. If unset, a default is used.

### Client Options Map <a id="options_map"></a>

The following is not an exhaustive list of options that can be passed to the
Client, but rather the set of options that configure the Client and not the
drivers. To find the options supported by an individual driver, see the drivers
documentation [here](/docs/drivers/index.html)

* `consul.address`: The address to the local Consul agent given in the format of
  `host:port`. The default is the same as the Consul default address,
  `127.0.0.1:8500`.

* `consul.token`: Token is used to provide a per-request ACL token.This options
  overrides the agent's default token

* `consul.auth`: The auth information to use for http access to the Consul
  Agent.

* `consul.ssl`: This boolean option sets the transport scheme to talk to the Consul
  Agent as `https`. This option is unset by default and so the default transport
  scheme for the consul api client is `http`.

* `consul.verifyssl`: This option enables SSL verification when the transport
 scheme for the Consul API client is `https`. This is set to true by default.

* `driver.whitelist`: A comma seperated list of whitelisted drivers (e.g.
  "docker,qemu"). If specified, drivers not in the whitelist will be disabled.
  If the whitelist is empty, all drivers are fingerprinted and enabled where
  applicable.

* `fingerprint.whitelist`: A comma seperated list of whitelisted fingerprinters.
  If specified, fingerprinters not in the whitelist will be disabled. If the
  whitelist is empty, all fingerprinters are used.

## Atlas Options

**NOTE**: Nomad integration with Atlas is awaiting release of Atlas features
for Nomad support.  Nomad currently only validates configuration options for
Atlas but does not use them.
See [#183](https://github.com/hashicorp/nomad/issues/183) for more details.

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
* `-join=<address>`: Address of another agent to join upon starting up. This can
  be specified multiple times to specify multiple agents to join.
* `-log-level=<level>`: Equivalent to the [log_level](#log_level) config option.
* `-meta=<key=value>`: Equivalent to the Client [meta](#meta) config option.
* `-network-interface<interface>`: Equivalent to the Client
   [network_interface](#network_interface) config option.
* `-network-speed<MBits>`: Equivalent to the Client
  [network_speed](#network_speed) config option.
* `-node=<name>`: Equivalent to the [name](#name) config option.
* `-node-class=<class>`: Equivalent to the Client [node_class](#node_class)
  config option.
* `-region=<region>`: Equivalent to the [region](#region) config option.
* `-rejoin`: Equivalent to the [rejoin_after_leave](#rejoin_after_leave) config option.
* `-retry-interval`: Equivalent to the [retry_interval](#retry_interval) config option.
* `-retry-join`: Similar to `-join` but allows retrying a join if the first attempt fails.
* `-retry-max`: Similar to the [retry_max](#retry_max) config option.
* `-server`: Enable server mode on the local agent.
* `-servers=<host:port>`: Equivalent to the Client [servers](#servers) config
  option.
* `-state-dir=<path>`: Equivalent to the Client [state_dir](#state_dir) config
  option.
