---
layout: "intro"
page_title: "Install Nomad"
sidebar_current: "getting-started-install"
description: |-
  The first step to using Nomad is to get it installed.
---

# Install Nomad

To simplify the getting started experience, we will be working in a Vagrant
environment. Create a new directory, and download [this
`Vagrantfile`](https://raw.githubusercontent.com/hashicorp/nomad/master/demo/vagrant/Vagrantfile).

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
Usage: nomad [-version] [-help] [-autocomplete-(un)install] <command> [args]

Common commands:
    run         Run a new job or update an existing job
    stop        Stop a running job
    status      Display the status output for a resource
    alloc       Interact with allocations
    job         Interact with jobs
    node        Interact with nodes
    agent       Runs a Nomad agent

Other commands:
    acl             Interact with ACL policies and tokens
    agent-info      Display status information about the local agent
    deployment      Interact with deployments
    eval            Interact with evaluations
    namespace       Interact with namespaces
    operator        Provides cluster-level tools for Nomad operators
    quota           Interact with quotas
    sentinel        Interact with Sentinel policies
    server          Interact with servers
    ui              Open the Nomad Web UI
    version         Prints the Nomad version
```

If you get an error that Nomad could not be found, then your Vagrant box
may not have provisioned correctly. Check for any error messages that may have
been emitted during `vagrant up`. You can always [destroy the box][destroy] and
re-create it.

## Next Steps

Vagrant is running and Nomad is installed. Let's [start Nomad](/intro/getting-started/running.html)!

[destroy]: https://www.vagrantup.com/docs/cli/destroy.html
