---
layout: "docs"
page_title: "Upgrade Guides"
sidebar_current: "docs-upgrade-specific"
description: |-
  Specific versions of Nomad may have additional information about the upgrade
  process beyond the standard flow.
---

# Upgrading Specific Versions

The [upgrading page](/docs/upgrade/index.html) covers the details of doing
a standard upgrade. However, specific versions of Nomad may have more
details provided for their upgrades as a result of new features or changed
behavior. This page is used to document those details separately from the
standard upgrade flow.

## Nomad 0.6.0

### Default `advertise` address changes

When no `advertise` address was specified and Nomad's `bind_addr` was loopback
or `0.0.0.0`, Nomad attempted to resolve the local hostname to use as an
advertise address.

Many hosts cannot properly resolve their hostname, so Nomad 0.6 defaults
`advertise` to the first private IP on the host (e.g. `10.1.2.3`).

If you manually configure `advertise` addresses no changes are necessary.

## Nomad 0.5.5

### Docker `load` changes

Nomad 0.5.5 has a backward incompatible change in the `docker` driver's
configuration. Prior to 0.5.5 the `load` configuration option accepted a list
images to load, in 0.5.5 it has been changed to a single string. No
functionality was changed. Even if more than one item was specified prior to
0.5.5 only the first item was used.

To do a zero-downtime deploy with jobs that use the `load` option:

* Upgrade servers to version 0.5.5 or later.

* Deploy new client nodes on the same version as the servers.

* Resubmit jobs with the `load` option fixed and a constraint to only run on
  version 0.5.5 or later:

```hcl
    constraint {
      attribute = "${attr.nomad.version}"
      operator  = "version"
      value     = ">= 0.5.5"
    }
```

* Drain and shutdown old client nodes.

### Validation changes

Due to internal job serialization and validation changes you may run into
issues using 0.5.5 command line tools such as `nomad run` and `nomad validate`
with 0.5.4 or earlier agents.

It is recommended you upgrade agents before or alongside your command line
tools.

## Nomad 0.4.0

Nomad 0.4.0 has backward incompatible changes in the logic for Consul
deregistration.  When a Task which was started by Nomad v0.3.x is uncleanly shut
down, the Nomad 0.4 Client will no longer clean up any stale services.  If an
in-place upgrade of the Nomad client to 0.4 prevents the Task from gracefully
shutting down and deregistering its Consul-registered services, the Nomad Client
will not clean up the remaining Consul services registered with the 0.3
Executor.

We recommend draining a node before upgrading to 0.4.0 and then re-enabling the
node once the upgrade is complete.


## Nomad 0.3.1

Nomad 0.3.1 removes artifact downloading from driver configurations and places them as
a first class element of the task. As such, jobs will have to be rewritten in
the proper format and resubmitted to Nomad. Nomad clients will properly
re-attach to existing tasks but job definitions must be updated before they can
be dispatched to clients running 0.3.1.

## Nomad 0.3.0

Nomad 0.3.0 has made several substantial changes to job files included a new
`log` block and variable interpretation syntax (`${var}`), a modified `restart`
policy syntax, and minimum resources for tasks as well as validation. These
changes require a slight change to the default upgrade flow.

After upgrading the version of the servers, all previously submitted jobs must
be resubmitted with the updated job syntax using a Nomad 0.3.0 binary.

* All instances of `$var` must be converted to the new syntax of `${var}`

* All tasks must provide their required resources for CPU, memory and disk as
  well as required network usage if ports are required by the task.

* Restart policies must be updated to indicate whether it is desired for the
  task to restart on failure or to fail using `mode = "delay"` or `mode =
  "fail"` respectively.

* Service names that include periods will fail validation. To fix, remove any
  periods from the service name before running the job.

After updating the Servers and job files, Nomad Clients can be upgraded by first
draining the node so no tasks are running on it. This can be verified by running
`nomad node-status <node-id>` and verify there are no tasks in the `running`
state. Once that is done the client can be killed, the `data_dir` should be
deleted and then Nomad 0.3.0 can be launched.
