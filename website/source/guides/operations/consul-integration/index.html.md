---
layout: "docs"
page_title: "Service Discovery"
sidebar_current: "docs-service-discovery"
description: |-
  Learn how to add service discovery to jobs
---

# Service Discovery

Nomad schedules workloads of various types across a cluster of generic hosts.
Because of this, placement is not known in advance and you will need to use
service discovery to connect tasks to other services deployed across your
cluster. Nomad integrates with [Consul][] to provide service discovery and
monitoring.

Note that in order to use Consul with Nomad, you will need to configure and
install Consul on your nodes alongside Nomad, or schedule it as a system job.
Nomad does not currently run Consul for you.

## Configuration

To enable Consul integration, please see the
[Nomad agent Consul integration](/docs/agent/configuration/consul.html)
configuration.


## Service Definition Syntax

To configure a job to register with service discovery, please see the
[`service` job specification documentation][service].

## Assumptions

- Consul 0.7.2 or later is needed for `tls_skip_verify` in HTTP checks.

- Consul 0.6.4 or later is needed for using the Script checks.

- Consul 0.6.0 or later is needed for using the TCP checks.

- The service discovery feature in Nomad depends on operators making sure that
  the Nomad client can reach the Consul agent.

- Tasks running inside Nomad also need to reach out to the Consul agent if
  they want to use any of the Consul APIs. Ex: A task running inside a docker
  container in the bridge mode won't be able to talk to a Consul Agent running
  on the loopback interface of the host since the container in the bridge mode
  has its own network interface and doesn't see interfaces on the global
  network namespace of the host. There are a couple of ways to solve this, one
  way is to run the container in the host networking mode, or make the Consul
  agent listen on an interface in the network namespace of the container.

[consul]: https://www.consul.io/ "Consul by HashiCorp"
[service]: /docs/job-specification/service.html "Nomad service Job Specification"
