---
layout: "docs"
page_title: "Upgrading Specific Versions"
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
