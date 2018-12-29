---
layout: "guides"
page_title: "Manually Bootstrapping a Nomad Cluster"
sidebar_current: "guides-cluster-manual"
description: |-
  Learn how to manually bootstrap a Nomad cluster using the server-join
  command. This section also discusses Nomad federation across multiple
  datacenters and regions.
---

# Manual Bootstrapping

Manually bootstrapping a Nomad cluster does not rely on additional tooling, but
does require operator participation in the cluster formation process. When
bootstrapping, Nomad servers and clients must be started and informed with the
address of at least one Nomad server.

As you can tell, this creates a chicken-and-egg problem where one server must
first be fully bootstrapped and configured before the remaining servers and
clients can join the cluster. This requirement can add additional provisioning
time as well as ordered dependencies during provisioning.

First, we bootstrap a single Nomad server and capture its IP address. After we
have that nodes IP address, we place this address in the configuration.

For Nomad servers, this configuration may look something like this:

```hcl
server {
  enabled          = true
  bootstrap_expect = 3

  # This is the IP address of the first server we provisioned
  retry_join = ["<known-address>:4648"]
}
```

Alternatively, the address can be supplied after the servers have all been
started by running the [`server-join` command](/docs/commands/server-join.html)
on the servers individually to cluster the servers. All servers can join just
one other server, and then rely on the gossip protocol to discover the rest.

```
$ nomad server-join <known-address>
```

For Nomad clients, the configuration may look something like:

```hcl
client {
  enabled = true
  servers = ["<known-address>:4647"]
}
```

At this time, there is no equivalent of the <tt>server-join</tt> command for
Nomad clients.

The port corresponds to the RPC port. If no port is specified with the IP
address, the default RPC port of `4647` is assumed.

As servers are added or removed from the cluster, this information is pushed to
the client. This means only one server must be specified because, after initial
contact, the full set of servers in the client's region are shared with the
client.
