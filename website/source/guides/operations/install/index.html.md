---
layout: "guides"
page_title: "Installing Nomad"
sidebar_current: "guides-operations-installing"
description: |-
  Learn how to install Nomad.
---

# Installing Nomad

Installing Nomad is simple. There are two approaches to installing Nomad:

1. Using a <a href="#precompiled-binaries">precompiled binary</a>
1. Installing <a href="#from-source">from source</a>

Downloading a precompiled binary is easiest, and we provide downloads over
TLS along with SHA-256 sums to verify the binary.

<a name="precompiled-binaries"></a>
## Precompiled Binaries

To install the precompiled binary,
[download](/downloads.html) the appropriate package for your system.
Nomad is currently packaged as a zip file. We do not have any near term
plans to provide system packages.

Once the zip is downloaded, unzip it into any directory. The
`nomad` (or `nomad.exe` for Windows) binary inside is all that is
necessary to run Nomad. Any additional files, if any, are not
required to run Nomad.

Copy the binary to anywhere on your system. If you intend to access it
from the command-line, make sure to place it somewhere on your `PATH`.

<a name="from-source"></a>
## Compiling from Source

To compile from source, you will need [Go](https://golang.org) installed and
configured properly (including a `GOPATH` environment variable set), as well
as a copy of [`git`](https://www.git-scm.com/) in your `PATH`.

  1. Clone the Nomad repository from GitHub into your `GOPATH`:

    ```shell
    $ mkdir -p $GOPATH/src/github.com/hashicorp && cd $_
    $ git clone https://github.com/hashicorp/nomad.git
    $ cd nomad
    ```

  1. Bootstrap the project. This will download and compile libraries and tools
  needed to compile Nomad:

    ```shell
    $ make bootstrap
    ```

  1. Build Nomad for your current system and put the
  binary in `./bin/` (relative to the git checkout). The `make dev` target is
  just a shortcut that builds `nomad` for only your local build environment (no
  cross-compiled targets).

    ```shell
    $ make dev
    ```

## Verifying the Installation

To verify Nomad is properly installed, run `nomad -v` on your system. You should
see help output. If you are executing it from the command line, make sure it is
on your `PATH` or you may get an error about `nomad` not being found.

```shell
$ nomad -v
```
