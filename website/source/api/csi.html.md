---
layout: api
page_title: CSI - HTTP API
sidebar_current: api-csi
description: |-
  The /csi endpoints are used to query for and interact with CSI plugins and volumes.
---

# CSI HTTP API

The `/csi` endpoints are used to query for and interact with CSI plugins and volumes.

## List Plugins

This endpoint lists all known plugins in the system, where plugins are
configured as [jobs](/api/jobs.html) with tasks containing a
`csi_plugin` stanza.

| Method | Path                      | Produces                   |
| ------ | ------------------------- | -------------------------- |
| `GET`  | `/v1/csi/plugins`         | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required                |
| ---------------- | --------------------------- |
| `NO`             | `namespace:list-jobs`       |

### Parameters

No parameters.

### Sample Request

```text
$ curl \
    https://localhost:4646/v1/csi/plugins
```

### Sample Response

```json
[
  {
    "ID": "example",
    "JobIDs": {
      "example_namespace": [
        "example_job_id",
        "example2_job_id"
      ]
    },
    "ControllerRequired": true,
    "ControllersHealthy": 2,
    "ControllersExpected": 3,
    "NodesHealthy": 14,
    "NodesExpected": 16,
    "CreateIndex": 52,
    "ModifyIndex": 93
  }
]
```

## Get Plugin Info

This endpoint gets detailed information about one plugin

| Method  | Path                       | Produces                   |
| ------- | -------------------------  | -------------------------- |
| `Get`   | `/v1/csi/plugin/plugin_id` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `namespace:submit-job`<br>`namespace:sentinel-override` if `PolicyOverride` set |

### Parameters

No parameters.

### Sample Request

```text
$ curl \
    https://localhost:4646/v1/csi/plugin/plugin_id
```

### Sample Response

```json
[
  {
    "ID": "example_plugin_id",
    "Namespace": "fixme, not used",
    "Topologies": [
      {"key": "val"},
      {"key": "val2"}
    ],
    "Jobs": [
      {
        "ID": "example",
        "ParentID": "",
        "Name": "example",
        "Type": "service",
        "Priority": 50,
        "Status": "pending",
        "StatusDescription": "",
        "JobSummary": {
          "JobID": "example",
          "Summary": {
            "cache": {
              "Queued": 1,
              "Complete": 1,
              "Failed": 0,
              "Running": 0,
              "Starting": 0,
              "Lost": 0
            }
          },
          "Children": {
            "Pending": 0,
            "Running": 0,
            "Dead": 0
          },
          "CreateIndex": 52,
          "ModifyIndex": 96
        },
        "CreateIndex": 52,
        "ModifyIndex": 93,
        "JobModifyIndex": 52
      }
    ],
    "ControllersRequired": true,
    "ControllersHealthy": 1,
    "Controllers": {
      "example_controller_id": {
        "PluginID": "example_plugin_id",
        "Healthy": true,
        "HealthDescription": "healthy",
        "UpdateTime": "2020-01-31T00:00:00.000Z",
        "RequiresControllerPlugin": true,
        "RequiresTopologies": true,
        "ControllerInfo": {
          "SupportsReadOnlyAttach": true,
            "SupportsAttachDetach": true,
            "SupportsListVolumes": true,
            "SupportsListVolumesAttachedNodes": false
        }
      }
    },
    "NodesHealthy": 1,
    "Nodes": {
      "example_node_id": {
        "PluginID": "example_plugin_id",
        "Healthy": true,
        "HealthDescription": "healthy",
        "UpdateTime": "2020-01-30T00:00:00.000Z",
        "RequiresControllerPlugin": true,
        "RequiresTopologies": true,
        "NodeInfo": {
          "ID": "example_node_id",
          "MaxVolumes": 51,
          "AccessibleTopology": {
            "key": "val2"
          },
          "RequiresNodeStageVolume": true
        }
      },
    },
    "CreateIndex": 52,
    "ModifyIndex": 93
  }
]
```


## List Plugin Jobs

This endpoint lists all known plugins in the system, where plugins are
configured as [jobs](/api/jobs.html) with tasks containing a
`csi_plugin` stanza.

| Method | Path                      | Produces                   |
| ------ | ------------------------- | -------------------------- |
| `GET`  | `/v1/csi/jobs`            | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required                |
| ---------------- | --------------------------- |
| `NO`             | `namespace:list-jobs`       |

### Parameters

No parameters.

### Sample Request

```text
$ curl \
    https://localhost:4646/v1/csi/jobs
```

```text
$ curl \
    https://localhost:4646/v1/csi/jobs?plugin_id=example
```

### Sample Response

```json
[
  {
    "ID": "example",
    "JobIDs": {
      "example_namespace": [
        "example_job_id",
        "example2_job_id"
      ]
    },
    "ControllerRequired": true,
    "ControllersHealthy": 2,
    "ControllersExpected": 3,
    "NodesHealthy": 14,
    "NodesExpected": 16,
    "CreateIndex": 52,
    "ModifyIndex": 93
  }
]
```

## Get Plugin Job

This endpoint will parse a HCL jobspec and produce the equivalent JSON encoded
job.

| Method | Path                      | Produces                   |
| ------ | ------------------------- | -------------------------- |
| `GET`  | `/v1/job/:job_id`         | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required                                 |
| ---------------- | ------------                                 |
| `YES`            | `namespace:read-job`, `namespace:csi-access` |

### Parameters

No parameters.

### Sample Request

```text
$ curl \
    https://localhost:4646/v1/job/example_id
```

### Sample Response

```json
{
  "ID": "example_id",
  "Namespace": "default",
  [...] Job Response [...]
  "CSIPlugin": {
    "ID": "example",
    "ControllerRequired": true,
    "ControllersHealthy": 2,
    "ControllersExpected": 3,
    "NodesHealthy": 14,
    "NodesExpected": 16,
    "CreateIndex": 52,
    "ModifyIndex": 93
  }
}
```
