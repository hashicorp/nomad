---
layout: "docs"
page_title: "JSON Job Specification"
sidebar_current: "docs-jobspec-json-syntax"
description: |-
  Learn about the Job specification used to submit jobs to Nomad in JSON.
---

# Job Specification

Jobs can be specified either in [HCL](https://github.com/hashicorp/hcl) or JSON.
This guide covers the json syntax for submitting jobs to Nomad. A useful command
for generating valid JSON versions of HCL jobs is `nomad run -output <job.nomad>`
which will emit a JSON version of the job.

## JSON Syntax

Below is an example of a json object that submits a `Periodic` job to Nomad:

```
{
"Job": {
      "Datacenters": [
        "dc1"
      ],
      "ID": "example1",
      "AllAtOnce": false,
      "Priority": 50,
      "Type": "batch",
      "Name": "example1",
      "Region": "global",
      "Constraints": [
        {
          "Operand": "=",
          "RTarget": "linux",
          "LTarget": "${attr.kernel.name}"
        }
      ],
      "TaskGroups": [
        {
          "Meta": null,
          "Tasks": [
            {
              "LogConfig": {
                "MaxFileSizeMB": 10,
                "MaxFiles": 10
              },
              "KillTimeout": 5000000000,
              "Name": "redis",
              "Driver": "docker",
              "Config": {
                "Command": "/bin/date",
                "port_map": [
                  {
                    "db": 6379
                  }
                ],
                "image": "redis:latest"
              },
              "Env": {
                  "foo": "bar"
              },
              "Services": [
                {
                  "Checks": [
                    {
                      "Timeout": 2000000000,
                      "Interval": 10000000000,
                      "Protocol": "",
                      "Path": "",
                      "Script": "",
                      "Type": "tcp",
                      "Name": "alive"
                    }
                  ],
                  "PortLabel": "db",
                  "Tags": [
                    "global",
                    "cache"
                  ],
                  "Name": "cache-redis"
                }
              ],
              "Constraints": null,
              "Resources": {
                "Networks": [
                  {
                    "Mbits": 10,
                    "DynamicPorts": [
                      {
                        "Value": 0,
                        "Label": "db"
                      }
                    ]
                  }
                ],
                "IOPS": 0,
                "DiskMB": 300,
                "MemoryMB": 256,
                "CPU": 500
              },
              "Meta": {
                  "foo": "bar",
                  "baz": "pipe"
              }
            }
          ],
          "RestartPolicy": {
            "Mode": "delay",
            "Delay": 25000000000,
            "Interval": 300000000000,
            "Attempts": 10
          },
          "Constraints": null,
          "Count": 1,
          "Name": "cache"
        }
      ],
      "Update": {
        "MaxParallel": 1,
        "Stagger": 10000000000
      },
      "Periodic": {
          "Enabled": true,
          "Spec": "* * * * *",
          "SpecType": "cron",
          "ProhibitOverlap": true
      },
      "Meta": {
          "foo": "bar",
          "baz": "pipe"
      }
    }
}
```

## Syntax Reference

Following is a syntax reference for the possible keys that are supported
and their default values if any for each type of object.

### Job

The `Job` object supports the following keys:

* `AllAtOnce` - Controls if the entire set of tasks in the job must
  be placed atomically or if they can be scheduled incrementally.
  This should only be used for special circumstances. Defaults to `false`.

* `Constraints` - A list to define additional constraints where a job can be
  run. See the constraint reference for more details.

* `Datacenters` - A list of datacenters in the region which are eligible
  for task placement. This must be provided, and does not have a default.

* `TaskGroups` - A list to define additional task groups. See the task group
  reference for more details.

* `Meta` - Annotates the job with opaque metadata.

* `Priority` - Specifies the job priority which is used to prioritize
  scheduling and access to resources. Must be between 1 and 100 inclusively,
  and defaults to 50.

* `Region` - The region to run the job in, defaults to "global".

* `Type` - Specifies the job type and switches which scheduler
  is used. Nomad provides the `service`, `system` and `batch` schedulers,
  and defaults to `service`. To learn more about each scheduler type visit
  [here](/docs/jobspec/schedulers.html)

*   `Update` - Specifies the task's update strategy. When omitted, rolling
    updates are disabled. The `Update` object supports the following attributes:

    * `MaxParallel` - `MaxParallel` is given as an integer value and specifies
      the number of tasks that can be updated at the same time.

    * `Stagger` - `Stagger` introduces a delay between sets of task updates and
      is given as an as a time duration. The value of stagger has to be in
      nanoseconds.

    An example `update` block:

    ```
    "Update": {
        "MaxParallel" : 3,
        "Stagger" : 300000000
    }
    ```

*   `Periodic` - `Periodic` allows the job to be scheduled at fixed times, dates
    or intervals. The `Periodic` object supports the following attributes:

    * `Enabled` - `Enabled` determines whether the periodic job will spawn child
    jobs.

    * `SpecType` - `SpecType` determines how Nomad is going to interpret the
      periodic expression. `cron` is the only supported `SpecType` currently.

    * `Spec` - A cron expression configuring the interval the job is launched
    at. Supports predefined expressions such as "@daily" and "@weekly" See
    [here](https://github.com/gorhill/cronexpr#implementation) for full
    documentation of supported cron specs and the predefined expressions.

    * <a id="prohibit_overlap">`ProhibitOverlap`</a> - `ProhibitOverlap` can
      be set to true to enforce that the periodic job doesn't spawn a new
      instance of the job if any of the previous jobs are still running. It is
      defaulted to false.

    An example `periodic` block:

    ```
        "Periodic": {
            "Spec": "*/15 * * * * *"
            "SpecType": "cron",
            "Enabled": true,
            "ProhibitOverlap": true
        }
    ```

### Task Group

`TaskGroups` is a list of `Task Group`, and each object supports the following
attributes:

* `Count` - Specifies the number of the task groups that should
  be running. Must be positive, defaults to one.

* `Constraints` - This is a list of `Constraint` objects. See the constraint
  reference for more details.

* `RestartPolicy` - Specifies the restart policy to be applied to tasks in this group.
  If omitted, a default policy for batch and non-batch jobs is used based on the
  job type. See the restart policy reference for more details.

* `Tasks` - It's a list of `Task` object, allows adding tasks as
  part of the group.

* `Meta` - Annotates the task group with opaque metadata.

### Task

The `Task` object supports the following keys:

* `Driver` - Specifies the task driver that should be used to run the
  task. See the [driver documentation](/docs/drivers/index.html) for what
  is available. Examples include `docker`, `qemu`, `java`, and `exec`.

* `Constraints` - This is a list of `Constraint` objects. See the constraint
  reference for more details.


* `Config` - A map of key/value configuration passed into the driver
  to start the task. The details of configurations are specific to
  each driver.

* `Services` - Nomad integrates with Consul for service discovery. A service
  block represents a routable and discoverable service on the network. Nomad
  automatically registers when a task is started and de-registers it when the
  task transitions to the dead state. [Click
  here](/docs/jobspec/servicediscovery.html) to learn more about services.

*   `Env` - A map of key/value representing environment variables that
    will be passed along to the running process. Nomad variables are
    interpreted when set in the environment variable values. See the table of
    interpreted variables [here](/docs/jobspec/interpreted.html).

    For example the below environment map will be reinterpreted:

    ```
        "Env": {
            "NODE_CLASS" : "${nomad.class}"
        }
    ```

* `Resources` - Provides the resource requirements of the task.
  See the resources reference for more details.

* `Meta` - Annotates the task group with opaque metadata.

* `KillTimeout` - `KillTimeout` is a time duration in nanoseconds. It can be
  used to configure the time between signaling a task it will be killed and
  actually killing it. Drivers first sends a task the `SIGINT` signal and then
  sends `SIGTERM` if the task doesn't die after the `KillTimeout` duration has
  elapsed.

* `LogConfig` - This allows configuring log rotation for the `stdout` and `stderr`
  buffers of a Task. See the log rotation reference below for more details.

### Resources

The `Resources` object supports the following keys:

* `CPU` - The CPU required in MHz.

* `DiskMB` - The disk required in MB.

* `IOPS` - The number of IOPS required given as a weight between 10-1000.

* `MemoryMB` - The memory required in MB.

* `Networks` - A list of network objects.

The Network object supports the following keys:

* `MBits` - The number of MBits in bandwidth required.

Nomad can allocate two types of ports to a task - Dynamic and Static ports. A
network object allows the user to specify a list of `DynamicPorts` orj
`StaticPorts`. Each object supports the following attributes:

* `Value` - The port number for static ports. If the port is dynamic, then this
  attribute is ignored.
* `Label` - The label to annotate a port so that it can be referred in the
  service discovery block or environment variables.

### Restart Policy

The `RestartPolicy` object supports the following keys:

* `Attempts` - `attempts` is the number of restarts allowed in an `interval`.

* `Interval` - `interval` is a time duration that can be specified using the
  `s`, `m`, and `h` suffixes, such as `30s`.  The `interval` begins when the
  first task starts and ensures that only `attempts` number of restarts happens
  within it. If more than `attempts` number of failures happen, behavior is
  controlled by `mode`.

* `Delay` - A duration to wait before restarting a task. It is specified in
  nanoseconds. A random jitter of up to 25% is added to the delay.

*   `Mode` - Controls the behavior when the task fails more than `Attempts`
    times in an interval. Possible values are listed below:

    * `delay` - `delay` will delay the next restart until the next `Interval` is
      reached.

    * `fail` - `fail` will not restart the task again.

### Constraint

The Constraint object supports the following keys:

* `Attribute` - Specifies the attribute to examine for the
  constraint. See the table of attributes [here](/docs/jobspec/interpreted.html#interpreted_node_vars).

* `Operator` - Specifies the comparison operator. Defaults to equality,
  and can be `=`, `==`, `is`, `!=`, `not`, `>`, `>=`, `<`, `<=`. The
  ordering is compared lexically.

* `Value` - Specifies the value to compare the attribute against.
  This can be a literal value or another attribute.

* `Version` - Specifies a version constraint against the attribute.
  This sets the operator to `version` and the `value` to what is
  specified. This supports a comma separated list of constraints,
  including the pessimistic operator. See the
  [go-version](https://github.com/hashicorp/go-version) repository
  for examples.

* `Regexp` - Specifies a regular expression constraint against
  the attribute. This sets the operator to "regexp" and the `value`
  to the regular expression.

*   `DistinctHosts` - `DistinctHosts` accepts a boolean `true`. The default is
    `false`.

    When `DistinctHosts` is `true` at the Job level, each instance of all task
    Groups specified in the job is placed on a separate host.

    When `DistinctHosts` is `true` at the task group level with count > 1, each
    instance of a task group is placed on a separate host. Different task groups in
    the same job _may_ be co-scheduled.

    Tasks within a task group are always co-scheduled.

### Log Rotation

The `LogConfig` object configures the log rotation policy for a task's `stdout` and
`stderr`. The `LogConfig` object supports the following attributes:

* `MaxFiles` - The maximum number of rotated files Nomad will retain for
  `stdout` and `stderr`, each tracked individually.

* `MaxFileSize` - The size of each rotated file. The size is specified in
  `MB`.

If the amount of disk resource requested for the task is less than the total
amount of disk space needed to retain the rotated set of files, Nomad will return
a validation error when a job is submitted.

```
"LogConfig: {
    "MaxFiles": 3,
    "MaxFileSizeMB": 10
}
```

In the above example we have asked Nomad to retain 3 rotated files for both
`stderr` and `stdout` and size of each file is 10MB. The minimum disk space that
would be required for the task would be 60MB.

### Artifact

Nomad downloads artifacts using
[`go-getter`](https://github.com/hashicorp/go-getter). The `go-getter` library
allows downloading of artifacts from various sources using a URL as the input
source. The key/value pairs given in the `options` block map directly to
parameters appended to the supplied `source` url. These are then used by
`go-getter` to appropriately download the artifact. `go-getter` also has a CLI
tool to validate its URL and can be used to check if the Nomad `artifact` is
valid.

Nomad allows downloading `http`, `https`, and `S3` artifacts. If these artifacts
are archives (zip, tar.gz, bz2, etc.), these will be unarchived before the task
is started.

The `Artifact` object maps supports the following keys:

* `GetterSource` - The path to the artifact to download.

* `RelativeDest` - The destination to download the artifact relative the task's
  directory.

* `GetterOptions` - A `map[string]string` block of options for `go-getter`.
  Full documentation of supported options are available
  [here](https://github.com/hashicorp/go-getter/tree/ef5edd3d8f6f482b775199be2f3734fd20e04d4a#protocol-specific-options-1).
  An example is given below:

```
"GetterOptions": {
    "checksum": "md5:c4aa853ad2215426eb7d70a21922e794",

    "aws_access_key_id": "<id>",
    "aws_access_key_secret": "<secret>",
    "aws_access_token": "<token>"
}
```

