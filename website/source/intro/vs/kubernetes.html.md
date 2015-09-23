---
layout: "intro"
page_title: "Nomad vs. Kubernetes"
sidebar_current: "vs-other-kubernetes"
description: |-
  Comparison between Nomad and Kubernetes
---

# Nomad vs. Kubernetes

Kubernetes is an orchestration system for Docker developed by the Cloud Native
Computing Foundation (CNCF). Kubernetes aims to provide all the features
needed to run Docker based applications including cluster management,
scheduling, service discovery, monitoring, secrets management and more.

Nomad only aims to provide cluster management and scheduling and is designed
with the Unix philosophy of having a small scope while composing with tools like [Consul](https://consul.io)
for service discovery and [Vault](https://www.vaultproject.io) for secret management.

While Kubernetes is specifically focused on Docker, Nomad is more general purpose.
Nomad supports virtualized, containerized and standalone applications, including Docker.
Nomad is designed with extensible drivers and support will be extended to all
common drivers.

Kubernetes is designed as a collection of more than a half-dozen interoperating
services which together provide the full functionality. Coordination and
storage is provided by etcd at the core. The state is wrapped by API controllers
which are consumed by other services that provide higher level APIs or features
like scheduling. Kubernetes supports running in a high available
configuration but is operationally complex to setup.

Nomad is architecturally much simpler. Nomad is a single binary, both for clients
and servers, and requires no external services for coordination or storage.
Nomad combines a lightweight resource managers and a sophisticated scheduler
into a single system. By default, Nomad is distributed, highly available,
and operationally simple.

At the time of writing, Kubernetes targets managing 100 node clusters and supports
only a single region. Nomad is designed to support clusters several orders of magnitude
larger and supports multi-datacenter and multi-region configurations.

