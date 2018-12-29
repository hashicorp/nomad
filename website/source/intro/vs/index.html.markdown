---
layout: "intro"
page_title: "Nomad vs. Other Software"
sidebar_current: "vs-other"
description: |-
  Comparisons between Nomad and other cluster managers.
---

# Nomad vs. Other Software

The following characteristics generally differentiate Nomad from related products:

* **Simplicity**: Nomad runs as a single process with zero external dependencies. 
  Operators can easily provision, manage, and scale Nomad. Developers can easily 
  define and run applications.
* **Flexibility**: Nomad can run a diverse workload of containerized, legacy, 
  microservice, and batch applications. Nomad can schedule service, batch 
  processing and system jobs, and can run on both Linux and Windows.
* **Scalability and High Performance**: Nomad can schedule thousands of containers 
  per second, scale to thousands of nodes in a single cluster, and easily federate 
  across regions and cloud providers.
* **HashiCorp Interoperability**: Nomad elegantly integrates with Vault for secrets
  management and Consul for service discovery and dynamic configuration. Nomad's 
  Consul-like architecture and Terraform-like job specification lower the barrier 
  to entry for existing users of the HashiCorp stack.

There are many relevant categories for comparison including cluster managers, 
resource managers, workload managers, and schedulers. There are many existing 
tools in each category, and the comparisons are not exhaustive of the entire space.

Due to the bias of the comparisons being on the Nomad website, we attempt to only 
use facts. If you find something that is invalid or out of date in the comparisons, 
please [open an issue](https://github.com/hashicorp/nomad/issues) and we will 
address it as soon as possible.

Use the navigation on the left to read comparisons of Nomad versus other systems.
