---
layout: "docs"
page_title: "Frequently Asked Questions"
sidebar_current: "docs-faq"
description: |-
    Frequently asked questions and answers for Nomad
---

# Frequently Asked Questions

## Q: What is Checkpoint? / Does Nomad call home?

Nomad makes use of a HashiCorp service called [Checkpoint](https://checkpoint.hashicorp.com)
which is used to check for updates and critical security bulletins.
Only anonymous information, which cannot be used to identify the user or host, is
sent to Checkpoint. An anonymous ID is sent which helps de-duplicate warning messages.
This anonymous ID can be disabled. Using the Checkpoint service is optional and can be disabled.

See [`disable_anonymous_signature`](/docs/configuration/index.html#disable_anonymous_signature)
and [`disable_update_check`](/docs/configuration/index.html#disable_update_check).

## Q: Is Nomad eventually or strongly consistent?

Nomad makes use of both a [consensus protocol](/docs/internals/consensus.html) and
a [gossip protocol](/docs/internals/gossip.html). The consensus protocol is strongly
consistent, and is used for all state replication and scheduling. The gossip protocol
is used to manage the addresses of servers for automatic clustering and multi-region
federation. This means all data that is managed by Nomad is strongly consistent.

## Q: Is Nomad's `datacenter` parameter the same as Consul's?

No. For those familiar with Consul, [Consul's notion of a
datacenter][consul_dc] is more equivalent to a [Nomad region][nomad_region].
Nomad supports grouping nodes into multiple datacenters, which should reflect
nodes being colocated, while being managed by a single set of Nomad servers.

Consul on the other hand does not have this two-tier approach to servers and
agents and instead [relies on federation to create larger logical
clusters][consul_fed].

[consul_dc]: https://www.consul.io/docs/agent/options.html#_datacenter
[consul_fed]: https://www.consul.io/docs/guides/datacenters.html
[nomad_region]: /docs/configuration/index.html#datacenter
