[![go-lxc](https://linuxcontainers.org/static/img/containers.png)](https://linuxcontainers.org/)
# Go Bindings for LXC (Linux Containers)

LXC is the well-known and heavily tested low-level Linux container runtime. It
is in active development since 2008 and has proven itself in critical
production environments world-wide. Some of its core contributors are the same
people that helped to implement various well-known containerization features
inside the Linux kernel.


This package implements [Go](https://golang.org) bindings for the [LXC](https://linuxcontainers.org/lxc/introduction/) C API (liblxc).

## Status
Type            | Service               | Status
---             | ---                   | ---
CI (Linux)      | Travis                | [![Build Status](https://travis-ci.org/lxc/go-lxc.svg?branch=v2)](https://travis-ci.org/lxc/go-lxc/)
Go documentation    | Godoc                 | [![GoDoc](https://godoc.org/gopkg.in/lxc/go-lxc.v2?status.svg)](https://godoc.org/gopkg.in/lxc/go-lxc.v2)
Static analysis     | GoReport              | [![Go Report Card](https://goreportcard.com/badge/gopkg.in/lxc/go-lxc.v2)](https://goreportcard.com/report/gopkg.in/lxc/go-lxc.v2)

## Requirements

This package requires [LXC >= 1.0.0](https://github.com/lxc/lxc/releases) and its development package and their dependencies to be installed. Additionally, go-lxc requires Golang 1.10 or later to work. Following command should install required dependencies on Ubuntu 18.10:

```bash
sudo apt update
sudo apt install git golang gcc make liblxc1 liblxc-dev lxc-utils pkg-config
```

## Installing

To install it, run:

```bash
go get gopkg.in/lxc/go-lxc.v2
```

## Trying

To try examples, run:

```bash
# cd ~/go/src/gopkg.in/lxc/go-lxc.v2/examples/

# make
==> Running go vet 
==> Building ...
...

# create/create
2018/12/27 22:39:27 Creating container...

# start/start 
2018/12/27 22:39:39 Starting the container...
2018/12/27 22:39:39 Waiting container to startup networking...

# attach/attach 
2018/12/27 22:39:46 AttachShell
root@rubik:/# hostname
rubik
root@rubik:/# exit
exit
2018/12/27 22:39:52 RunCommand
uid=0(root) gid=0(root) groups=0(root)

# stop/stop 
2018/12/27 22:39:54 Stopping the container...

# destroy/destroy 
2018/12/27 22:39:57 Destroying container...
```

## Stability

The package API will remain stable as described in [gopkg.in](https://gopkg.in).

## Backwards Compatibility

LXC has always focused on strong backwards compatibility. In fact, the API hasn't been broken from release `1.0.0` onwards. Main LXC is currently at version `2.*.*`.

## Examples

See the [examples](https://github.com/lxc/go-lxc/tree/v2/examples) directory for some.

## Bug reports

Bug reports can be filed at: <https://github.com/lxc/go-lxc/issues/new>

## Contributing

Fixes and new features are greatly appreciated. We'd love to see go-lxc improve. To contribute to go-lxc;

* **Fork** the repository
* **Modify** your fork
* Ensure your fork **passes all tests**
* **Send** a pull request
	* Bonus points if the pull request includes *what* you changed, *why* you changed it, and *tests* attached.

## Getting help

When you find you need help, the LXC projects provides you with several options.

### Discuss Forum

We maintain an discuss forum at

- https://discuss.linuxcontainers.org/

where you can get support.

### IRC

You can find support by joining `#lxcontainers` on `Freenode`.

### Mailing Lists

You can check out one of the two LXC mailing list archives and register if interested:

- http://lists.linuxcontainers.org/listinfo/lxc-devel
- http://lists.linuxcontainers.org/listinfo/lxc-users