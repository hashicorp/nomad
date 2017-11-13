---
layout: "intro"
page_title: "Install Nomad"
sidebar_current: "getting-started-install"
description: |-
  The first step to using Nomad is to get it installed.
---

# Install Nomad

The task drivers that are available to Nomad vary by operating system,
for example Docker is only available on Linux machines. To simplify the
getting started experience, we will be working in a Vagrant environment.
Create a new directory, and download [this `Vagrantfile`](https://raw.githubusercontent.com/hashicorp/nomad/master/demo/vagrant/Vagrantfile).

## Vagrant Setup

Note: To use the Vagrant Setup first install Vagrant following these instructions: https://www.vagrantup.com/docs/installation/

Once you have created a new directory and downloaded the `Vagrantfile`
you must create the virtual machine:

```shell
$ vagrant up
```

This will take a few minutes as the base Ubuntu box must be downloaded
and provisioned with both Docker and Nomad. Once this completes, you should
see output similar to:

```text
Bringing machine 'default' up with 'virtualbox' provider...
==> default: Importing base box 'bento/ubuntu-16.04'...
...
==> default: Running provisioner: docker...

```

At this point the Vagrant box is running and ready to go.

## Verifying the Installation

After starting the Vagrant box, verify the installation worked by connecting
to the box using SSH and checking that `nomad` is available. By executing
`nomad`, you should see help output similar to the following:

```shell
$ vagrant ssh
...

vagrant@nomad:~$ nomad

Usage: nomad [-version] [-help] [-autocomplete-(un)install] <command> [<args>]

Available commands are:
    acl                   Interact with ACL policies and tokens
    agent                 Runs a Nomad agent
    agent-info            Display status information about the local agent
    alloc-status          Display allocation status information and metadata
    client-config         View or modify client configuration details
    deployment            Interact with deployments
    eval-status           Display evaluation status and placement failure reasons
    fs                    Inspect the contents of an allocation directory
    init                  Create an example job file
    inspect               Inspect a submitted job
    job                   Interact with jobs
    keygen                Generates a new encryption key
    keyring               Manages gossip layer encryption keys
    logs                  Streams the logs of a task.
    namespace             Interact with namespaces
    node-drain            Toggle drain mode on a given node
    node-status           Display status information about nodes
    operator              Provides cluster-level tools for Nomad operators
    plan                  Dry-run a job update to determine its effects
    quota                 Interact with quotas
    run                   Run a new job or update an existing job
    sentinel              Interact with Sentinel policies
    server-force-leave    Force a server into the 'left' state
    server-join           Join server nodes together
    server-members        Display a list of known servers and their status
    status                Display the status output for a resource
    stop                  Stop a running job
    ui                    Open the Nomad Web UI
    validate              Checks if a given job specification is valid
    version               Prints the Nomad version
```

If you get an error that Nomad could not be found, then your Vagrant box
may not have provisioned correctly. Check for any error messages that may have
been emitted during `vagrant up`. You can always [destroy the box][destroy] and
re-create it.

## Next Steps

Vagrant is running and Nomad is installed. Let's [start Nomad](/intro/getting-started/running.html)!

[destroy]: https://www.vagrantup.com/docs/cli/destroy.html
