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

* **Multi-Datacenter and Multi-Region Aware**: Nomad is designed to be
  a global-scale scheduler. Multiple datacenters can be managed as part
  of a larger region, and jobs can be scheduled across datacenters if
  requested. Multiple regions join together and federate jobs making it
  easy to run jobs anywhere.

* **Operationally Simple**: Nomad runs as a single binary that can be
  either a client or server, and is completely self contained. Nomad does
  not require any external services for storage or coordination. This means
  Nomad combines the features of a resource manager and scheduler in a single
  system.

* **Distributed and Highly-Available**: Nomad servers cluster together and
  perform leader election and state replication to provide high availability
  in the face of failure. The Nomad scheduling engine is optimized for
  optimistic concurrency allowing all servers to make scheduling decisions to
  maximize throughput.

* **HashiCorp Ecosystem**: Nomad integrates with the entire HashiCorp
  ecosystem of tools. Along with all HashiCorp tools, Nomad is designed
  in the unix philosophy of doing something specific and doing it well.
  Nomad integrates with tools like Packer, Consul, and Terraform to support
  building artifacts, service discovery, monitoring and capacity management.

For more information, see the [introduction section](https://www.nomadproject.io/intro)
of the Nomad website.

Getting Started & Documentation
-------------------------------

All documentation is available on the [Nomad website](https://www.nomadproject.io).

Developing Nomad
--------------------

If you wish to work on Nomad itself or any of its built-in systems,
you will first need [Go](https://www.golang.org) installed on your
machine (version 1.9+ is *required*).

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
...
$ bin/nomad
...
```

If the Nomad UI is desired in the development version, run `make dev-ui`. This will build the UI from source and compile it into the dev binary.

```sh
$ make dev-ui
...
$ bin/nomad
...
```

**Note:** Building the Nomad UI from source requires Node, Yarn, and Ember CLI. These tools are already in the Vagrant VM. Read the [UI README](https://github.com/hashicorp/nomad/blob/master/ui/README.md) for more info.

To cross-compile Nomad, run `make release`. This will compile Nomad for multiple
platforms and place the resulting binaries into the `./pkg` directory:

```sh
$ make release
...
$ ls ./pkg
...
```
