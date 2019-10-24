---
layout: "docs"
page_title: "sidecar_service Stanza - Job Specification"
sidebar_current: "docs-job-specification-sidecar-service"
description: |-
  The "sidecar_service" stanza allows specifying options for configuring
  sidecar proxies used in Consul Connect integration
---

# `sidecar_service` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>job -> group -> service -> connect -> **sidecar_service** </code>
    </td>
  </tr>
</table>

The `sidecar_service` stanza allows configuring various options for the sidecar
proxy managed by Nomad for [Consul
Connect](/guides/integrations/consul-connect/index.html) integration.  It is
valid only within the context of a connect stanza.

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

## `sidecar_service` Parameters

- `tags` <code>(array<string>: nil)</code> - Custom Consul service tags for the sidecar service.

- `port` `(string: )` - Port label for sidecar service.

- `proxy` <code>([proxy][]: nil)</code> - This is used to configure the sidecar proxy service.


## `sidecar_service` Examples

The following example is a minimal `sidecar_service` stanza with defaults

```hcl
  connect {
    sidecar_service {}
  }
```

The following example includes specifying upstreams.

```hcl
   sidecar_service {
     proxy {
       upstreams {
         destination_name = "count-api"
         local_bind_port = 8080
       }
     }
   }

 ```

[job]: /docs/job-specification/job.html "Nomad job Job Specification"
[group]: /docs/job-specification/group.html "Nomad group Job Specification"
[task]: /docs/job-specification/task.html "Nomad task Job Specification"
[interpolation]: /docs/runtime/interpolation.html "Nomad interpolation"
[sidecar_service]: /docs/job-specification/sidecar_service.html "Nomad sidecar service Specification"
[proxy]: /docs/job-specification/proxy.html "Nomad sidecar proxy config Specification"
