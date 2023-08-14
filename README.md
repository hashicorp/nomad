Nomad
[![License: BUSL-1.1](https://img.shields.io/badge/License-BUSL--1.1-yellow.svg)](LICENSE)
[![Discuss](https://img.shields.io/badge/discuss-nomad-00BC7F?style=flat)](https://discuss.hashicorp.com/c/nomad)
===

<p align="center" style="text-align:center;">
  <a href="https://nomadproject.io">
    <img alt="HashiCorp Nomad logo" src="website/public/img/logo-hashicorp.svg" width="500" />
  </a>
</p>

Nomad is a simple and flexible workload orchestrator to deploy and manage containers ([docker](https://www.nomadproject.io/docs/drivers/docker.html), [podman](https://www.nomadproject.io/docs/drivers/podman)), non-containerized applications ([executable](https://www.nomadproject.io/docs/drivers/exec.html), [Java](https://www.nomadproject.io/docs/drivers/java)), and virtual machines ([qemu](https://www.nomadproject.io/docs/drivers/qemu.html)) across on-prem and clouds at scale.

Nomad is supported on Linux, Windows, and macOS. A commercial version of Nomad, [Nomad Enterprise](https://www.nomadproject.io/docs/enterprise), is also available.

* Website: https://nomadproject.io
* Tutorials: [HashiCorp Learn](https://learn.hashicorp.com/nomad)
* Forum: [Discuss](https://discuss.hashicorp.com/c/nomad)

Nomad provides several key features:

* **Deploy Containers and Legacy Applications**: Nomad’s flexibility as an orchestrator enables an organization to run containers, legacy, and batch applications together on the same infrastructure.  Nomad brings core orchestration benefits to legacy applications without needing to containerize via pluggable task drivers.

* **Simple & Reliable**:  Nomad runs as a single binary and is entirely self contained - combining resource management and scheduling into a single system.  Nomad does not require any external services for storage or coordination.  Nomad automatically handles application, node, and driver failures.  Nomad is distributed and resilient, using leader election and state replication to provide high availability in the event of failures.

* **Device Plugins & GPU Support**: Nomad offers built-in support for GPU workloads such as machine learning (ML) and artificial intelligence (AI).  Nomad uses device plugins to automatically detect and utilize resources from hardware devices such as GPU, FPGAs, and TPUs.

* **Federation for Multi-Region, Multi-Cloud**: Nomad was designed to support infrastructure at a global scale.  Nomad supports federation out-of-the-box and can deploy applications across multiple regions and clouds.

* **Proven Scalability**: Nomad is optimistically concurrent, which increases throughput and reduces latency for workloads.  Nomad has been proven to scale to clusters of 10K+ nodes in real-world production environments.

* **HashiCorp Ecosystem**: Nomad integrates seamlessly with Terraform, Consul, Vault for provisioning, service discovery, and secrets management.

Quick Start
---

#### Testing
See [Learn: Getting Started](https://learn.hashicorp.com/collections/nomad/get-started) for instructions on setting up a local Nomad cluster for non-production use.

Optionally, find Terraform manifests for bringing up a development Nomad cluster on a public cloud in the [`terraform`](terraform/) directory.

#### Production
See [Learn: Nomad Reference Architecture](https://developer.hashicorp.com/nomad/tutorials/enterprise/production-reference-architecture-vm-with-consul) for recommended practices and a reference architecture for production deployments.

Documentation
---
Full, comprehensive documentation is available on the Nomad website: https://www.nomadproject.io/docs

Guides are available on [HashiCorp Learn](https://learn.hashicorp.com/nomad).

Roadmap
---

A timeline of major features expected for the next release or two can be found in the [Public Roadmap](https://github.com/orgs/hashicorp/projects/202/views/1).

This roadmap is a best guess at any given point, and both release dates and projects in each release are subject to change. Do not take any of these items as commitments, especially ones later than one major release away.

Contributing
--------------------
See the [`contributing`](contributing/) directory for more developer documentation.
