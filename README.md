Nomad [![Build Status](https://travis-ci.org/hashicorp/nomad.svg)](https://travis-ci.org/hashicorp/nomad) [![Join the chat at https://gitter.im/hashicorp-nomad/Lobby](https://badges.gitter.im/hashicorp-nomad/Lobby.svg)](https://gitter.im/hashicorp-nomad/Lobby?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)
=========

<p align="center" style="text-align:center;">
  <img src="https://cdn.rawgit.com/hashicorp/nomad/master/website/source/assets/images/logo-text.svg" width="500" />
</p>

Overview
-------------------------------

Nomad is an easy-to-use, flexible, and performant workload orchestrator that deploys:

* [Containers](https://www.nomadproject.io/docs/drivers/docker.html)
* [Legacy applications](https://www.nomadproject.io/docs/drivers/exec.html)
* [Virtual machines](https://www.nomadproject.io/docs/drivers/qemu.html)

Nomad enables developers to use declarative infrastructure-as-code for deploying their applications (jobs).  Nomad uses bin packing to efficiently schedule jobs and optimize for resource utilization.  Nomad is supported on macOS, Windows, and Linux.

Nomad is widely adopted and used in production by PagerDuty, Target, Citadel, Trivago, SAP, Pandora, Roblox, eBay, Deluxe Entertainment, and more.   

* **Deploy Containers and Legacy Applications**: Nomad’s flexibility as an orchestrator enables an organization to run containers, legacy, and batch applications together on the same infrastructure.  Nomad brings core orchestration benefits to legacy applications without needing to containerize via pluggable task drivers.

* **Simple & Reliable**:  Nomad runs as a single 75MB binary and is entirely self contained - combining resource management and scheduling into a single system.  Nomad does not require any external services for storage or coordination.  Nomad automatically handles application, node, and driver failures.  Nomad is distributed and resilient, using leader election and state replication to provide high availability in the event of failures.   

* **Device Plugins & GPU Support**: Nomad offers built-in support for GPU workloads such as machine learning (ML) and artificial intelligence (AI).  Nomad uses device plugins to automatically detect and utilize resources from hardware devices such as GPU, FPGAs, and TPUs.

* **Federation for Multi-Region, Multi-Cloud**: Nomad was designed to support infrastructure at a global scale.  Nomad supports federation out-of-the-box and can deploy jobs across multiple regions and clouds.

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

These methods are not meant for production.

Documentation & Guides
-------------------------------

* [Installing Nomad for Production](https://www.nomadproject.io/guides/operations/deployment-guide.html)
* [Advanced Job Scheduling on Nomad with Affinities](https://www.nomadproject.io/guides/advanced-scheduling/affinity.html)
* [Increasing Nomad Fault Tolerance with Spread](https://www.nomadproject.io/guides/advanced-scheduling/spread.html)
* [Load Balancing on Nomad with Fabio & Consul](https://www.nomadproject.io/guides/load-balancing/fabio.html)
* [Deploying Stateful Workloads via Portworx](https://www.nomadproject.io/guides/stateful-workloads/portworx.html)
* [Running Apache Spark on Nomad](https://www.nomadproject.io/guides/spark/spark.html)
* [Integrating Vault with Nomad for Secrets Management](https://www.nomadproject.io/guides/operations/vault-integration/index.html)
* [Securing Nomad with TLS](https://www.nomadproject.io/guides/security/securing-nomad.html)
* [Continuous Deployment with Nomad and Terraform](https://www.hashicorp.com/blog/continuous-deployment-with-nomad-and-terraform)
* [Auto-bootstrapping a Nomad Cluster](https://www.hashicorp.com/blog/auto-bootstrapping-a-nomad-cluster)

Documentation is available on the Nomad website [here](https://www.nomadproject.io/docs/index.html).

Resources
-------------------------------

* Website
  * [www.nomadproject.io](https://www.nomadproject.io)
* Mailing List
  * [Google Groups](https://groups.google.com/group/nomad-tool)
* Gitter
  * [Nomad Chat Room](https://gitter.im/hashicorp-nomad/Lobby)
* Webinars
  * [Running Microservices with Nomad](https://www.hashicorp.com/resources/solutions-engineering-hangout-microservices-with-nomad)
  * [Running Heterogeneous Apps on Nomad](https://www.hashicorp.com/resources/se-hangout-running-heterogeneous-apps-nomad)
  * [Supporting Multiple Teams on a Single Nomad Cluster](https://www.hashicorp.com/resources/supporting-multiple-teams-single-nomad-cluster)
  * [Moving Your Legacy VMWare Workloads to Nomad](https://www.hashicorp.com/resources/move-your-vmware-workloads-nomad)
  * [Machine Learning Workflows with HashiCorp Nomad & Apache Spark](https://www.hashicorp.com/resources/machine-learning-workflows-hashicorp-nomad-apache-spark)
* Community Calls
  * [04/03/2019 with Pandora & Q2EBanking](https://www.youtube.com/watch?v=OsZeKTP2u98&t=2s)
  * [05/24/2018 with SAP Ariba](https://www.youtube.com/watch?v=eSwZwVVTDqw&t=2660s)

Who Uses Nomad
--------------------
* CircleCI
  * [How CircleCI Processes 4.5 Million Builds Per Month](https://stackshare.io/circleci/how-circleci-processes-4-5-million-builds-per-month)
  * [Security & Scheduling are Not Your Core Competencies](https://www.hashicorp.com/resources/nomad-vault-circleci-security-scheduling)
* Citadel
  * [End-to-End Production Nomad at Citadel](https://www.hashicorp.com/resources/end-to-end-production-nomad-citadel)
  * [Extreme Scaling with HashiCorp Nomad & Consul](https://www.hashicorp.com/resources/citadel-scaling-hashicorp-nomad-consul)
* Deluxe Entertainment
  * [How Deluxe Uses the Complete HashiStack for Video Production](https://www.hashicorp.com/resources/deluxe-hashistack-video-production)
* Jet.com (Walmart)
  * [Driving down costs at Jet.com with HashiCorp Nomad](https://www.hashicorp.com/resources/jet-walmart-hashicorp-nomad-azure-run-apps)
* PagerDuty
  * [PagerDuty’s Nomadic Journey](https://www.hashicorp.com/resources/pagerduty-nomad-journey)
* Pandora
  * [How Pandora Uses Nomad](https://www.youtube.com/watch?v=OsZeKTP2u98&t=2s)
* SAP Ariba
  * [HashiCorp Nomad @ SAP Ariba](https://www.hashicorp.com/resources/nomad-community-call-core-team-sap-ariba)
* SeatGeek
  * [Nomad Helper Tools](https://github.com/seatgeek/nomad-helper)
* Spaceflight Industries
  * [Spaceflight’s Hub-And-Spoke Infrastructure](https://www.hashicorp.com/blog/spaceflight-uses-hashicorp-consul-for-service-discovery-and-real-time-updates-to-their-hub-and-spoke-network-architecture)
* SpotInst
  * [SpotInst and HashiCorp Nomad to Reduce EC2 Costs for Users](https://www.hashicorp.com/blog/spotinst-and-hashicorp-nomad-to-reduce-ec2-costs-fo)
* Target
  * [Nomad at Target:  Scaling Microservices Across Public and Private Clouds](https://www.hashicorp.com/resources/nomad-scaling-target-microservices-across-cloud)
  * [Playing with Nomad from HashiCorp](https://danielparker.me/nomad/hashicorp/schedulers/nomad/)
* Trivago
  * [Maybe You Don’t Need Kubernetes](https://matthias-endler.de/2019/maybe-you-dont-need-kubernetes/)
  * [Nomad - Our Experiences and Best Practices](https://tech.trivago.com/2019/01/25/nomad-our-experiences-and-best-practices/)
* Roblox
  * [How Roblox runs a platform for 70 million gamers on HashiCorp Nomad](https://portworx.com/architects-corner-roblox-runs-platform-70-million-gamers-hashicorp-nomad/)
* Oscar Health
  * [Scalable CI at Oscar Health with Nomad and Docker](https://www.hashicorp.com/resources/scalable-ci-oscar-health-insurance-nomad-docker)
* eBay
  * [HashiStack at eBay: A Fully Containerized Platform Based on Infrastructure as Code](https://www.hashicorp.com/resources/ebay-hashistack-fully-containerized-platform-iac)
* Joyent
  * [Build Your Own Autoscaling Feature with HashiCorp Nomad](https://www.hashicorp.com/resources/autoscaling-hashicorp-nomad)
* Dutch National Police
  * [Going Cloud-Native at the Dutch National Police](https://www.hashicorp.com/resources/going-cloud-native-at-the-dutch-national-police)
* N26
  * [Tech at N26 - The Bank in the Cloud](https://medium.com/insiden26/tech-at-n26-the-bank-in-the-cloud-e5ff818b528b)
* Elsevier
  * [Eslevier’s Container Framework with Nomad, Terraform, and Consul](https://www.hashicorp.com/resources/elsevier-nomad-container-framework-demo)
* Palantir
  * [Enterprise Security at Palantir with the HashiCorp stack](https://www.hashicorp.com/resources/enterprise-security-hashicorp-stack)
* Graymeta
  * [Backend Batch Processing At Scale with Nomad](https://www.hashicorp.com/resources/backend-batch-processing-nomad)
* NIH NCBI
  * [NCBI’s Legacy Migration to Hybrid Cloud with Consul & Nomad](https://www.hashicorp.com/resources/ncbi-legacy-migration-hybrid-cloud-consul-nomad)
* Q2Ebanking
  * [Q2’s Nomad Use and Overview](https://www.youtube.com/watch?v=OsZeKTP2u98&feature=youtu.be&t=1499)
* imgix
  * [Cluster Schedulers & Why We Chose Nomad Over Kubernetes](https://medium.com/@copyconstruct/schedulers-kubernetes-and-nomad-b0f2e14a896)
* Region Syddanmark

...and more!

Contributing to Nomad
--------------------

If you wish to contribute to Nomad, you will  need [Go](https://www.golang.org) installed on your machine (version 1.11.11+ is *required*).

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
