---
layout: "docs"
page_title: "connect Stanza - Job Specification"
sidebar_current: "docs-job-specification-connect"
description: |-
  The "connect" stanza allows specifying options for Consul Connect integration
---

# `connect` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>job -> group -> service -> **connect**</code>
    </td>
  </tr>
</table>

The `connect` stanza allows configuring various options for
[Consul Connect](/guides/integrations/consul-connect/index.html). It is
valid only within the context of a service definition at the task group
level.

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

## `connect` Parameters

- `sidecar_service` - <code>([sidecar_service][]: nil)</code> - This is used to configure the sidecar
  service injected by Nomad for Consul Connect.

- `sidecar_task` - <code>([sidecar_task][]:nil)</code> - This modifies the configuration of the Envoy
  proxy task.

## `connect` Examples

The following example is a minimal connect stanza with defaults and is
sufficient to start an Envoy proxy sidecar for allowing incoming connections
via Consul Connect.

```hcl
  connect {
    sidecar_service {}
  }
```

The following example includes specifying [`upstreams`][upstreams].

```hcl
  connect {
     sidecar_service {
       proxy {
         upstreams {
           destination_name = "count-api"
           local_bind_port = 8080
         }
       }
     }
  }
 ```

### Limitations

[Consul Connect Native services][native] and [Nomad variable
interpolation][interpolation] are *not* supported in Nomad 0.10.0.

[job]: /docs/job-specification/job.html "Nomad job Job Specification"
[group]: /docs/job-specification/group.html "Nomad group Job Specification"
[task]: /docs/job-specification/task.html "Nomad task Job Specification"
[interpolation]: /docs/runtime/interpolation.html "Nomad interpolation"
[sidecar_service]: /docs/job-specification/sidecar_service.html "Nomad sidecar service Specification"
[sidecar_task]: /docs/job-specification/sidecar_task.html "Nomad sidecar task config Specification"
[upstreams]: /docs/job-specification/upstreams.html "Nomad sidecar service upstreams Specification"
[native]: https://www.consul.io/docs/connect/native.html
