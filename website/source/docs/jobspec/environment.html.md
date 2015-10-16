---
layout: "docs"
page_title: "Runtime Environment"
sidebar_current: "docs-jobspec-environment"
description: |-
  Learn how to configure the Nomad runtime environment.
---

# Runtime Environment

Some settings you specify in your [job specification](/docs/jobspec/) are passed to tasks
when they start. Other settings are dynamically allocated when your job is
scheduled. Both types of values are made available to your job through
environment variables.

## Resources

When you request resources for a job, Nomad creates a resource offer. The final
resources for your job are not determined until it is scheduled. Nomad will
tell you which resources have been allocated after evaluation and placement.

### CPU and Memory

Nomad will pass CPU and memory limits to your job as `NOMAD_CPU_LIMIT` and
`NOMAD_MEMORY_LIMIT`. Your task should use these values to adapt its behavior to
fit inside the resource allocation that nomad provides. For example, you can use
the memory limit to inform how large your in-process cache should be, or to
decide when to flush buffers to disk.

Both CPU and memory are presented as integers. The unit for CPU limit is
`1024 = 1Ghz`. The unit for memory is `1 = 1 megabytes`.

Writing your applications to adjust to these values at runtime provides greater
scheduling flexibility since you can adjust the resource allocations in your
job specification without needing to change your code. You can also schedule workloads
that accept dynamic resource allocations so they can scale down/up as your
cluster gets more or less busy.

### IPs and Named Ports

Each task will receive port allocations on a single IP address. The IP is made
available through `NOMAD_IP.`

If you requested reserved ports in your job specification and your task is successfully
scheduled, these ports are available for your use. Ports from `reserved_ports`
in the job spec are not exposed through the environment. If you requested
dynamic ports in your job specification these are made known to your application via
environment variables `NOMAD_PORT_{LABEL}`. For example
`dynamic_ports = ["HTTP"]` becomes `NOMAD_PORT_HTTP`.

Some drivers such as Docker and QEMU use port mapping. If a driver supports port
mapping and you specify a numeric label, the label will be automatically used as
the private port number. For example, `dynamic_ports = ["5000"]` will have a
random port mapped to port 5000 inside the container or VM. These ports are also
exported as environment variables for consistency, e.g. `NOMAD_PORT_5000`.

Please see the relevant driver documentation for details.

<a id="task_dir">### Task Directories</a>

Nomad makes the following two directories available to tasks:

* `alloc/`: This directory is shared across all tasks in a task group and can be
  used to store data that needs to be used by multiple tasks, such as a log
  shipper.
* `local/`: This directory is private to each task. It can be used to store
  arbitrary data that shouldn't be shared by tasks in the task group.

Both these directories are persisted until the allocation is removed, which
occurs hours after all the tasks in the task group enter terminal states. This
gives time to view the data produced by tasks.

Depending on the driver and operating system being targeted, the directories are
made available in various ways. For example, on `docker` the directories are
binded to the container, while on `exec` on Linux the directories are mounted into the
chroot. Regardless of how the directories are made available, the path to the
directories can be read through the following environment variables:
`NOMAD_ALLOC_DIR` and `NOMAD_TASK_DIR`.

## Meta

The job specification also allows you to specify a `meta` block to supply arbitrary
configuration to a task. This allows you to easily provide job-specific
configuration even if you use the same executable unit in multiple jobs. These
key-value pairs are passed through to the job as `NOMAD_META_{KEY}={value}`,
where `key` is UPPERCASED from the job specification.

Currently there is no enforcement that the meta values be lowercase, but using
multiple keys with the same uppercased representation will lead to undefined
behavior.
