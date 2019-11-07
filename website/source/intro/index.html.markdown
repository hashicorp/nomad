---
layout: "intro"
page_title: "Introduction"
sidebar_current: "what"
description: |-
  Welcome to the intro guide to Nomad! This guide is the best place to start with Nomad. We cover what Nomad is, what problems it can solve, how it compares to existing software, and a quick start for using Nomad.
---

# Introduction to Nomad

Welcome to the intro guide to Nomad! This guide is the best place to start with Nomad. We cover what Nomad is, what problems it can solve, how it compares to existing software, and how you can get started using it. If you are familiar with the basics of Nomad, the [documentation](https://www.nomadproject.io/docs/index.html) and [guides](https://www.nomadproject.io/guides/index.html) provides a more detailed reference of available features.

## What is Nomad?

Nomad is a flexible workload orchestrator that enables an organization to easily deploy and manage any containerized or legacy application using a single, unified workflow. Nomad can run a diverse workload of Docker, non-containerized, microservice, and batch applications.

Nomad enables developers to use declarative infrastructure-as-code for deploying applications.  Nomad uses bin packing to efficiently schedule jobs and optimize for resource utilization.  Nomad is supported on macOS, Windows, and Linux.

Nomad is widely adopted and used in production by PagerDuty, Target, Citadel, Trivago, SAP, Pandora, Roblox, eBay, Deluxe Entertainment, and more.   

## Key Features

* **Deploy Containers and Legacy Applications**: Nomad’s flexibility as an orchestrator enables an organization to run containers, legacy, and batch applications together on the same infrastructure.  Nomad brings core orchestration benefits to legacy applications without needing to containerize via pluggable [task drivers](https://www.nomadproject.io/docs/drivers/index.html).

* **Simple & Reliable**:  Nomad runs as a single 75MB binary and is entirely self contained - combining resource management and scheduling into a single system.  Nomad does not require any external services for storage or coordination.  Nomad automatically handles application, node, and driver failures.  Nomad is distributed and resilient, using leader election and state replication to provide high availability in the event of failures.   

* **Device Plugins & GPU Support**: Nomad offers built-in support for GPU workloads such as machine learning (ML) and artificial intelligence (AI).  Nomad uses [device plugins](https://www.nomadproject.io/docs/devices/index.html) to automatically detect and utilize resources from hardware devices such as GPU, FPGAs, and TPUs.

* **Federation for Multi-Region**: Nomad has native support for multi-region federation. This built-in capability allows multiple clusters to be linked together, which in turn enables developers to deploy jobs to any cluster in any region. Federation also enables automatic replication of ACL policies, namespaces, resource quotas and Sentinel policies across all clusters.

* **Proven Scalability**: Nomad is optimistically concurrent, which increases throughput and reduces latency for workloads.  Nomad has been proven to scale to clusters of 10K+ nodes in real-world production environments.

* **HashiCorp Ecosystem**: Nomad integrates seamlessly with Terraform, Consul, Vault for provisioning, service discovery, and secrets management.

## How Nomad Compares to Other Tools

Nomad differentiates from related tools by virtue of its **simplicity**, **flexibility**,
**scalability**, and **high performance**. Nomad's synergy and integration points with
HashiCorp Terraform, Consul, and Vault make it uniquely suited for easy integration into
an organization's existing workflows, minimizing the time-to-market for critical initiatives.

See the [Nomad vs. Other Software](/intro/vs/index.html) page for additional details and
comparisons.

## Next Steps

See the [Use Cases](/intro/use-cases.html) and [Who Uses Nomad](/intro/who-uses-nomad.html) page to understand the many ways Nomad is used in production today across many industries to solve critical, real-world business objectives.
