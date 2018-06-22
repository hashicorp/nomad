---
layout: "docs"
page_title: "consul Stanza - Agent Configuration"
sidebar_current: "docs-configuration-consul"
description: |-
  The "consul" stanza configures the Nomad agent's communication with
  Consul for service discovery and key-value integration. When
  configured, tasks can register themselves with Consul, and the Nomad cluster
  can automatically bootstrap itself.
---

# `consul` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>**consul**</code>
    </td>
  </tr>
</table>


The `consul` stanza configures the Nomad agent's communication with
[Consul][consul] for service discovery and key-value integration. When
configured, tasks can register themselves with Consul, and the Nomad cluster can
[automatically bootstrap][bootstrap] itself.

```hcl
consul {
  address = "127.0.0.1:8500"
  auth    = "admin:password"
  token   = "abcd1234"
}
```

A default `consul` stanza is automatically merged with all Nomad agent
configurations. These sane defaults automatically enable Consul integration if
Consul is detected on the system. This allows for seamless bootstrapping of the
cluster with zero configuration. To put it another way: if you have a Consul
agent running on the same host as the Nomad agent with the default
configuration, Nomad will automatically connect and configure with Consul.

An important requirement is that each Nomad agent talks to a unique Consul
agent. Nomad agents should be configured to talk to Consul agents and not
Consul servers. If you are observing flapping services, you may have have
multiple Nomad agents talking to the same Consul agent. As such avoid
configuring Nomad to talk to Consul via DNS such as consul.service.consul

## `consul` Parameters

- `address` `(string: "127.0.0.1:8500")` - Specifies the address to the local
  Consul agent, given in the format `host:port`. Supports Unix sockets with the
  format: `unix:///tmp/consul/consul.sock`

- `auth` `(string: "")` - Specifies the HTTP Basic Authentication information to
  use for access to the Consul Agent, given in the format `username:password`.

- `auto_advertise` `(bool: true)` - Specifies if Nomad should advertise its
  services in Consul. The services are named according to `server_service_name`
  and `client_service_name`. Nomad servers and clients advertise their
  respective services, each tagged appropriately with either `http` or `rpc`
  tag. Nomad servers also advertise a `serf` tagged service.

- `ca_file` `(string: "")` - Specifies an optional path to the CA certificate
  used for Consul communication. This defaults to the system bundle if
  unspecified.

- `cert_file` `(string: "")` - Specifies the path to the certificate used for
  Consul communication. If this is set then you need to also set `key_file`.

- `checks_use_advertise` `(bool: false)` - Specifies if Consul health checks
  should bind to the advertise address. By default, this is the bind address.

- `client_auto_join` `(bool: true)` - Specifies if the Nomad clients should
  automatically discover servers in the same region by searching for the Consul
  service name defined in the `server_service_name` option. The search occurs if
  the client is not registered with any servers or it is unable to heartbeat to
  the leader of the region, in which case it may be partitioned and searches for
  other servers.

- `client_service_name` `(string: "nomad-client")` - Specifies the name of the
  service in Consul for the Nomad clients.

- `client_http_check_name` `(string: "Nomad Client HTTP Check")` - Specifies the
  HTTP health check name in Consul for the Nomad clients.

- `key_file` `(string: "")` - Specifies the path to the private key used for
  Consul communication. If this is set then you need to also set `cert_file`.

- `server_service_name` `(string: "nomad")` - Specifies the name of the service
  in Consul for the Nomad servers.

- `server_http_check_name` `(string: "Nomad Server HTTP Check")` - Specifies the
  HTTP health check name in Consul for the Nomad servers.

-  `server_serf_check_name` `(string: "Nomad Server Serf Check")` - Specifies
  the Serf health check name in Consul for the Nomad servers.

-  `server_rpc_check_name` `(string: "Nomad Server RPC Check")` - Specifies
  the RPC health check name in Consul for the Nomad servers.

- `server_auto_join` `(bool: true)` - Specifies if the Nomad servers should
  automatically discover and join other Nomad servers by searching for the
  Consul service name defined in the `server_service_name` option. This search
  only happens if the server does not have a leader.

- `ssl` `(bool: false)` - Specifies if the transport scheme should use HTTPS to
  communicate with the Consul agent.

- `token` `(string: "")` - Specifies the token used to provide a per-request ACL
  token. This option overrides the Consul Agent's default token. If the token is 
  not set here or on the Consul agent, it will default to Consul's anonymous policy, 
  which may or may not allow writes.

- `verify_ssl` `(bool: true)`- Specifies if SSL peer verification should be used
  when communicating to the Consul API client over HTTPS


If the local Consul agent is configured and accessible by the Nomad agents, the
Nomad cluster will [automatically bootstrap][bootstrap] provided
`server_auto_join`, `client_auto_join`, and `auto_advertise` are all enabled
(which is the default).

## `consul` Examples

### Default

This example shows the default Consul integration:

```hcl
consul {
  address             = "127.0.0.1:8500"
  server_service_name = "nomad"
  client_service_name = "nomad-client"
  auto_advertise      = true
  server_auto_join    = true
  client_auto_join    = true
}
```

### Custom Address and Port

This example shows pointing the Nomad agent at a different Consul address. Note
that you should **never** point directly at a Consul server; always point to a
local client. In this example, the Consul server is bound and listening on the
node's private IP address instead of localhost, so we use that:

```hcl
consul {
  address = "10.0.2.4:8500"
}
```

### Custom SSL

This example shows configuring custom SSL certificates to communicate with
the Consul agent. The Consul agent should be configured to accept certificates
similarly, but that is not discussed here:

```hcl
consul {
  ssl       = true
  ca_file   = "/var/ssl/bundle/ca.bundle"
  cert_file = "/etc/ssl/consul.crt"
  key_file  = "/etc/ssl/consul.key"
}
```

[consul]: https://www.consul.io/ "Consul by HashiCorp"
[bootstrap]: /guides/operations/cluster/automatic.html "Automatic Bootstrapping"
