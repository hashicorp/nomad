---
layout: api
page_title: JSON Job Specification - HTTP API
sidebar_current: api-json-jobs
description: |-
  Jobs can also be specified via the HTTP API using a JSON format. This guide
  discusses the job specification in JSON format.
---

# JSON Job Specification

This guide covers the JSON syntax for submitting jobs to Nomad. A useful command
for generating valid JSON versions of HCL jobs is:

```shell
$ nomad job run -output my-job.nomad
```

## Syntax

Below is the JSON representation of the job outputted by `$ nomad init`:

```json
{
    "Job": {
        "ID": "example",
        "Name": "example",
        "Type": "service",
        "Priority": 50,
        "Datacenters": [
            "dc1"
        ],
        "TaskGroups": [{
            "Name": "cache",
            "Count": 1,
            "Migrate": {
                    "HealthCheck": "checks",
                    "HealthyDeadline": 300000000000,
                    "MaxParallel": 1,
                    "MinHealthyTime": 10000000000
            },
            "Tasks": [{
                "Name": "redis",
                "Driver": "docker",
                "User": "",
                "Config": {
                    "image": "redis:3.2",
                    "port_map": [{
                        "db": 6379
                    }]
                },
                "Services": [{
                    "Id": "",
                    "Name": "redis-cache",
                    "Tags": [
                        "global",
                        "cache"
                    ],
                    "PortLabel": "db",
                    "AddressMode": "",
                    "Checks": [{
                        "Id": "",
                        "Name": "alive",
                        "Type": "tcp",
                        "Command": "",
                        "Args": null,
                        "Header": {},
                        "Method": "",
                        "Path": "",
                        "Protocol": "",
                        "PortLabel": "",
                        "Interval": 10000000000,
                        "Timeout": 2000000000,
                        "InitialStatus": "",
                        "TLSSkipVerify": false,
                        "CheckRestart": {
                            "Limit": 3,
                            "Grace": 30000000000,
                            "IgnoreWarnings": false
                        }
                    }]
                }],
                "Resources": {
                    "CPU": 500,
                    "MemoryMB": 256,
                    "Networks": [{
                        "Device": "",
                        "CIDR": "",
                        "IP": "",
                        "MBits": 10,
                        "DynamicPorts": [{
                            "Label": "db",
                            "Value": 0
                        }]
                    }]
                },
                "Leader": false
            }],
            "RestartPolicy": {
                "Interval": 1800000000000,
                "Attempts": 2,
                "Delay": 15000000000,
                "Mode": "fail"
            },
            "ReschedulePolicy": {
                "Attempts": 10,
                "Delay": 30000000000,
                "DelayFunction": "exponential",
                "Interval": 0,
                "MaxDelay": 3600000000000,
                "Unlimited": true
            },
            "EphemeralDisk": {
                "SizeMB": 300
            }
        }],
        "Update": {
            "MaxParallel": 1,
            "MinHealthyTime": 10000000000,
            "HealthyDeadline": 180000000000,
            "AutoRevert": false,
            "Canary": 0
        }
    }
}
```

The example JSON could be submitted as a job using the following:

```text
$ curl -XPUT -d @example.json http://127.0.0.1:4646/v1/job/example
{
  "EvalID": "5d6ded54-0b2a-8858-6583-be5f476dec9d",
  "EvalCreateIndex": 12,
  "JobModifyIndex": 11,
  "Warnings": "",
  "Index": 12,
  "LastContact": 0,
  "KnownLeader": false
}
```

## Syntax Reference

Following is a syntax reference for the possible keys that are supported and
their default values if any for each type of object.

### Job

The `Job` object supports the following keys:

- `AllAtOnce` - Controls whether the scheduler can make partial placements if
  optimistic scheduling resulted in an oversubscribed node. This does not
  control whether all allocations for the job, where all would be the desired
  count for each task group, must be placed atomically. This should only be
  used for special circumstances. Defaults to `false`.

- `Constraints` - A list to define additional constraints where a job can be
  run. See the constraint reference for more details.

- `Datacenters` - A list of datacenters in the region which are eligible
  for task placement. This must be provided, and does not have a default.

- `TaskGroups` - A list to define additional task groups. See the task group
  reference for more details.

- `Meta` - Annotates the job with opaque metadata.

- `Namespace` - The namespace to execute the job in, defaults to "default".
  Values other than default are not allowed in non-Enterprise versions of Nomad.

- `ParameterizedJob` - Specifies the job as a parameterized job such that it can
  be dispatched against. The `ParameterizedJob` object supports the following
  attributes:

  - `MetaOptional` - Specifies the set of metadata keys that may be provided
    when dispatching against the job as a string array.

  - `MetaRequired` - Specifies the set of metadata keys that must be provided
    when dispatching against the job as a string array.

  - `Payload` - Specifies the requirement of providing a payload when
    dispatching against the parameterized job. The options for this field are
    "optional", "required" and "forbidden". The default value is "optional".

- `Payload` - The payload may not be set when submitting a job but may appear in
  a dispatched job. The `Payload` will be a base64 encoded string containing the
  payload that the job was dispatched with. The `payload` has a **maximum size
  of 16 KiB**.

- `Priority` - Specifies the job priority which is used to prioritize
  scheduling and access to resources. Must be between 1 and 100 inclusively,
  and defaults to 50.

- `Region` - The region to run the job in, defaults to "global".

- `Type` - Specifies the job type and switches which scheduler
  is used. Nomad provides the `service`, `system` and `batch` schedulers,
  and defaults to `service`. To learn more about each scheduler type visit
  [here](/docs/schedulers.html)

- `Update` - Specifies an update strategy to be applied to all task groups
  within the job. When specified both at the job level and the task group level,
  the update blocks are merged with the task group's taking precedence. For more
  details on the update stanza, please see below.

-   `Periodic` - `Periodic` allows the job to be scheduled at fixed times, dates
    or intervals. The periodic expression is always evaluated in the UTC
    timezone to ensure consistent evaluation when Nomad Servers span multiple
    time zones. The `Periodic` object is optional and supports the following attributes:

    - `Enabled` - `Enabled` determines whether the periodic job will spawn child
    jobs.

    - `TimeZone` - Specifies the time zone to evaluate the next launch interval
      against. This is useful when wanting to account for day light savings in
      various time zones. The time zone must be parsable by Golang's
      [LoadLocation](https://golang.org/pkg/time/#LoadLocation). The default is
      UTC.

    - `SpecType` - `SpecType` determines how Nomad is going to interpret the
      periodic expression. `cron` is the only supported `SpecType` currently.

    - `Spec` - A cron expression configuring the interval the job is launched
    at. Supports predefined expressions such as "@daily" and "@weekly" See
    [here](https://github.com/gorhill/cronexpr#implementation) for full
    documentation of supported cron specs and the predefined expressions.

    - <a id="prohibit_overlap">`ProhibitOverlap`</a> - `ProhibitOverlap` can
      be set to true to enforce that the periodic job doesn't spawn a new
      instance of the job if any of the previous jobs are still running. It is
      defaulted to false.

    An example `periodic` block:

    ```json
    {
      "Periodic": {
          "Spec": "*/15 - *",
          "TimeZone": "Europe/Berlin",
          "SpecType": "cron",
          "Enabled": true,
          "ProhibitOverlap": true
      }
    }
    ```

- `ReschedulePolicy` - Specifies a reschedule policy to be applied to all task groups
  within the job. When specified both at the job level and the task group level,
  the reschedule blocks are merged, with the task group's taking precedence. For more
  details on `ReschedulePolicy`, please see below.

### Task Group

`TaskGroups` is a list of `TaskGroup` objects, each supports the following
attributes:

- `Constraints` - This is a list of `Constraint` objects. See the constraint
  reference for more details.

- `Count` - Specifies the number of the task groups that should
  be running. Must be non-negative, defaults to one.

- `Meta` - A key-value map that annotates the task group with opaque metadata.

- `Migrate` - Specifies a migration strategy to be applied during [node
  drains][drain].

  - `HealthCheck` - One of `checks` or `task_states`. Indicates how task health
    should be determined: either via Consul health checks or whether the task
    was able to run successfully.

  - `HealthyDeadline` - Specifies duration a task must become healthy within
    before it is considered unhealthy.

  - `MaxParallel` - Specifies how many allocations may be migrated at once.

  - `MinHealthyTime` - Specifies duration a task must be considered healthy
    before the migration is considered healthy.

- `Name` - The name of the task group. Must be specified.

- `RestartPolicy` - Specifies the restart policy to be applied to tasks in this group.
  If omitted, a default policy for batch and non-batch jobs is used based on the
  job type. See the [restart policy reference](#restart_policy) for more details.

- `ReschedulePolicy` - Specifies the reschedule policy to be applied to tasks in this group.
  If omitted, a default policy is used for batch and service jobs. System jobs are not eligible
  for rescheduling. See the [reschedule policy reference](#reschedule_policy) for more details.

- `EphemeralDisk` - Specifies the group's ephemeral disk requirements. See the
  [ephemeral disk reference](#ephemeral_disk) for more details.

- `Update` - Specifies an update strategy to be applied to all task groups
  within the job. When specified both at the job level and the task group level,
  the update blocks are merged with the task group's taking precedence. For more
  details on the update stanza, please see below.

- `Tasks` - A list of `Task` object that are part of the task group.

### Task

The `Task` object supports the following keys:

- `Artifacts` - `Artifacts` is a list of `Artifact` objects which define
  artifacts to be downloaded before the task is run. See the artifacts
  reference for more details.

- `Config` - A map of key-value configuration passed into the driver
  to start the task. The details of configurations are specific to
  each driver.

- `Constraints` - This is a list of `Constraint` objects. See the constraint
  reference for more details.

- `DispatchPayload` - Configures the task to have access to dispatch payloads.
  The `DispatchPayload` object supports the following attributes:

  - `File` - Specifies the file name to write the content of dispatch payload
    to. The file is written relative to the task's local directory.

- `Driver` - Specifies the task driver that should be used to run the
  task. See the [driver documentation](/docs/drivers/index.html) for what
  is available. Examples include `docker`, `qemu`, `java`, and `exec`.

-   `Env` - A map of key-value representing environment variables that
    will be passed along to the running process. Nomad variables are
    interpreted when set in the environment variable values. See the table of
    interpreted variables [here](/docs/runtime/interpolation.html).

    For example the below environment map will be reinterpreted:

    ```json
    {
      "Env": {
        "NODE_CLASS" : "${nomad.class}"
      }
    }
    ```

- `KillSignal` - Specifies a configurable kill signal for a task, where the
  default is SIGINT. Note that this is only supported for drivers which accept
  sending signals (currently `docker`, `exec`, `raw_exec`, and `java` drivers).

- `KillTimeout` - `KillTimeout` is a time duration in nanoseconds. It can be
  used to configure the time between signaling a task it will be killed and
  actually killing it. Drivers first sends a task the `SIGINT` signal and then
  sends `SIGTERM` if the task doesn't die after the `KillTimeout` duration has
  elapsed. The default `KillTimeout` is 5 seconds.

- `Leader` - Specifies whether the task is the leader task of the task group. If
  set to true, when the leader task completes, all other tasks within the task
  group will be gracefully shutdown.

- `LogConfig` - This allows configuring log rotation for the `stdout` and `stderr`
  buffers of a Task. See the log rotation reference below for more details.

- `Meta` - Annotates the task group with opaque metadata.

- `Name` - The name of the task. This field is required.

- `Resources` - Provides the resource requirements of the task.
  See the resources reference for more details.

- `Services` - `Services` is a list of `Service` objects. Nomad integrates with
  Consul for service discovery. A `Service` object represents a routable and
  discoverable service on the network. Nomad automatically registers when a task
  is started and de-registers it when the task transitions to the dead state.
  [Click here](/guides/operations/consul-integration/index.html#service-discovery) to learn more about
  services. Below is the fields in the `Service` object:

     - `Name`: An explicit name for the Service. Nomad will replace `${JOB}`,
       `${TASKGROUP}` and `${TASK}` by the name of the job, task group or task,
       respectively. `${BASE}` expands to the equivalent of
       `${JOB}-${TASKGROUP}-${TASK}`, and is the default name for a Service.
       Each service defined for a given task must have a distinct name, so if
       a task has multiple services only one of them can use the default name
       and the others must be explicitly named. Names must adhere to
       [RFC-1123 §2.1](https://tools.ietf.org/html/rfc1123#section-2) and are
       limited to alphanumeric and hyphen characters (i.e. `[a-z0-9\-]`), and be
       less than 64 characters in length.

     - `Tags`: A list of string tags associated with this Service. String
       interpolation is supported in tags.

     - `CanaryTags`: A list of string tags associated with this Service while it
       is a canary. Once the canary is promoted, the registered tags will be
       updated to the set defined in the `Tags` field. String interpolation is
       supported in tags.

     - `PortLabel`: `PortLabel` is an optional string and is used to associate
       a port with the service.  If specified, the port label must match one
       defined in the resources block.  This could be a label of either a
       dynamic or a static port.

     - `AddressMode`: Specifies what address (host or driver-specific) this
       service should advertise.  This setting is supported in Docker since
       Nomad 0.6 and rkt since Nomad 0.7. Valid options are:

       - `auto` - Allows the driver to determine whether the host or driver
         address should be used. Defaults to `host` and only implemented by
	 Docker. If you use a Docker network plugin such as weave, Docker will
         automatically use its address.

       - `driver` - Use the IP specified by the driver, and the port specified
         in a port map. A numeric port may be specified since port maps aren't
	 required by all network plugins. Useful for advertising SDN and
         overlay network addresses. Task will fail if driver network cannot be
         determined. Only implemented for Docker and rkt.

       - `host` - Use the host IP and port.

     - `Checks`: `Checks` is an array of check objects. A check object defines a
       health check associated with the service. Nomad supports the `script`,
       `http` and `tcp` Consul Checks. Script checks are not supported for the
       qemu driver since the Nomad client doesn't have access to the file system
       of a task using the Qemu driver.

         - `Type`:  This indicates the check types supported by Nomad. Valid
           options are currently `script`, `http` and `tcp`.

         - `Name`: The name of the health check.

	 - `AddressMode`: Same as `AddressMode` on `Service`.  Unlike services,
	   checks do not have an `auto` address mode as there's no way for
	   Nomad to know which is the best address to use for checks. Consul
	   needs access to the address for any HTTP or TCP checks. Added in
	   Nomad 0.7.1. Unlike `PortLabel`, this setting is *not* inherited
           from the `Service`.

	 - `PortLabel`: Specifies the label of the port on which the check will
	   be performed. Note this is the _label_ of the port and not the port
	   number unless `AddressMode: "driver"`. The port label must match one
	   defined in the Network stanza. If a port value was declared on the
	   `Service`, this will inherit from that value if not supplied. If
	   supplied, this value takes precedence over the `Service.PortLabel`
	   value. This is useful for services which operate on multiple ports.
	  `http` and `tcp` checks require a port while `script` checks do not.
	  Checks will use the host IP and ports by default. In Nomad 0.7.1 or
	  later numeric ports may be used if `AddressMode: "driver"` is set on
          the check.

	 - `Header`: Headers for HTTP checks. Should be an object where the
	   values are an array of values. Headers will be written once for each
           value.

         - `Interval`: This indicates the frequency of the health checks that
           Consul will perform.

         - `Timeout`: This indicates how long Consul will wait for a health
           check query to succeed.

         - `Method`: The HTTP method to use for HTTP checks. Defaults to GET.

         - `Path`: The path of the HTTP endpoint which Consul will query to query
           the health of a service if the type of the check is `http`. Nomad
           will add the IP of the service and the port, users are only required
           to add the relative URL of the health check endpoint. Absolute paths
           are not allowed.

         - `Protocol`: This indicates the protocol for the HTTP checks. Valid
           options are `http` and `https`. We default it to `http`.

         - `Command`: This is the command that the Nomad client runs for doing
           script based health check.

         - `Args`: Additional arguments to the `command` for script based health
           checks.

	 - `TLSSkipVerify`: If true, Consul will not attempt to verify the
	   certificate when performing HTTPS checks. Requires Consul >= 0.7.2.

	   - `CheckRestart`: `CheckRestart` is an object which enables
	     restarting of tasks based upon Consul health checks.

	     - `Limit`: The number of unhealthy checks allowed before the
	       service is restarted. Defaults to `0` which disables
               health-based restarts.

	     - `Grace`: The duration to wait after a task starts or restarts
	       before counting unhealthy checks count against the limit.
               Defaults to "1s".

	     - `IgnoreWarnings`: Treat checks that are warning as passing.
	       Defaults to false which means warnings are considered unhealthy.

- `ShutdownDelay` - Specifies the duration to wait when killing a task between
  removing it from Consul and sending it a shutdown signal. Ideally services
  would fail healthchecks once they receive a shutdown signal. Alternatively
  `ShutdownDelay` may be set to give in flight requests time to complete before
  shutting down.

- `Templates` - Specifies the set of [`Template`](#template) objects to render for the task.
  Templates can be used to inject both static and dynamic configuration with
  data populated from environment variables, Consul and Vault.

- `User` - Set the user that will run the task. It defaults to the same user
  the Nomad client is being run as. This can only be set on Linux platforms.

### Resources

The `Resources` object supports the following keys:

- `CPU` - The CPU required in MHz.

- `IOPS` - The number of IOPS required given as a weight between 10-1000.

- `MemoryMB` - The memory required in MB.

- `Networks` - A list of network objects.

The Network object supports the following keys:

- `MBits` - The number of MBits in bandwidth required.

Nomad can allocate two types of ports to a task - Dynamic and Static/Reserved
ports. A network object allows the user to specify a list of `DynamicPorts` and
`ReservedPorts`. Each object supports the following attributes:

- `Value` - The port number for static ports. If the port is dynamic, then this
  attribute is ignored.
- `Label` - The label to annotate a port so that it can be referred in the
  service discovery block or environment variables.

<a id="ephemeral_disk"></a>

### Ephemeral Disk

The `EphemeralDisk` object supports the following keys:

- `Migrate` - Specifies that the Nomad client should make a best-effort attempt
  to migrate the data from a remote machine if placement cannot be made on the
  original node. During data migration, the task will block starting until the
  data migration has completed. Value is a boolean and the default is false.

- `SizeMB` - Specifies the size of the ephemeral disk in MB. Default is 300.

- `Sticky` - Specifies that Nomad should make a best-effort attempt to place the
  updated allocation on the same machine. This will move the `local/` and
  `alloc/data` directories to the new allocation. Value is a boolean and the
  default is false.

<a id="reschedule_policy"></a>

### Reschedule Policy

The `ReschedulePolicy` object supports the following keys:

- `Attempts` - `Attempts` is the number of reschedule attempts allowed
  in an `Interval`.

- `Interval` - `Interval` is a time duration that is specified in nanoseconds.
  The `Interval` is a sliding window within which at most `Attempts` number
  of reschedule attempts are permitted.

- `Delay` - A duration to wait before attempting rescheduling. It is specified in
  nanoseconds.

- `DelayFunction` - Specifies the function that is used to calculate subsequent reschedule delays.
  The initial delay is specified by the `Delay` parameter. Allowed values for `DelayFunction` are listed below:
    - `constant` - The delay between reschedule attempts stays at the `Delay` value.
    - `exponential` - The delay between reschedule attempts doubles.
    - `fibonacci` - The delay between reschedule attempts is calculated by adding the two most recent
      delays applied. For example if `Delay` is set to 5 seconds, the next five reschedule attempts  will be
      delayed by 5 seconds, 5 seconds, 10 seconds, 15 seconds, and 25 seconds respectively.

- `MaxDelay`  - `MaxDelay` is an upper bound on the delay beyond which it will not increase. This parameter is used when
   `DelayFunction` is `exponential` or `fibonacci`, and is ignored when `constant` delay is used.

- `Unlimited` - `Unlimited` enables unlimited reschedule attempts. If this is set to true
  the `Attempts` and `Interval` fields are not used.


<a id="restart_policy"></a>

### Restart Policy

The `RestartPolicy` object supports the following keys:

- `Attempts` - `Attempts` is the number of restarts allowed in an `Interval`.

- `Interval` - `Interval` is a time duration that is specified in nanoseconds.
  The `Interval` begins when the first task starts and ensures that only
  `Attempts` number of restarts happens within it. If more than `Attempts`
  number of failures happen, behavior is controlled by `Mode`.

- `Delay` - A duration to wait before restarting a task. It is specified in
  nanoseconds. A random jitter of up to 25% is added to the delay.

-   `Mode` - `Mode` is given as a string and controls the behavior when the task
    fails more than `Attempts` times in an `Interval`. Possible values are listed
    below:

    - `delay` - `delay` will delay the next restart until the next `Interval` is
      reached.

    - `fail` - `fail` will not restart the task again.

### Update

Specifies the task group update strategy. When omitted, rolling updates are
disabled. The update stanza can be specified at the job or task group level.
When specified at the job, the update stanza is inherited by all task groups.
When specified in both the job and in a task group, the stanzas are merged with
the task group's taking precedence. The `Update` object supports the following
attributes:

- `MaxParallel` - `MaxParallel` is given as an integer value and specifies
the number of tasks that can be updated at the same time.

- `HealthCheck` - Specifies the mechanism in which allocations health is
determined. The potential values are:

  - "checks" - Specifies that the allocation should be considered healthy when
    all of its tasks are running and their associated [checks][] are healthy,
    and unhealthy if any of the tasks fail or not all checks become healthy.
    This is a superset of "task_states" mode.

  - "task_states" - Specifies that the allocation should be considered healthy
    when all its tasks are running and unhealthy if tasks fail.

  - "manual" - Specifies that Nomad should not automatically determine health
    and that the operator will specify allocation health using the [HTTP
    API](/api/deployments.html#set-allocation-health-in-deployment).

- `MinHealthyTime` - Specifies the minimum time the allocation must be in the
  healthy state before it is marked as healthy and unblocks further allocations
  from being updated.

- `HealthyDeadline` - Specifies the deadline in which the allocation must be
  marked as healthy after which the allocation is automatically transitioned to
  unhealthy.

- `ProgressDeadline` - Specifies the deadline in which an allocation must be
  marked as healthy. The deadline begins when the first allocation for the
  deployment is created and is reset whenever an allocation as part of the
  deployment transitions to a healthy state. If no allocation transitions to the
  healthy state before the progress deadline, the deployment is marked as
  failed. If the `progress_deadline` is set to `0`, the first allocation to be
  marked as unhealthy causes the deployment to fail.

- `AutoRevert` - Specifies if the job should auto-revert to the last stable job
  on deployment failure. A job is marked as stable if all the allocations as
  part of its deployment were marked healthy.

- `Canary` - Specifies that changes to the job that would result in destructive
  updates should create the specified number of canaries without stopping any
  previous allocations. Once the operator determines the canaries are healthy,
  they can be promoted which unblocks a rolling update of the remaining
  allocations at a rate of `max_parallel`.

- `Stagger` - Specifies the delay between migrating allocations off nodes marked
  for draining.

An example `Update` block:

```json
{
  "Update": {
        "MaxParallel": 3,
        "HealthCheck": "checks",
        "MinHealthyTime": 15000000000,
        "HealthyDeadline": 180000000000,
        "AutoRevert": false,
        "Canary": 1
  }
}
```

### Constraint

The `Constraint` object supports the following keys:

- `LTarget` - Specifies the attribute to examine for the
  constraint. See the table of attributes [here](/docs/runtime/interpolation.html#interpreted_node_vars).

- `RTarget` - Specifies the value to compare the attribute against.
  This can be a literal value, another attribute or a regular expression if
  the `Operator` is in "regexp" mode.

- `Operand` - Specifies the test to be performed on the two targets. It takes on the
  following values:

  - `regexp` - Allows the `RTarget` to be a regular expression to be matched.

  - `set_contains` - Allows the `RTarget` to be a comma separated list of values
    that should be contained in the LTarget's value.

  - `distinct_hosts` - If set, the scheduler will not co-locate any task groups on the same
        machine. This can be specified as a job constraint which applies the
        constraint to all task groups in the job, or as a task group constraint which
        scopes the effect to just that group. The constraint may not be
        specified at the task level.

        Placing the constraint at both the job level and at the task group level is
        redundant since when placed at the job level, the constraint will be applied
        to all task groups. When specified, `LTarget` and `RTarget` should be
        omitted.

  - `distinct_property` - If set, the scheduler selects nodes that have a
        distinct value of the specified property. The `RTarget` specifies how
        many allocations are allowed to share the value of a property. The
        `RTarget` must be 1 or greater and if omitted, defaults to 1. This can
        be specified as a job constraint which applies the constraint to all
        task groups in the job, or as a task group constraint which scopes the
        effect to just that group. The constraint may not be specified at the
        task level.

        Placing the constraint at both the job level and at the task group level is
        redundant since when placed at the job level, the constraint will be applied
        to all task groups. When specified, `LTarget` should be the property
        that should be distinct and `RTarget` should be omitted.

  - Comparison Operators - `=`, `==`, `is`, `!=`, `not`, `>`, `>=`, `<`, `<=`. The
    ordering is compared lexically.

### Log Rotation

The `LogConfig` object configures the log rotation policy for a task's `stdout` and
`stderr`. The `LogConfig` object supports the following attributes:

- `MaxFiles` - The maximum number of rotated files Nomad will retain for
  `stdout` and `stderr`, each tracked individually.

- `MaxFileSizeMB` - The size of each rotated file. The size is specified in
  `MB`.

If the amount of disk resource requested for the task is less than the total
amount of disk space needed to retain the rotated set of files, Nomad will return
a validation error when a job is submitted.

```json
{
  "LogConfig": {
    "MaxFiles": 3,
    "MaxFileSizeMB": 10
  }
}
```

In the above example we have asked Nomad to retain 3 rotated files for both
`stderr` and `stdout` and size of each file is 10 MB. The minimum disk space that
would be required for the task would be 60 MB.

### Artifact

Nomad downloads artifacts using
[`go-getter`](https://github.com/hashicorp/go-getter). The `go-getter` library
allows downloading of artifacts from various sources using a URL as the input
source. The key-value pairs given in the `options` block map directly to
parameters appended to the supplied `source` URL. These are then used by
`go-getter` to appropriately download the artifact. `go-getter` also has a CLI
tool to validate its URL and can be used to check if the Nomad `artifact` is
valid.

Nomad allows downloading `http`, `https`, and `S3` artifacts. If these artifacts
are archives (zip, tar.gz, bz2, etc.), these will be unarchived before the task
is started.

The `Artifact` object supports the following keys:

- `GetterSource` - The path to the artifact to download.

- `RelativeDest` - An optional path to download the artifact into relative to the
  root of the task's directory. If omitted, it will default to `local/`.

- `GetterOptions` - A `map[string]string` block of options for `go-getter`.
  Full documentation of supported options are available
  [here](https://github.com/hashicorp/go-getter/tree/ef5edd3d8f6f482b775199be2f3734fd20e04d4a#protocol-specific-options-1).
  An example is given below:

```json
{
  "GetterOptions": {
    "checksum": "md5:c4aa853ad2215426eb7d70a21922e794",

    "aws_access_key_id": "<id>",
    "aws_access_key_secret": "<secret>",
    "aws_access_token": "<token>"
  }
}
```

An example of downloading and unzipping an archive is as simple as:

```json
{
  "Artifacts": [
    {
      "GetterSource": "https://example.com/my.zip",
      "GetterOptions": {
        "checksum": "md5:7f4b3e3b4dd5150d4e5aaaa5efada4c3"
      }
    }
  ]
}
```

#### S3 examples

S3 has several different types of addressing and more detail can be found
[here](http://docs.aws.amazon.com/AmazonS3/latest/dev/UsingBucket.html#access-bucket-intro)

S3 region specific endpoints can be found
[here](http://docs.aws.amazon.com/general/latest/gr/rande.html#s3_region)

Path based style:

```json
{
  "Artifacts": [
    {
      "GetterSource": "https://s3-us-west-2.amazonaws.com/my-bucket-example/my_app.tar.gz",
    }
  ]
}
```

or to override automatic detection in the URL, use the S3-specific syntax

```json
{
  "Artifacts": [
    {
      "GetterSource": "s3::https://s3-eu-west-1.amazonaws.com/my-bucket-example/my_app.tar.gz",
    }
  ]
}
```

Virtual hosted based style

```json
{
  "Artifacts": [
    {
      "GetterSource": "my-bucket-example.s3-eu-west-1.amazonaws.com/my_app.tar.gz",
    }
  ]
}
```

### Template

The `Template` block instantiates an instance of a template renderer. This
creates a convenient way to ship configuration files that are populated from
environment variables, Consul data, Vault secrets, or just general
configurations within a Nomad task.

Nomad utilizes a tool called [Consul Template][ct]. Since Nomad v0.5.3, the
template can reference [Nomad's runtime environment variables][env]. For a full
list of the API template functions, please refer to the [Consul Template
README][ct].


`Template` object supports following attributes:

- `ChangeMode` - Specifies the behavior Nomad should take if the rendered
  template changes. The default value is `"restart"`. The possible values are:

  - `"noop"` - take no action (continue running the task)
  - `"restart"` - restart the task
  - `"signal"` - send a configurable signal to the task

- `ChangeSignal` - Specifies the signal to send to the task as a string like
  "SIGUSR1" or "SIGINT". This option is required if the `ChangeMode` is
  `signal`.

- `DestPath` - Specifies the location where the resulting template should be
  rendered, relative to the task directory.

- `EmbeddedTmpl` -  Specifies the raw template to execute. One of `SourcePath`
  or `EmbeddedTmpl` must be specified, but not both. This is useful for smaller
  templates, but we recommend using `SourcePath` for larger templates.

- `Envvars` - Specifies the template should be read back as environment
  variables for the task.

- `LeftDelim` - Specifies the left delimiter to use in the template. The default
  is "{{" for some templates, it may be easier to use a different delimiter that
  does not conflict with the output file itself.

- `Perms` - Specifies the rendered template's permissions. File permissions are
  given as octal of the Unix file permissions `rwxrwxrwx`.

- `RightDelim` - Specifies the right delimiter to use in the template. The default
  is "}}" for some templates, it may be easier to use a different delimiter that
  does not conflict with the output file itself.

- `SourcePath` - Specifies the path to the template to be rendered. `SourcePath`
  is mutually exclusive with `EmbeddedTmpl` attribute. The source can be fetched
  using an [`Artifact`](#artifact) resource. The template must exist on the
  machine prior to starting the task; it is not possible to reference a template
  inside of a Docker container, for example.

- `Splay` - Specifies a random amount of time to wait between 0 ms and the given
  splay value before invoking the change mode. Should be specified in
  nanoseconds.

- `VaultGrace` - Specifies the grace period between lease renewal and secret
  re-acquisition. When renewing a secret, if the remaining lease is less than or
  equal to the configured grace, the template will request a new credential.
  This prevents Vault from revoking the secret at its expiration and the task
  having a stale secret. If the grace is set to a value that is higher than your
  default TTL or max TTL, the template will always read a new secret. If the
  task defines several templates, the `vault_grace` will be set to the lowest
  value across all the templates.

```json
{
  "Templates": [
    {
      "SourcePath": "local/config.conf.tpl",
      "DestPath": "local/config.conf",
      "EmbeddedTmpl": "",
      "ChangeMode": "signal",
      "ChangeSignal": "SIGUSR1",
      "Splay": 5000000000
    }
  ]
}
```

[ct]: https://github.com/hashicorp/consul-template "Consul Template by HashiCorp"
[drain]: /docs/commands/node/drain.html
[env]: /docs/runtime/environment.html "Nomad Runtime Environment"
