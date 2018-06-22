---
layout: "guides"
page_title: "Consul Integration"
sidebar_current: "guides-operations-consul-integration"
description: |-
  Learn how to integrate Nomad with Consul and add service discovery to jobs
---

# Consul Integration

[Consul][] is a tool for discovering and configuring services in your 
infrastructure. Consul's key features include service discover, health checking, 
a KV store, and robust support for multi-datacenter deployments. Nomad's integration 
with Consul enables automatic clustering, built-in service registration, and 
dynamic rendering of configuration files and environment variables. The sections 
below describe the integration in more detail.

## Configuration

In order to use Consul with Nomad, you will need to configure and
install Consul on your nodes alongside Nomad, or schedule it as a system job.
Nomad does not currently run Consul for you.

To enable Consul integration, please see the
[Nomad agent Consul integration](/docs/configuration/consul.html)
configuration.

## Automatic Clustering with Consul

Nomad servers and clients will be automatically informed of each other's 
existence when a running Consul cluster already exists and the Consul agent is 
installed and configured on each host. Please see the [Automatic Clustering with 
Consul](/guides/operations/cluster/automatic.html) guide for more information.

## Service Discovery

Nomad schedules workloads of various types across a cluster of generic hosts.
Because of this, placement is not known in advance and you will need to use
service discovery to connect tasks to other services deployed across your
cluster. Nomad integrates with Consul to provide service discovery and
monitoring.

To configure a job to register with service discovery, please see the
[`service` job specification documentation][service].

## Dynamic Configuration

Nomad's job specification includes a [`template` stanza](/docs/job-specification/template.html) 
that utilizes a Consul ecosystem tool called [Consul Template](https://github.com/hashicorp/consul-template). This mechanism creates a convenient way to ship configuration files 
that are populated from environment variables, Consul data, Vault secrets, or just 
general configurations within a Nomad task.

For more information on Nomad's template stanza and how it leverages Consul Template, 
please see the [`template` job specification documentation](/docs/job-specification/template.html).

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
