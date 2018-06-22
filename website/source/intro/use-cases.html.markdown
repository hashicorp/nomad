---
layout: "intro"
page_title: "Use Cases"
sidebar_current: "use-cases"
description: |-
  This page lists some concrete use cases for Nomad, but the possible use cases 
  are much broader than what we cover.
---

# Use Cases

This page lists Nomad's core use cases. Please note that the full range of potential 
use cases is much broader than what is currently covered here. Reading through the 
[Introduction to Nomad](/intro/index.html) is highly recommended before diving into 
the use cases. 

## Docker Container Management

Organizations are increasingly moving towards a Docker centric workflow for 
application deployment and management. This transition requires new tooling 
to automate placement, perform job updates, enable self-service for developers, 
and to handle failures automatically. Nomad supports a [first-class Docker workflow](/docs/drivers/docker.html) 
and integrates seamlessly with [Consul](/guides/operations/consul-integration/index.html) 
and [Vault](/guides/operations/vault-integration/index.html) to enable a complete solution 
while maximizing operational flexibility. Nomad is easy to use, can scale to 
thousands of nodes in a single cluster, and can easily deploy across private data 
centers and multiple clouds.

## Legacy Application Deployment

A virtual machine based application deployment strategy can lead to low hardware 
utlization rates and high infrastructure costs. While a Docker-based deployment 
strategy can be impractical for some organizations or use cases, the potential for 
greater automation, increased resilience, and reduced cost is very attractive. 
Nomad natively supports running legacy applications, static binaries, JARs, and 
simple OS commands directly. Workloads are natively isolated at runtime and bin 
packed to maximize efficiency and utilization (reducing cost). Developers and 
operators benefit from API-driven automation and enhanced reliability for 
applications through automatic failure handling.

## Microservices

Microservices and Service Oriented Architectures (SOA) are a design paradigm in 
which many services with narrow scope, tight state encapsulation, and API driven 
communication interact together to form a larger solution. However, managing hundreds 
or thousands of services instead of a few large applications creates an operational 
challenge. Nomad elegantly integrates with [Consul](/guides/operations/consul-integration/index.html) 
for automatic service registration and dynamic rendering of configuration files. Nomad 
and Consul together provide an ideal solution for managing microservices, making it 
easier to adopt the paradigm.

## Batch Processing Workloads

As data science and analytics teams grow is size and complexity, they increasingly 
benefit from highly performant and scalable tools that can run batch workloads with 
minimal operational overhead. Nomad can natively run batch jobs, [parameterized](https://www.hashicorp.com/blog/replacing-queues-with-nomad-dispatch) jobs, and [Spark](https://github.com/hashicorp/nomad-spark) 
workloads. Nomad's architecture enables easy scalability and an optimistically 
concurrent scheduling strategy that can yield [thousands of container deployments per 
second](https://www.hashicorp.com/c1m). Alternatives are overly complex and limited 
in terms of their scheduling throughput, scalability, and multi-cloud capabilities.

**Related video**: [End to End Production Nomad at Citadel](https://www.youtube.com/watch?reload=9&v=ZOBcGpGsboA)

## Multi-region and Multi-cloud Deployments

Nomad is designed to natively handle multi-datacenter and multi-region deployments 
and is cloud agnostic. This allows Nomad to schedule in private datacenters running 
bare metal, OpenStack, or VMware alongside an AWS, Azure, or GCE cloud deployment. 
This makes it easier to migrate workloads incrementally and to utilize the cloud 
for bursting.


