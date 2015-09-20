---
layout: "intro"
page_title: "Install Nomad"
sidebar_current: "getting-started-install"
description: |-
  The first step to using Nomad is to get it installed.
---

# Install Nomad

Nomad must first be installed on your machine. Nomad is distributed as
a [binary package](/downloads.html) for all supported platforms and
architectures. This page will not cover how to compile Nomad from source,
but compiling from source is covered in the [documentation](/docs/install/index.html)
for those who want to be sure they're compiling source they trust into
the final binary.

## Installing Nomad

To install Nomad, find the [appropriate package](/downloads.html) for
your system and download it. Nomad is packaged as a zip archive.

After downloading Nomad, unzip the package. Nomad runs as a single binary
named `nomad`. Any other files in the package can be safely removed and
Nomad will still function.

The final step is to make sure that `nomad` is available on the PATH.
See [this page](https://stackoverflow.com/questions/14637979/how-to-permanently-set-path-on-linux)
for instructions on setting the PATH on Linux and Mac.
[This page](https://stackoverflow.com/questions/1618280/where-can-i-set-path-to-make-exe-on-windows)
contains instructions for setting the PATH on Windows.

## Verifying the Installation

After installing Nomad, verify the installation worked by opening a new
terminal session and checking that `nomad` is available. By executing
`nomad`, you should see help output similar to the following:

```
$ nomad
usage: nomad [--version] [--help] <command> [<args>]

Available commands are:
    agent                Runs a Nomad agent
    agent-force-leave    Force a member into the 'left' state
    agent-info           Display status information about the local agent
    agent-join           Join server nodes together
    agent-members        Display a list of known members and their status
    node-drain           Toggle drain mode on a given node
    node-status          Display status information about nodes
    status               Display status information about jobs
    version              Prints the Nomad version
```

If you get an error that Nomad could not be found, then your PATH environment
variable was not setup properly. Please go back and ensure that your PATH
variable contains the directory where Nomad was installed.

Otherwise, Nomad is installed and ready to go!
