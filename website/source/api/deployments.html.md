---
layout: api
page_title: Deployments - HTTP API
sidebar_current: api-deployments
description: |-
  The /deployment are used to query for and interact with deployments.
---

# Deployments HTTP API

The `/deployment` endpoints are used to query for and interact with deployments.

## List Deployments

This endpoint lists all deployments.

| Method | Path                     | Produces                   |
| ------ | ------------------------ | -------------------------- |
| `GET`  | `/v1/deployments`        | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `YES`            | `none`       |

### Parameters

- `prefix` `(string: "")`- Specifies a string to filter deployments on based on
  an index prefix. This is specified as a querystring parameter.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/deployments
```

```text
$ curl \
    https://nomad.rocks/v1/deployments?prefix=25ba81c
```

### Sample Response

```json
[
  {
    "ID": "70638f62-5c19-193e-30d6-f9d6e689ab8e",
    "JobID": "example",
    "JobVersion": 1,
    "JobModifyIndex": 17,
    "JobCreateIndex": 7,
    "TaskGroups": {
      "cache": {
        "Promoted": false,
        "DesiredCanaries": 1,
        "DesiredTotal": 3,
        "PlacedAllocs": 1,
        "HealthyAllocs": 0,
        "UnhealthyAllocs": 0
      }
    },
    "Status": "running",
    "StatusDescription": "",
    "CreateIndex": 19,
    "ModifyIndex": 19
  }
]
```

## Read Deployment

This endpoint reads information about a specific deployment by ID.

| Method | Path                             | Produces                   |
| ------ | -------------------------------- | -------------------------- |
| `GET`  | `/v1/deployment/:deployment_id`  | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `YES`            | `none`       |

### Parameters

- `:deployment_id` `(string: <required>)`- Specifies the UUID of the deployment.
  This must be the full UUID, not the short 8-character one. This is specified
  as part of the path.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/deployment/70638f62-5c19-193e-30d6-f9d6e689ab8e
```

### Sample Response

```json
{
  "ID": "70638f62-5c19-193e-30d6-f9d6e689ab8e",
  "JobID": "example",
  "JobVersion": 1,
  "JobModifyIndex": 17,
  "JobCreateIndex": 7,
  "TaskGroups": {
    "cache": {
      "Promoted": false,
      "DesiredCanaries": 1,
      "DesiredTotal": 3,
      "PlacedAllocs": 1,
      "HealthyAllocs": 0,
      "UnhealthyAllocs": 0
    }
  },
  "Status": "running",
  "StatusDescription": "",
  "CreateIndex": 19,
  "ModifyIndex": 19
}
```

## List Allocations for Deployment

This endpoint lists the allocations created or modified for the given
deployment.

| Method | Path                                        | Produces                   |
| ------ | ------------------------------------------- | -------------------------- |
| `GET`  | `/v1/deployment/allocations/:deployment_id` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `YES`            | `none`       |

### Parameters

- `:deployment_id` `(string: <required>)`- Specifies the UUID of the deployment.
  This must be the full UUID, not the short 8-character one. This is specified
  as part of the path.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/deployment/allocations/5456bd7a-9fc0-c0dd-6131-cbee77f57577
```

### Sample Response

```json
[
  {
    "ID": "287b65cc-6c25-cea9-0332-e4a75ca2af98",
    "EvalID": "9751cb74-1a0d-190e-d026-ad2bc666ad2c",
    "Name": "example.cache[0]",
    "NodeID": "cb1f6030-a220-4f92-57dc-7baaabdc3823",
    "JobID": "example",
    "TaskGroup": "cache",
    "DesiredStatus": "run",
    "DesiredDescription": "",
    "ClientStatus": "running",
    "ClientDescription": "",
    "TaskStates": {
      "redis": {
        "State": "running",
        "Failed": false,
        "StartedAt": "2017-06-29T22:29:41.52000268Z",
        "FinishedAt": "0001-01-01T00:00:00Z",
        "Events": [
          {
            "Type": "Received",
            "Time": 1498775380693307400,
            "FailsTask": false,
            "RestartReason": "",
            "SetupError": "",
            "DriverError": "",
            "ExitCode": 0,
            "Signal": 0,
            "Message": "",
            "KillTimeout": 0,
            "KillError": "",
            "KillReason": "",
            "StartDelay": 0,
            "DownloadError": "",
            "ValidationError": "",
            "DiskLimit": 0,
            "FailedSibling": "",
            "VaultError": "",
            "TaskSignalReason": "",
            "TaskSignal": "",
            "DriverMessage": ""
          },
          {
            "Type": "Task Setup",
            "Time": 1498775380693659000,
            "FailsTask": false,
            "RestartReason": "",
            "SetupError": "",
            "DriverError": "",
            "ExitCode": 0,
            "Signal": 0,
            "Message": "Building Task Directory",
            "KillTimeout": 0,
            "KillError": "",
            "KillReason": "",
            "StartDelay": 0,
            "DownloadError": "",
            "ValidationError": "",
            "DiskLimit": 0,
            "FailedSibling": "",
            "VaultError": "",
            "TaskSignalReason": "",
            "TaskSignal": "",
            "DriverMessage": ""
          },
          {
            "Type": "Started",
            "Time": 1498775381508493800,
            "FailsTask": false,
            "RestartReason": "",
            "SetupError": "",
            "DriverError": "",
            "ExitCode": 0,
            "Signal": 0,
            "Message": "",
            "KillTimeout": 0,
            "KillError": "",
            "KillReason": "",
            "StartDelay": 0,
            "DownloadError": "",
            "ValidationError": "",
            "DiskLimit": 0,
            "FailedSibling": "",
            "VaultError": "",
            "TaskSignalReason": "",
            "TaskSignal": "",
            "DriverMessage": ""
          }
        ]
      }
    },
    "DeploymentStatus": null,
    "CreateIndex": 19,
    "ModifyIndex": 22,
    "CreateTime": 1498775380678486300
  }
]
```

## Fail Deployment

This endpoint is used to mark a deployment as failed. This should be done to
force the scheduler to stop creating allocations as part of the deployment or to
cause a rollback to a previous job version.

| Method  | Path                                 | Produces                   |
| ------- | ------------------------------------ | -------------------------- |
| `POST`  | `/v1/deployment/fail/:deployment_id` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

- `:deployment_id` `(string: <required>)`- Specifies the UUID of the deployment.
  This must be the full UUID, not the short 8-character one. This is specified
  as part of the path.

### Sample Request

```text
$ curl \
    --request POST \
    https://nomad.rocks/v1/deployment/fail/5456bd7a-9fc0-c0dd-6131-cbee77f57577
```

### Sample Response

```json
{
  "EvalID": "0d834913-58a0-81ac-6e33-e452d83a0c66",
  "EvalCreateIndex": 20,
  "DeploymentModifyIndex": 20,
  "RevertedJobVersion": 1,
  "Index": 20
}
```

## Pause Deployment

This endpoint is used to pause or unpause a deployment. This is done to pause
a rolling upgrade or resume it.

| Method  | Path                                  | Produces                   |
| ------- | ------------------------------------- | -------------------------- |
| `POST`  | `/v1/deployment/pause/:deployment_id` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

- `:deployment_id` `(string: <required>)`- Specifies the UUID of the deployment.
  This must be the full UUID, not the short 8-character one. This is specified
  as part of the path and in the JSON payload.

- `Pause` `(bool: false)` - Specifies whether to pause or resume the deployment.

### Sample Payload

```javascript
{
  "DeploymentID": "5456bd7a-9fc0-c0dd-6131-cbee77f57577",
  "Pause": true
}
```      

### Sample Request

```text
$ curl \
    --request POST \
    https://nomad.rocks/v1/deployment/pause/5456bd7a-9fc0-c0dd-6131-cbee77f57577
```

### Sample Response

```json
{
  "EvalID": "0d834913-58a0-81ac-6e33-e452d83a0c66",
  "EvalCreateIndex": 20,
  "DeploymentModifyIndex": 20,
  "Index": 20
}
```

## Promote Deployment

This endpoint is used to promote task groups that have canaries for a
deployment. This should be done when the placed canaries are healthy and the
rolling upgrade of the remaining allocations should begin.

| Method  | Path                                    | Produces                   |
| ------- | -------------------------------------   | -------------------------- |
| `POST`  | `/v1/deployment/promote/:deployment_id` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

- `:deployment_id` `(string: <required>)`- Specifies the UUID of the deployment.
  This must be the full UUID, not the short 8-character one. This is specified
  as part of the path and JSON payload.

- `All` `(bool: false)` - Specifies whether all task groups should be promoted.

- `Groups` `(array<string>: nil)` - Specifies a particular set of task groups
  that should be promoted.

### Sample Payload

```javascript
{
  "DeploymentID": "5456bd7a-9fc0-c0dd-6131-cbee77f57577",
  "All": true
}
```      

```javascript
{
  "DeploymentID": "5456bd7a-9fc0-c0dd-6131-cbee77f57577",
  "Groups": ["web", "api-server"]
}
```      

### Sample Request

```text
$ curl \
    --request POST \
    https://nomad.rocks/v1/deployment/promote/5456bd7a-9fc0-c0dd-6131-cbee77f57577
```

### Sample Response

```json
{
  "EvalID": "0d834913-58a0-81ac-6e33-e452d83a0c66",
  "EvalCreateIndex": 20,
  "DeploymentModifyIndex": 20,
  "Index": 20
}
```

## Set Allocation Health in Deployment

This endpoint is used to set the health of an allocation that is in the
deployment manually. In some use cases, automatic detection of allocation health
may not be desired. As such those task groups can be marked with an upgrade
policy that uses `health_check = "manual"`. Those allocations must have their
health marked manually using this endpoint. Marking an allocation as healthy
will allow the rolling upgrade to proceed. Marking it as failed will cause the
deployment to fail.

| Method  | Path                                              | Produces                   |
| ------- | ------------------------------------------------- | -------------------------- |
| `POST`  | `/v1/deployment/allocation-health/:deployment_id` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

- `:deployment_id` `(string: <required>)`- Specifies the UUID of the deployment.
  This must be the full UUID, not the short 8-character one. This is specified
  as part of the path and the JSON payload.

- `HealthyAllocationIDs` `(array<string>: nil)` - Specifies the set of
  allocation that should be marked as healthy.

- `UnhealthyAllocationIDs` `(array<string>: nil)` - Specifies the set of
  allocation that should be marked as unhealthy.

### Sample Payload

```javascript
{
  "DeploymentID": "5456bd7a-9fc0-c0dd-6131-cbee77f57577",
  "HealthyAllocationIDs": [
    "eb13bc8a-7300-56f3-14c0-d4ad115ec3f5",
    "6584dad8-7ae3-360f-3069-0b4309711cc1"
  ]
}
```      

### Sample Request

```text
$ curl \
    --request POST \
    https://nomad.rocks/v1/deployment/allocation-health/5456bd7a-9fc0-c0dd-6131-cbee77f57577
```

### Sample Response

```json
{
  "EvalID": "0d834913-58a0-81ac-6e33-e452d83a0c66",
  "EvalCreateIndex": 20,
  "DeploymentModifyIndex": 20,
  "RevertedJobVersion": 1,
  "Index": 20
}
```
