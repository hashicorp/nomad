---
layout: "docs"
page_title: "High Availability"
sidebar_current: "docs-concepts-ha"
description: |-
  Nomad can be highly available, allowing you to run multiple Nomads to protect against outages.
---

# High Availability Mode (HA)

Nomad supports multi-server mode for high availability. This mode protects
against outages by running multiple Nomad servers. High availability mode
is automatically enabled when using a storage backend that supports it.

You can tell if a backend supports high availability mode ("HA") by
starting the server and seeing if "(HA available)" is outputted next to
the backend information. If it is, then HA will begin happening automatically.

To be highly available, Nomad elects a leader and does request forwarding to
the leader. Due to this architecture, HA does not enable increased scalability.
In general, the bottleneck of Nomad is the storage backend itself, not
Nomad core. For example: to increase scalability of Nomad with Consul, you
would scale Consul instead of Nomad.

In addition to using a backend that supports HA, you have to configure
Nomad with an _advertise address_. This is the address that Nomad advertises
to other Nomad servers in the cluster for request forwarding. By default,
Nomad will use the first private IP address it finds, but you can override
this to any address you want.

## Backend Support

Currently there are several backends that support high availability mode,
including Consul, ZooKeeper and etcd. These may change over time, and the
[configuration page](/docs/config/index.html) should be referenced.

The Consul backend is the recommended HA backend, as it is used in production
by HashiCorp and it's customers with commercial support.

If you're interested in implementing another backend or adding HA support
to another backend, we'd love your contributions. Adding HA support
requires implementing the `physical.HABackend` interface for the storage backend.
