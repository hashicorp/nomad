---
layout: "docs"
page_title: "Job Specification"
sidebar_current: "docs-jobspec-syntax"
description: |-
  Learn about the Job specification used to submit jobs to Nomad.
---

# Job Specification

Jobs can be specified either in [HCL](https://github.com/hashicorp/hcl) or JSON.
HCL is meant to strike a balance between human readable and editable, and machine-friendly.

For machine-friendliness, Nomad can also read JSON configurations. In general, we recommend
using the HCL syntax.

## HCL Syntax

For a detailed description of HCL general syntax, [see this guide](https://github.com/hashicorp/hcl#syntax).
Here we cover the details of the Job specification for Nomad:

```
# Define a job called my-service
job "my-service" {
    # Job should run in the US region
    region = "us"

    # Spread tasks between us-west-1 and us-east-1
    datacenters = ["us-west-1", "us-east-1"]

    # run this job globally
    type = "system"

    # Rolling updates should be sequential
    update {
        stagger = "30s"
        max_parallel = 1
    }

    group "webs" {
        # We want 5 web servers
        count = 5

        # Create a web front end using a docker image
        task "frontend" {
            driver = "docker"
            config {
                image = "hashicorp/web-frontend"
            }
            service {
                port = "http"
                check {
                    type = "http"
                    path = "/health"
                    interval = "10s"
                    timeout = "2s"
                }
            }
            env {
                DB_HOST = "db01.example.com"
                DB_USER = "web"
                DB_PASSWORD = "loremipsum"
            }
            resources {
                cpu = 500
                memory = 128
                network {
                    mbits = 100
                    # Request for a dynamic port
                    port "http" {
                    }
                    # Request for a static port
                    port "https" {
                        static = 443
                    }
                }
            }
        }
    }
}
```

This is a fairly simple example job, but demonstrates many of the features and syntax
of the job specification. The primary "objects" are the job, task group, and task.
Each job file has only a single job, however a job may have multiple task groups,
and each task group may have multiple tasks. Task groups are a set of tasks that
must be co-located on a machine. Groups with a single task and count of one
can be declared outside of a group which is created implicitly.

Constraints can be specified at the job, task group, or task level to restrict
where a task is eligible for running. An example constraint looks like:

```
# Restrict to only nodes running linux
constraint {
    attribute = "$attr.kernel.name"
    value = "linux"
}
```

Jobs can also specify additional metadata at the job, task group, or task level.
This metadata is opaque to Nomad and can be used for any purpose, including
defining constraints on the metadata. Metadata can be specified by:

```
# Setup ELB via metadata and setup foo
meta {
    foo = "bar"
    elb_mode = "tcp"
    elb_check_interval = "10s"
}
```

## Syntax Reference

Following is a syntax reference for the possible keys that are supported
and their default values if any for each type of object.

### Job

The `job` object supports the following keys:

* `all_at_once` - Controls if the entire set of tasks in the job must
  be placed atomically or if they can be scheduled incrementally.
  This should only be used for special circumstances. Defaults to `false`.

* `constraint` - This can be provided multiple times to define additional
  constraints. See the constraint reference for more details.

* `datacenters` - A list of datacenters in the region which are eligible
  for task placement. This must be provided, and does not have a default.

* `group` - This can be provided multiple times to define additional
  task groups. See the task group reference for more details.

* `meta` - Annotates the job with opaque metadata.

* `priority` - Specifies the job priority which is used to prioritize
  scheduling and access to resources. Must be between 1 and 100 inclusively,
  and defaults to 50.

* `region` - The region to run the job in, defaults to "global".

* `task` - This can be specified multiple times to add a task as
  part of the job. Tasks defined directly in a job are wrapped in
  a task group of the same name.

* `type` - Specifies the job type and switches which scheduler
  is used. Nomad provides the `service`, `system` and `batch` schedulers,
  and defaults to `service`. To learn more about each scheduler type visit
  [here](/docs/jobspec/schedulers.html)

* `update` - Specifies the task update strategy. This requires providing
  `max_parallel` as an integer and `stagger` as a time duration. If stagger
  is provided as an integer, seconds are assumed. Otherwise the "s", "m",
  and "h" suffix can be used, such as "30s". Both values default to zero,
  which disables rolling updates.

*   `periodic` - `periodic` allows the job to be scheduled at fixed times, dates
    or intervals. The `periodic` block has the following configuration:

    ```
    periodic {
        // Enabled is defaulted to true if the block is included. It can be set
        // to false to pause the periodic job from running.
        enabled = true

        // A cron expression configuring the interval the job is launched at.
        // Supports predefined expressions such as "@daily" and "@weekly"
        cron = "*/15 * * * * *"
    }
    ```

    `cron`: See [here](https://github.com/gorhill/cronexpr#implementation)
    for full documentation of supported cron specs and the predefined expressions.

### Task Group

The `group` object supports the following keys:

* `count` - Specifies the number of the task groups that should
  be running. Must be positive, defaults to one.

* `constraint` - This can be provided multiple times to define additional
  constraints. See the constraint reference for more details.

* `restart` - Specifies the restart policy to be applied to tasks in this group.
  If omitted, a default policy for batch and non-batch jobs is used based on the
  job type. See the restart policy reference for more details.

* `task` - This can be specified multiple times, to add a task as
  part of the group.

* `meta` - Annotates the task group with opaque metadata.

### Task

The `task` object supports the following keys:

* `driver` - Specifies the task driver that should be used to run the
  task. See the [driver documentation](/docs/drivers/index.html) for what
  is available. Examples include `docker`, `qemu`, `java`, and `exec`.

* `constraint` - This can be provided multiple times to define additional
  constraints. See the constraint reference for more details.

* `config` - A map of key/value configuration passed into the driver
  to start the task. The details of configurations are specific to
  each driver.

* `service` - Nomad integrates with Consul for service discovery. A service
  block represents a routable and discoverable service on the network. Nomad
  automatically registers when a task is started and de-registers it when the
  task transitons to the dead state. [Click
  here](/docs/jobspec/servicediscovery.html) to learn more about services.

* `env` - A map of key/value representing environment variables that
  will be passed along to the running process.

* `resources` - Provides the resource requirements of the task.
  See the resources reference for more details.

* `meta` - Annotates the task group with opaque metadata.

### Resources

The `resources` object supports the following keys:

* `cpu` - The CPU required in MHz.

* `disk` - The disk required in MB.

* `iops` - The number of IOPS required given as a weight between 10-1000.

* `memory` - The memory required in MB.

* `network` - The network required. Details below.

The `network` object supports the following keys:

* `mbits` - The number of MBits in bandwidth required.

*   `port` - `port` is a repeatable object that can be used to specify both
    dynamic ports and reserved ports. It has the following format:

    ```
    port "label" {
        // If the `static` field is omitted, a dynamic port will be assigned.
        static = 6539
    }
    ```

### Restart Policy

The `restart` object supports the following keys:

* `attempts` - `attempts` is the number of restarts allowed in an `interval`.

* `interval` - `interval` is a time duration that can be specified using the
  `s`, `m`, and `h` suffixes, such as `30s`.  The `interval` begins when the
  first task starts and ensures that only `attempts` number of restarts happens
  within it. If more than `attempts` number of failures happen, behavior is
  controlled by `mode`.

* `delay` - A duration to wait before restarting a task. It is specified as a
  time duration using the `s`, `m`, and `h` suffixes, such as `30s`. A random
  jitter of up to 25% is added to the the delay.

* `on_success` - `on_success` controls whether a task is restarted when the
  task exits successfully.

*   `mode` - Controls the behavior when the task fails more than `attempts`
    times in an interval. Possible values are listed below:

    * `delay` - `delay` will delay the next restart until the next `interval` is
      reached.

    * `fail` - `fail` will not restart the task again.

The default `batch` restart policy is:

```
restart {
    attempts = 15
    delay = "15s"
    interval = "168h" # 7 days
    on_success = false
    mode = "delay"
}
```

The default non-batch restart policy is:

```
restart {
    interval = "1m"
    attempts = 2
    delay = "15s"
    on_success = true
    mode = "delay"
}
```

### Constraint

The `constraint` object supports the following keys:

* `attribute` - Specifies the attribute to examine for the
  constraint. See the table of attributes below.

* `operator` - Specifies the comparison operator. Defaults to equality,
  and can be `=`, `==`, `is`, `!=`, `not`, `>`, `>=`, `<`, `<=`. The
  ordering is compared lexically.

* `value` - Specifies the value to compare the attribute against.
  This can be a literal value or another attribute.

* `version` - Specifies a version constraint against the attribute.
  This sets the operator to `version` and the `value` to what is
  specified. This supports a comma seperated list of constraints,
  including the pessimistic operator. See the
  [go-version](https://github.com/hashicorp/go-version) repository
  for examples.

* `regexp` - Specifies a regular expression constraint against
  the attribute. This sets the operator to "regexp" and the `value`
  to the regular expression.

*   `distinct_hosts` - `distinct_hosts` accepts a boolean `true`. The default is
    `false`.

    When `distinct_hosts` is `true` at the Job level, each instance of all task
    Groups specified in the job is placed on a separate host.

    When `distinct_hosts` is `true` at the task group level with count > 1, each
    instance of a task group is placed on a separate host. Different task groups in
    the same job _may_ be co-scheduled.

    Tasks within a task group are always co-scheduled.

Below is a table documenting the variables that can be interpreted:

<table class="table table-bordered table-striped">
  <tr>
    <th>Variable</th>
    <th>Description</th>
  </tr>
  <tr>
    <td>$node.id</td>
    <td>The client node identifier</td>
  </tr>
  <tr>
    <td>$node.datacenter</td>
    <td>The client node datacenter</td>
  </tr>
  <tr>
    <td>$node.name</td>
    <td>The client node name</td>
  </tr>
  <tr>
    <td>$attr.\<key\></td>
    <td>The attribute given by `key` on the client node.</td>
  </tr>
  <tr>
    <td>$meta.\<key\></td>
    <td>The metadata value given by `key` on the client node.</td>
  </tr>
</table>

Below is a table documenting common node attributes:

<table class="table table-bordered table-striped">
  <tr>
    <th>Attribute</th>
    <th>Description</th>
  </tr>
  <tr>
    <td>arch</td>
    <td>CPU architecture of the client. Examples: `amd64`, `386`</td>
  </tr>
  <tr>
    <td>consul.datacenter</td>
    <td>The Consul datacenter of the client node if Consul found</td>
  </tr>
  <tr>
    <td>cpu.numcores</td>
    <td>Number of CPU cores on the client</td>
  </tr>
  <tr>
    <td>driver.\<key\></td>
    <td>See the [task drivers](/docs/drivers/index.html) for attribute documentation</td>
  </tr>
  <tr>
    <td>hostname</td>
    <td>Hostname of the client</td>
  </tr>
  <tr>
    <td>kernel.name</td>
    <td>Kernel of the client. Examples: `linux`, `darwin`</td>
  </tr>
  <tr>
    <td>kernel.version</td>
    <td>Version of the client kernel. Examples: `3.19.0-25-generic`, `15.0.0`</td>
  </tr>
  <tr>
    <td>platform.aws.ami-id</td>
    <td>On EC2, the AMI ID of the client node</td>
  </tr>
  <tr>
    <td>platform.aws.instance-type</td>
    <td>On EC2, the instance type of the client node</td>
  </tr>
  <tr>
    <td>os.name</td>
    <td>Operating system of the client. Examples: `ubuntu`, `windows`, `darwin`</td>
  </tr>
  <tr>
    <td>os.version</td>
    <td>Version of the client OS</td>
  </tr>
</table>

## JSON Syntax

Job files can also be specified in JSON. The conversion is straightforward
and self-documented. The downsides of JSON are less human readability and
the lack of comments. Otherwise, the two are completely interoperable.

See the API documentation for more details on the JSON structure.


