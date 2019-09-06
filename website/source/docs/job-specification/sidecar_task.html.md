---
layout: "docs"
page_title: "sidecar_service Stanza - Job Specification"
sidebar_current: "docs-job-specification-sidecar-task"
description: |-
  The "sidecar_task" stanza allows specifying options for configuring
  the task of the sidecar proxies used in Consul Connect integration
---

# `sidecar_task` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>job -> group -> service -> connect -> **sidecar_task** </code>
    </td>
  </tr>
</table>

The `sidecar_task` stanza allows configuring various options for
the sidecar proxy managed by Nomad for Consul Connect integration.
It is valid only within the context of a connect stanza.

```hcl
 job "countdash" {
   datacenters = ["dc1"]
   group "api" {
     network {
       mode = "bridge"
     }

     service {
       name = "count-api"
       port = "9001"

       connect {
         sidecar_service {}
         sidecar_task {
            resources {
               cpu = 500
               memory = 1024
            }
         }
       }
     }
     task "web" {
         driver = "docker"
         config {
           image = "test/test:v1"
         }
     }
   }
 }

```

## `sidecar_task` Parameters
- `name` `(string: )` - Name of the task.

- `driver` `(string: )` - Driver used for the sidecar task.

- `user` `(string:nil)` - Determines which user is used to run the task, defaults
   to the same user the Nomad client is being run as.

- `config` `(map:nil )` - Configuration provided to the driver for initialization.

- `env` `(map:nil )` - Map of environment variables used by the driver.

- `resources` <code>[resources][]</code> - Resources needed by this task.

- `meta` `(map:nil )` - Arbitrary metadata associated with this task that's opaque to Nomad.

- `logs` <code>([Logs][]: nil)</code> - Specifies logging configuration for the
  `stdout` and `stderr` of the task.

- `kill_timeout` `(int:)` - Time between signalling a task that will be killed and killing it.

- `shutdown_delay` `(int:)` - Delay between deregistering the task from Consul and sendint it a
  signal to shutdown.

- `kill_signal` `(string:SIGINT)` - Kill signal to use for the task, defaults to SIGINT.


## `sidecar_task` Examples
The following example configures resources for the sidecar task and other configuration.

```hcl
   sidecar_task {
     resources {
       cpu = 500
       memory = 1024
     }

     env {
       FOO = "abc"
     }

     shutdown_delay = "5s"
   }

 ```

[job]: /docs/job-specification/job.html "Nomad job Job Specification"
[group]: /docs/job-specification/group.html "Nomad group Job Specification"
[task]: /docs/job-specification/task.html "Nomad task Job Specification"
[interpolation]: /docs/runtime/interpolation.html "Nomad interpolation"
[sidecar_service]: /docs/job-specification/sidecar_service.html "Nomad sidecar service Specification"
[resources]: /docs/job-specification/resources.html "Nomad resources Job Specification"
[logs]: /docs/job-specification/logs.html "Nomad logs Job Specification"