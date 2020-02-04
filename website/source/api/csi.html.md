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
| `NO`             | `namespace:csi-access`      |

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
    "JobID": "example_job_id",
    "Namespace": "example_namespace",
    "PluginType": "controller",
    "ControllerRequired": true,
    "ControllersHealthy": 2,
    "ControllersExpected": 3,
    "NodesHealthy": 14,
    "NodesExpected": 16,
    "CreateIndex": 52,
    "ModifyIndex": 93
  }, {
    "ID": "example",
    "JobID": "example_job_id2",
    "Namespace": "example_namespace",
    "PluginType": "node",
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
    "example-plugin": {
      "ID": "example-plugin",
      "PluginType": "controller",
      "ControllerRequired": true,
      "ControllersHealthy": 2,
      "ControllersExpected": 3,
      "NodesHealthy": 14,
      "NodesExpected": 16,
      "CreateIndex": 52,
      "ModifyIndex": 93
    }
  }
}
```
## List Volumes

This endpoint lists all known volumes in the system.

| Method | Path                      | Produces                   |
| ------ | ------------------------- | -------------------------- |
| `GET`  | `/v1/csi/volumes`         | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required                |
| ---------------- | --------------------------- |
| `NO`             | `namespace:csi-access`      |

### Parameters

`plug_id`: List only volumes for the plugin

### Sample Request

```text
$ curl \
    https://localhost:4646/v1/csi/volumes
```

```text
$ curl \
    https://localhost:4646/v1/csi/volumes?plugin_id=plugin-id1
```

### Sample Response

```json
[
  {
    "ID": "volume-id1",
    "Namespace": "default",
    "Topologies": [
      {
        "foo": "bar"
      },
      {
        "foo": "baz"
      },
    ],
    "AccessMode": "multi-node-single-writer",
    "AttachmentMode": "file-system",
    "Schedulable": true,
    "ReadAllocs": 12,
    "WriteAllocs": 1,
    "PluginID": "plugin-id1",
    "ControllerRequired": true,
    "ControllersHealthy": 3,
    "ControllersExpected": 3,
    "NodesHealthy": 15,
    "NodesExpected": 18,
    "ResourceExhausted": 0,
    "CreateIndex": 42,
    "ModifyIndex": 64,
  }
]
```

## Get Volume Info

This endpoint describes a volume in detail.

| Method | Path                        | Produces                   |
| ------ | -------------------------   | -------------------------- |
| `GET`  | `/v1/csi/volume/volume-id1` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required                |
| ---------------- | --------------------------- |
| `NO`             | `namespace:csi-access`      |

### Parameters

No parameters.

### Sample Request

```text
$ curl \
    https://localhost:4646/v1/csi/volume/volume-id1
```

### Sample Response

```json
[
  {
    "ID": "volume-id1",
    "Name": "volume id1",
    "Namespace": "default",
    "ExternalID": "volume-id1",
    "Topologies": [
      {"foo": "bar"}
    ],
    "AccessMode": "multi-node-single-writer",
    "AttachmentMode": "file-system",
    "Allocations": [
      {
        "ID": "a8198d79-cfdb-6593-a999-1e9adabcba2e",
        "EvalID": "5456bd7a-9fc0-c0dd-6131-cbee77f57577",
        "Name": "example.cache[0]",
        "NodeID": "fb2170a8-257d-3c64-b14d-bc06cc94e34c",
        [...] AllocListStub [...]
      }
    ],
    "Schedulable": true,
    "PluginID": "plugin-id1",
    "ControllerRequired": true,
    "ControllersHealthy": 3,
    "ControllersExpected": 3,
    "NodesHealthy": 15,
    "NodesExpected": 18,
    "ResourceExhausted": 0,
    "CreateIndex": 42,
    "ModifyIndex": 64,
  }
]
```

## Register Volume

This endpoint describes a volume in detail.

| Method | Path                        | Produces                   |
| ------ | -------------------------   | -------------------------- |
| `PUT`  | `/v1/csi/volume/volume-id1` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required                |
| ---------------- | --------------------------- |
| `NO`             | `namespace:csi-create`      |

### Parameters

- `Volume` `(Volume: <required>)` - Specifies the JSON definition of the volume.

### Sample Payload

```json
{
  "ID": "volume-id1",
  "Name": "volume id1",
  "Namespace": "default",
  "ExternalID": "volume-id1",
  "Topologies": [
    {"foo": "bar"}
  ],
  "AccessMode": "multi-node-single-writer",
  "AttachmentMode": "file-system"
  "PluginID": "plugin-id1",
  "ControllerRequired": true,
}
```

### Sample Request

```text
$ curl \
    --request POST \
    --data @payload.json \
    https://localhost:4646/v1/csi/volume/volume-id1
```

### Sample Response

```json
{
  "Index": 1,
  "LastContact": 1,
  "KnownLeader": true
}
```
