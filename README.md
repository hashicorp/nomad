Nomad [![Build Status](https://travis-ci.org/hashicorp/nomad.svg)](https://travis-ci.org/hashicorp/nomad) [![Join the chat at https://gitter.im/hashicorp-nomad/Lobby](https://badges.gitter.im/hashicorp-nomad/Lobby.svg)](https://gitter.im/hashicorp-nomad/Lobby?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)
=========

* Website: [www.nomadproject.io](https://www.nomadproject.io)
* Mailing list: [Google Groups](https://groups.google.com/group/nomad-tool)

<p align="center" style="text-align:center;">
  <img src="https://cdn.rawgit.com/hashicorp/nomad/master/website/source/assets/images/logo-text.svg" width="500" />
</p>

Nomad is a cluster manager, designed for both long lived services and short
lived batch processing workloads. Developers use a declarative job specification
to submit work, and Nomad ensures constraints are satisfied and resource utilization
is optimized by efficient task packing. Nomad supports all major operating systems
and virtualized, containerized, or standalone applications.

The key features of Nomad are:

* **Docker Support**: Jobs can specify tasks which are Docker containers.
  Nomad will automatically run the containers on clients which have Docker
  installed, scale up and down based on the number of instances requested, and
  automatically recover from failures.

* **Operationally Simple**: Nomad runs as a single binary that can be
  either a client or server, and is completely self contained. Nomad does
  not require any external services for storage or coordination. This means
  Nomad combines the features of a resource manager and scheduler in a single
  system.

* **Multi-Datacenter and Multi-Region Aware**: Nomad is designed to be
  a global-scale scheduler. Multiple datacenters can be managed as part
  of a larger region, and jobs can be scheduled across datacenters if
  requested. Multiple regions join together and federate jobs making it
  easy to run jobs anywhere.

* **Flexible Workloads**: Nomad has extensible support for task drivers, allowing it to run
  containerized, virtualized, and standalone applications. Users can easily start Docker
  containers, VMs, or application runtimes like Java. Nomad supports Linux, Windows, BSD, and OSX,
  providing the flexibility to run any workload.

* **Built for Scale**: Nomad was designed from the ground up to support global scale
  infrastructure. Nomad is distributed and highly available, using both
  leader election and state replication to provide availability in the face
  of failures. Nomad is optimistically concurrent, enabling all servers to participate
  in scheduling decisions which increases the total throughput and reduces latency
  to support demanding workloads. Nomad has been proven to scale to cluster sizes that
  exceed 10k nodes in real-world production environments.

* **HashiCorp Ecosystem**: HashiCorp Ecosystem: Nomad integrates with the 
entire HashiCorp ecosystem of tools. Like all HashiCorp tools, Nomad follows 
the UNIX design philosophy of doing something specific and doing it well. 
Nomad integrates with Terraform, Consul, and Vault for provisioning, service 
discovery, and secrets management.

For more information, see the [introduction section](https://www.nomadproject.io/intro)
of the Nomad website.

Getting Started & Documentation
-------------------------------

All documentation is available on the [Nomad website](https://www.nomadproject.io).

Developing Nomad
--------------------

If you wish to work on Nomad itself or any of its built-in systems,
you will first need [Go](https://www.golang.org) installed on your
machine (version 1.10.2+ is *required*).

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
