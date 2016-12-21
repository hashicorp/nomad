# Go Bindings for LXC (Linux Containers)

This package implements [Go](http://golang.org) bindings for the [LXC](http://linuxcontainers.org/) C API (liblxc).

## Requirements

This package requires [LXC 1.x](https://github.com/lxc/lxc/releases) and its development package to be installed. Works with [Go 1.x](http://golang.org/dl). Following command should install required dependencies on Ubuntu:

	apt-get install -y pkg-config lxc-dev

## Installing

To install it, run:

    go get gopkg.in/lxc/go-lxc.v2

## Documentation

Documentation can be found at [GoDoc](http://godoc.org/gopkg.in/lxc/go-lxc.v2).

## Stability

The package API will remain stable as described in [gopkg.in](https://gopkg.in).

## Examples

See the [examples](https://github.com/lxc/go-lxc/tree/v2/examples) directory for some.

## Contributing

We'd love to see go-lxc improve. To contribute to go-lxc;

* **Fork** the repository
* **Modify** your fork
* Ensure your fork **passes all tests**
* **Send** a pull request
	* Bonus points if the pull request includes *what* you changed, *why* you changed it, and *tests* attached.
	* For the love of all that is holy, please use `go fmt` *before* you send the pull request.

We'll review it and merge it in if it's appropriate.
