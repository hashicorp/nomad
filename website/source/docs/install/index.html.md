---
layout: "docs"
page_title: "Install Nomad"
sidebar_current: "docs-install"
description: |-
  Learn how to install Nomad.
---

# Install Nomad

Installing Nomad is simple. There are two approaches to installing Nomad:
downloading a precompiled binary for your system, or installing from source.

Downloading a precompiled binary is easiest, and we provide downloads over
TLS along with SHA256 sums to verify the binary.

## Precompiled Binaries

To install the precompiled binary,
[download](/downloads.html) the appropriate package for your system.
Nomad is currently packaged as a zip file. We do not have any near term
plans to provide system packages.

Once the zip is downloaded, unzip it into any directory. The
`nomad` binary inside is all that is necessary to run Nomad (or
`nomad.exe` for Windows). Any additional files, if any, aren't
required to run Nomad.

Copy the binary to anywhere on your system. If you intend to access it
from the command-line, make sure to place it somewhere on your `PATH`.

## Compiling from Source

To compile from source, you will need [Go](https://golang.org) installed
and configured properly. you will also need Git.

  1. Clone the Nomad repository into your GOPATH: https://github.com/hashicorp/nomad

  1. Verify that the file `$GOPATH/src/github.com/hashicorp/nomad/main.go`
     exists. If it does not, then you did not clone Nomad into the proper
     path.

  1. Run `make bootstrap`. This will download and compile libraries and tools needed
     to compile Nomad.

  1. Run `make`. This will build Nomad for your current system
     and put the binary in `bin` (relative to the git checkout).

## Verifying the Installation

To verify Nomad is properly installed, execute the `nomad` binary on
your system. You should see help output. If you are executing it from
the command line, make sure it is on your PATH or you may get an error
about `nomad` not being found.
