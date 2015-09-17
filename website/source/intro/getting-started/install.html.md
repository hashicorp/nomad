---
layout: "intro"
page_title: "Install Nomad"
sidebar_current: "gettingstarted-install"
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
named `vault`. Any other files in the package can be safely removed and
Nomad will still function.

The final step is to make sure that `vault` is available on the PATH.
See [this page](http://stackoverflow.com/questions/14637979/how-to-permanently-set-path-on-linux)
for instructions on setting the PATH on Linux and Mac.
[This page](http://stackoverflow.com/questions/1618280/where-can-i-set-path-to-make-exe-on-windows)
contains instructions for setting the PATH on Windows.

## Verifying the Installation

After installing Nomad, verify the installation worked by opening a new
terminal session and checking that `vault` is available. By executing
`vault`, you should see help output similar to the following:

```
$ vault
usage: vault [-version] [-help] <command> [args]

Common commands:
    delete           Delete operation on secrets in Nomad
    path-help        Look up the help for a path
    read             Read data or secrets from Nomad
    renew            Renew the lease of a secret
    revoke           Revoke a secret.
    server           Start a Nomad server
    status           Outputs status of whether Nomad is sealed and if HA mode is enabled
    write            Write secrets or configuration into Nomad

All other commands:
    audit-disable    Disable an audit backend
    audit-enable     Enable an audit backend
    audit-list       Lists enabled audit backends in Nomad
    auth             Prints information about how to authenticate with Nomad
    auth-disable     Disable an auth provider
    auth-enable      Enable a new auth provider
    init             Initialize a new Nomad server
    key-status       Provides information about the active encryption key
    mount            Mount a logical backend
    mounts           Lists mounted backends in Nomad
    policies         List the policies on the server
    policy-delete    Delete a policy from the server
    policy-write     Write a policy to the server
    rekey            Rekeys Nomad to generate new unseal keys
    remount          Remount a secret backend to a new path
    rotate           Rotates the backend encryption key used to persist data
    seal             Seals the vault server
    token-create     Create a new auth token
    token-renew      Renew an auth token
    token-revoke     Revoke one or more auth tokens
    unmount          Unmount a secret backend
    unseal           Unseals the vault server
    version          Prints the Nomad version
```

If you get an error that Nomad could not be found, then your PATH environment
variable was not setup properly. Please go back and ensure that your PATH
variable contains the directory where Nomad was installed.

Otherwise, Nomad is installed and ready to go!
