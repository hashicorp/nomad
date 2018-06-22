---
layout: "intro"
page_title: "Introduction"
sidebar_current: "what"
description: |-
  Welcome to the intro guide to Nomad! This guide is the best place to start with Nomad. We cover what Nomad is, what problems it can solve, how it compares to existing software, and a quick start for using Nomad.
---

# Introduction to Nomad

Welcome to the intro guide to Nomad! This guide is the best
place to start with Nomad. We cover what Nomad is, what
problems it can solve, how it compares to existing software,
and contains a quick start for using Nomad.

If you are already familiar with the basics of Nomad, the [Guides](/guides/index.html) 
and the [reference documentation](/docs/index.html) will provide a more comprehensive 
resource.

## What is Nomad?

Nomad is a flexible container orchestration tool that enables an organization to 
easily deploy and manage any containerized or legacy application using a single, 
unified workflow. Nomad can run a diverse workload of Docker, non-containerized, 
microservice, and batch applications, and generally offers the following benefits 
to developers and operators:

* **API-driven Automation**: Workload placement, scaling, and upgrades can be 
  automated, simplifying operations and eliminating the need for homegrown tooling.
* **Self-service Deployments**: Developers are empowered to service application 
  lifecycles directly, allowing operators to focus on higher value tasks.
* **Workload Reliability**: Application, node, and driver failures are handled 
  automatically, reducing the need for manual operator intervention
* **Increased Efficiency and Reduced Cost**: Higher application densities allow 
  operators to reduce fleet sizes and save money.

Nomad is trusted by enterprises from a range of sectors including financial, 
retail, software, and others to run production workloads at scale across private 
infrastructure and the public cloud.

## How it Works

At its core, Nomad is a tool for managing a cluster of machines and running applications
on them. Nomad abstracts away machines and the location of applications,
and instead enables users to declare what they want to run while Nomad handles
where and how to run them. 

The key features of Nomad are:

* **Docker Support**: Nomad supports Docker as a first-class workload type.
  Jobs submitted to Nomad can use the `docker` driver to easily deploy containerized
  applications to a cluster. Nomad enforces the user-specified constraints,
  ensuring the application only runs in the correct region, datacenter, and host
  environment. Jobs can specify the number of instances needed and
  Nomad will handle placement and recover from failures automatically.

* **Operationally Simple**: Nomad ships as a single binary, both for clients and servers,
  and requires no external services for coordination or storage. Nomad combines features
  of both resource managers and schedulers into a single system. Nomad builds on the strength
  of [Serf](https://www.serf.io) and [Consul](https://www.consul.io), distributed management
  tools by [HashiCorp](https://www.hashicorp.com).

* **Multi-Datacenter and Multi-Region Aware**: Nomad models infrastructure as
  groups of datacenters which form a larger region. Scheduling operates at the region
  level allowing for cross-datacenter scheduling. Multiple regions federate together
  allowing jobs to be registered globally.

* **Flexible Workloads**: Nomad has extensible support for task drivers, allowing it to run
  containerized, virtualized, and standalone applications. Users can easily start Docker
  containers, VMs, or application runtimes like Java. Nomad supports Linux, Windows, BSD and OSX,
  providing the flexibility to run any workload.

* **Built for Scale**: Nomad was designed from the ground up to support global scale
  infrastructure. Nomad is distributed and highly available, using both
  leader election and state replication to provide availability in the face
  of failures. Nomad is optimistically concurrent, enabling all servers to participate
  in scheduling decisions which increases the total throughput and reduces latency
  to support demanding workloads. Nomad has been proven to scale to cluster sizes that 
  exceed 10k nodes in real-world production environments.

## How Nomad Compares to Other Tools

Nomad differentiates from related tools by virtue of its **simplicity**, **flexibility**, 
**scalability**, and **high performance**. Nomad's synergy and integration points with 
HashiCorp Terrform, Consul, and Vault make it uniquely suited for easy integration into 
an organization's existing workflows, minimizing the time-to-market for critical initiatives. 
See the [Nomad vs. Other Software](/intro/vs/index.html) page for additional details and 
comparisons.

## Next Steps

See the page on [Nomad use cases](/intro/use-cases.html) to see the
multiple ways Nomad can be used. Then see
[how Nomad compares to other software](/intro/vs/index.html)
to see how it fits into your existing infrastructure. Finally, continue onwards with
the [getting started guide](/intro/getting-started/install.html) to use
Nomad to run a job and see how it works in practice.

