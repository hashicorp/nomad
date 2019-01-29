---
layout: "docs"
page_title: "Task Drivers"
sidebar_current: "docs-drivers"
description: |-
  Task Drivers are used to integrate with the host OS to run tasks in Nomad.
---

# Task Drivers

Task drivers are used by Nomad clients to execute a task and provide resource
isolation. By having extensible task drivers, Nomad has the flexibility to
support a broad set of workloads across all major operating systems.

Starting with Nomad 0.9, task drivers are now pluggable. This gives users the
flexibility to introduce their own drivers without having to recompile Nomad.
You can view the [plugin stanza][plugin] documentation for examples on how to
use the `plugin` stanza in Nomad's client configuration. Note that we have
introduced new syntax when specifying driver options in the client configuration
(see [docker][docker_plugin] for an example). Keep in mind that even though all
built-in drivers are now plugins, Nomad remains a single binary and maintains
backwards compatibility except with the `lxc` driver. 

The list of supported task drivers is provided on the left of this page. Each
task driver documents the configuration available in a [job
specification](/docs/job-specification/index.html), the environments it can be
used in, and the resource isolation mechanisms available.

Nomad strives to mask the details of running a task from users and instead
provides a clean abstraction. It is possible for the same task to be executed
with different isolation levels depending on the client running the task. The
goal is to use the strictest isolation available and gracefully degrade
protections where necessary.

[plugin]: /docs/configuration/plugin.html
[docker_plugin]: /docs/drivers/docker.html#client-requirements
