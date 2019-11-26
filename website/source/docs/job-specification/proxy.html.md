---
layout: "docs"
page_title: "proxy Stanza - Job Specification"
sidebar_current: "docs-job-specification-proxy"
description: |-
  The "proxy" stanza allows specifying options for configuring
  sidecar proxies used in Consul Connect integration
---

# `proxy` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>job -> group -> service -> connect -> sidecar_service -> **proxy** </code>
    </td>
  </tr>
</table>

The `proxy` stanza allows configuring various options for the sidecar proxy
managed by Nomad for [Consul
Connect](/guides/integrations/consul-connect/index.html).  It is valid only
within the context of a `sidecar_service` stanza.

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

## `proxy` Parameters

- `local_service_address` `(string: "127.0.0.1")` - The address the local service binds to. Useful to
  customize in clusters with mixed Connect and non-Connect services.
- `local_service_port` <code>(int:[port][])</code> - The port the local service binds to.
   Usually the same as the parent service's port, it is useful to customize in clusters with mixed
   Connect and non-Connect services
- `upstreams` <code>([upstreams][]: nil)</code> - Used to configure details of each upstream service that
  this sidecar proxy communicates with.
- `config` <code>(map: nil)</code> - Proxy configuration that's opaque to Nomad and
  passed directly to Consul. See [Consul Connect's
  documentation](https://www.consul.io/docs/connect/proxies/envoy.html#dynamic-configuration)
  for details.

## `proxy` Examples

The following example is a proxy specification that includes upstreams configuration.

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
[upstreams]: /docs/job-specification/upstreams.html "Nomad upstream config Specification"
[port]: /docs/job-specification/network.html#port-parameters "Nomad network port configuration"
