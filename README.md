Nomad [![Build Status](https://circleci.com/gh/hashicorp/nomad.svg?style=svg)](https://circleci.com/gh/hashicorp/nomad) [![Discuss](https://img.shields.io/badge/discuss-nomad-00BC7F?style=flat)](https://discuss.hashicorp.com/c/nomad)
=========

<p align="center" style="text-align:center;">
  <img src="https://github.com/hashicorp/nomad/blob/19c404ca791d6ebe95a81738d7dc6623ab28564d/website/public/img/logo-hashicorp.svg" width="500" />
</p>

Overview
-------------------------------

Nomad is an easy-to-use, flexible, and performant workload orchestrator that deploys:

* [Containers](https://www.nomadproject.io/docs/drivers/docker.html)
* [Non-containerized applications](https://www.nomadproject.io/docs/drivers/exec.html)
* [Virtual machines](https://www.nomadproject.io/docs/drivers/qemu.html)

Nomad enables developers to use declarative infrastructure-as-code for deploying their applications (jobs).  Nomad uses bin packing to efficiently schedule jobs and optimize for resource utilization.  Nomad is supported on macOS, Windows, and Linux.

Nomad is widely adopted and used in production by PagerDuty, CloudFlare, Roblox, Pandora, and more.

* **Deploy Containers and Legacy Applications**: Nomad’s flexibility as an orchestrator enables an organization to run containers, legacy, and batch applications together on the same infrastructure.  Nomad brings core orchestration benefits to legacy applications without needing to containerize via pluggable task drivers.

* **Simple & Reliable**:  Nomad runs as a single binary and is entirely self contained - combining resource management and scheduling into a single system.  Nomad does not require any external services for storage or coordination.  Nomad automatically handles application, node, and driver failures.  Nomad is distributed and resilient, using leader election and state replication to provide high availability in the event of failures.

* **Device Plugins & GPU Support**: Nomad offers built-in support for GPU workloads such as machine learning (ML) and artificial intelligence (AI).  Nomad uses device plugins to automatically detect and utilize resources from hardware devices such as GPU, FPGAs, and TPUs.

* **Federation for Multi-Region, Multi-Cloud**: Nomad was designed to support infrastructure at a global scale.  Nomad supports federation out-of-the-box and can deploy applications across multiple regions and clouds.

* **Proven Scalability**: Nomad is optimistically concurrent, which increases throughput and reduces latency for workloads.  Nomad has been proven to scale to clusters of 10K+ nodes in real-world production environments.

* **HashiCorp Ecosystem**: Nomad integrates seamlessly with Terraform, Consul, Vault for provisioning, service discovery, and secrets management.

Getting Started
-------------------------------

Get started with Nomad quickly in a sandbox environment on the public cloud or on your computer.

* Local
  * [Via Vagrant](https://www.nomadproject.io/intro/getting-started/install.html)
* AWS
  * [Via Terraform](https://github.com/hashicorp/nomad/tree/master/terraform/aws)
* Azure
  * [Via Terraform](https://github.com/hashicorp/nomad/tree/master/terraform/azure)
* GCP
  * [Via Terraform](https://github.com/hashicorp/nomad/tree/master/terraform/gcp)

These methods are not meant for production.

Documentation & Guides
-------------------------------

Documentation is available on the Nomad website [here](https://www.nomadproject.io/docs/index.html).
Guides are available on HashiCorp Learn website [here](https://learn.hashicorp.com/nomad).

Resources
-------------------------------

* Website
  * [www.nomadproject.io](https://www.nomadproject.io)
* Mailing List
  * [Google Groups](https://groups.google.com/group/nomad-tool)
* Gitter
  * [Nomad Chat Room](https://gitter.im/hashicorp-nomad/Lobby)

Who Uses Nomad
--------------------
* Roblox
  * [How Roblox built a platform for 100 million players with Nomad (2020)](https://www.hashicorp.com/case-studies/roblox/)
  * [How Roblox runs a platform for 70 million gamers on Nomad (2019)](https://portworx.com/architects-corner-roblox-runs-platform-70-million-gamers-hashicorp-nomad/)
* CloudFlare
  * [How We Use HashiCorp Nomad (2020)](https://blog.cloudflare.com/how-we-use-hashicorp-nomad/)
* BetterHelp
  * [How the world's largest online therapy provider runs on Nomad (2020)](https://www.youtube.com/watch?v=eN2ghrGpiUo)
* Navi Capital
  * [How Nomad powers a $1B hedge fund in Brazil (2020)](https://www.hashicorp.com/blog/nomad-community-story-navi-capital/)
* Trivago
  * [Maybe You Don’t Need Kubernetes (2019)](https://endler.dev/2019/maybe-you-dont-need-kubernetes/)
  * [Nomad - Our Experiences and Best Practices (2019)](https://tech.trivago.com/2019/01/25/nomad-our-experiences-and-best-practices/)
* Reaktor
  * [Nomad: Kubernetes, but without the complexity (2019)](https://youtu.be/GkmyNBUugg8)
* Pandora
  * [How Pandora Uses Nomad (2019)](https://www.youtube.com/watch?v=OsZeKTP2u98&t=2s)
* CircleCI
  * [How CircleCI Processes 4.5 Million Builds Per Month (2019)](https://stackshare.io/circleci/how-circleci-processes-4-5-million-builds-per-month)
  * [Security & Scheduling are Not Your Core Competencies (2018)](https://www.hashicorp.com/resources/nomad-vault-circleci-security-scheduling)
* Q2
  * [Q2’s Nomad Use and Overview (2019)](https://www.youtube.com/watch?v=OsZeKTP2u98&feature=youtu.be&t=1499)
* Citadel
  * [End-to-End Production Nomad at Citadel (2017)](https://www.hashicorp.com/resources/end-to-end-production-nomad-citadel)
  * [Extreme Scaling with HashiCorp Nomad & Consul (2016)](https://www.hashicorp.com/resources/citadel-scaling-hashicorp-nomad-consul)
* Deluxe Entertainment
  * [How Deluxe Uses the Complete HashiStack for Video Production (2018)](https://www.hashicorp.com/resources/deluxe-hashistack-video-production)
* Jet.com (Walmart)
  * [Driving down costs at Jet.com with HashiCorp Nomad (2017)](https://www.hashicorp.com/resources/jet-walmart-hashicorp-nomad-azure-run-apps)
* PagerDuty
  * [PagerDuty’s Nomadic Journey (2017)](https://www.hashicorp.com/resources/pagerduty-nomad-journey)
* SAP Ariba
  * [HashiCorp Nomad @ SAP Ariba (2018)](https://www.hashicorp.com/resources/nomad-community-call-core-team-sap-ariba)
* Target
  * [Nomad at Target:  Scaling Microservices Across Public and Private Clouds (2018)](https://www.hashicorp.com/resources/nomad-scaling-target-microservices-across-cloud)
  * [Playing with Nomad from HashiCorp (2017)](https://danielparker.me/nomad/hashicorp/schedulers/nomad/)
* Oscar Health
  * [Scalable CI at Oscar Health with Nomad and Docker (2018)](https://www.hashicorp.com/resources/scalable-ci-oscar-health-insurance-nomad-docker)
* eBay
  * [HashiStack at eBay: A Fully Containerized Platform Based on Infrastructure as Code (2018)](https://www.hashicorp.com/resources/ebay-hashistack-fully-containerized-platform-iac)
* Dutch National Police
  * [Going Cloud-Native at the Dutch National Police (2018)](https://www.hashicorp.com/resources/going-cloud-native-at-the-dutch-national-police)
* N26
  * [Tech at N26 - The Bank in the Cloud (2018)](https://medium.com/insiden26/tech-at-n26-the-bank-in-the-cloud-e5ff818b528b)
* Elsevier
  * [Eslevier’s Container Framework with Nomad, Terraform, and Consul (2017)](https://www.hashicorp.com/resources/elsevier-nomad-container-framework-demo)
* Graymeta
  * [Backend Batch Processing At Scale with Nomad (2017)](https://www.hashicorp.com/resources/backend-batch-processing-nomad)
* NIH NCBI
  * [NCBI’s Legacy Migration to Hybrid Cloud with Consul & Nomad (2018)](https://www.hashicorp.com/resources/ncbi-legacy-migration-hybrid-cloud-consul-nomad)
* imgix
  * [Cluster Schedulers & Why We Chose Nomad Over Kubernetes (2017)](https://medium.com/@copyconstruct/schedulers-kubernetes-and-nomad-b0f2e14a896)

...and more!

Contributing to Nomad
--------------------

If you wish to contribute to Nomad, you will  need [Go](https://www.golang.org) installed on your machine (version 1.15.1+ is *required*, and `gcc-go` is not supported).

See the [`contributing`](contributing/) directory for more developer documentation.

**Developing with Vagrant**
There is an included Vagrantfile that can help bootstrap the process. The
created virtual machine is based off of Ubuntu 16, and installs several of the
base libraries that can be used by Nomad.

To use this virtual machine, checkout Nomad and run `vagrant up` from the root
of the repository:

```sh
$ git clone https://github.com/hashicorp/nomad.git
$ cd nomad
$ vagrant up
```

The virtual machine will launch, and a provisioning script will install the
needed dependencies.

**Developing locally**
For local dev first make sure Go is properly installed, including setting up a
[GOPATH](https://golang.org/doc/code.html#GOPATH). After setting up Go, clone this
repository into `$GOPATH/src/github.com/hashicorp/nomad`. Then you can
download the required build tools such as vet, cover, godep etc by bootstrapping
your environment.

```sh
$ make bootstrap
...
```

Nomad creates many file handles for communicating with tasks, log handlers, etc.
In some development environments, particularly macOS, the default number of file
descriptors is too small to run Nomad's test suite. You should set
`ulimit -n 1024` or higher in your shell. This setting is scoped to your current
shell and doesn't affect other running shells or future shells.

Afterwards type `make test`. This will run the tests. If this exits with exit status 0,
then everything is working!

```sh
$ make test
...
```

To compile a development version of Nomad, run `make dev`. This will put the
Nomad binary in the `bin` and `$GOPATH/bin` folders:

```sh
$ make dev
```

Optionally run Consul to enable service discovery and health checks:

```sh
$ sudo consul agent -dev
```

And finally start the nomad agent:

```sh
$ sudo bin/nomad agent -dev
```

If the Nomad UI is desired in the development version, run `make dev-ui`. This will build the UI from source and compile it into the dev binary.

```sh
$ make dev-ui
...
$ bin/nomad
...

To compile protobuf files, installing protoc is required: See
https://github.com/google/protobuf for more information.
```

**Note:** Building the Nomad UI from source requires Node, Yarn, and Ember CLI. These tools are already in the Vagrant VM. Read the [UI README](https://github.com/hashicorp/nomad/blob/master/ui/README.md) for more info.

To cross-compile Nomad, run `make prerelease` and `make release`.
This will generate all the static assets, compile Nomad for multiple
platforms and place the resulting binaries into the `./pkg` directory:

```sh
$ make prerelease
$ make release
...
$ ls ./pkg
...
```

API Compatibility
--------------------

Only the `api/` and `plugins/` packages are intended to be imported by other projects. The root Nomad module does not follow semver and is not intended to be imported directly by other projects.
