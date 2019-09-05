---
layout: "docs"
page_title: "upstreams Stanza - Job Specification"
sidebar_current: "docs-job-specification-upstreams"
description: |-
  The "upstreams" stanza allows specifying options for configuring
  upstream services
---

# `upstreams` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>job -> group -> service -> connect -> sidecar_service -> proxy -> **upstreams** </code>
    </td>
  </tr>
</table>

The `upstreams` stanza allows configuring various options for
managing upstream services that the proxy routes to.
It is valid only within the context of a `proxy` stanza.

```hcl
job "countdash" {
  datacenters = ["dc1"]

  group "dashboard" {
    network {
      mode = "bridge"

      port "http" {
        static = 9002
        to     = 9002
      }
    }

    service {
      name = "count-dashboard"
      port = "9002"

      connect {
        sidecar_service {
          proxy {
            upstreams {
              destination_name = "count-api"
              local_bind_port  = 8080
            }
          }
        }
      }
    }

    task "dashboard" {
      driver = "docker"

      env {
        COUNTING_SERVICE_URL = "http://${NOMAD_UPSTREAM_ADDR_count_api}"
      }

      config {
        image = "hashicorpnomad/counter-dashboard:v1"
      }
    }
  }
}

```

## `upstreams` Parameters

- `destination_name` `(string: nil)` - Name of the upstream service.
- `local_bind_port` - (int: nil)</code> - The port the proxy will receive
  connections for the upstream on.


## `upstreams` Examples

The following example is an upstream config with the name of the destination service
and a local bind port.

```hcl
    upstreams {
      destination_name = "count-api"
      local_bind_port = 8080
    }
 ```

[job]: /docs/job-specification/job.html "Nomad job Job Specification"
[group]: /docs/job-specification/group.html "Nomad group Job Specification"
[task]: /docs/job-specification/task.html "Nomad task Job Specification"
[interpolation]: /docs/runtime/interpolation.html "Nomad interpolation"
[sidecar_service]: /docs/job-specification/sidecar_service.html "Nomad sidecar service Specification"
[upstreams]: /docs/job-specification/upstreams.html "Nomad upstream config Specification"