---
layout: api
page_title: Evaluations - HTTP API
sidebar_current: api-evaluations
description: |-
  The /evaluation are used to query for and interact with evaluations.
---

# Evaluations HTTP API

The `/evaluation` endpoints are used to query for and interact with evaluations.

## List Evaluations

This endpoint lists all evaluations.

| Method | Path                     | Produces                   |
| ------ | ------------------------ | -------------------------- |
| `GET`  | `/v1/evaluations`        | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `YES`            | `none`       |

### Parameters

- `prefix` `(string: "")`- Specifies a string to filter evaluations on based on
  an index prefix. This is specified as a querystring parameter.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/evaluations
```

```text
$ curl \
    https://nomad.rocks/v1/evaluations?prefix=25ba81c
```

### Sample Response

```json
[
  {
    "ID": "5456bd7a-9fc0-c0dd-6131-cbee77f57577",
    "Priority": 50,
    "Type": "service",
    "TriggeredBy": "job-register",
    "JobID": "example",
    "JobModifyIndex": 52,
    "NodeID": "",
    "NodeModifyIndex": 0,
    "Status": "complete",
    "StatusDescription": "",
    "Wait": 0,
    "NextEval": "",
    "PreviousEval": "",
    "BlockedEval": "",
    "FailedTGAllocs": null,
    "ClassEligibility": null,
    "EscapedComputedClass": false,
    "AnnotatePlan": false,
    "SnapshotIndex": 53,
    "QueuedAllocations": {
      "cache": 0
    },
    "CreateIndex": 53,
    "ModifyIndex": 55
  }
]
```

## Read Evaluation

This endpoint reads information about a specific evaluation by ID.

| Method | Path                      | Produces                   |
| ------ | ------------------------- | -------------------------- |
| `GET`  | `/v1/evaluation/:eval_id` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `YES`            | `none`       |

### Parameters

- `:eval_id` `(string: <required>)`- Specifies the UUID of the evaluation. This
  must be the full UUID, not the short 8-character one. This is specified as
  part of the path.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/evaluation/5456bd7a-9fc0-c0dd-6131-cbee77f57577
```

### Sample Response

```json
{
  "ID": "5456bd7a-9fc0-c0dd-6131-cbee77f57577",
  "Priority": 50,
  "Type": "service",
  "TriggeredBy": "job-register",
  "JobID": "example",
  "JobModifyIndex": 52,
  "NodeID": "",
  "NodeModifyIndex": 0,
  "Status": "complete",
  "StatusDescription": "",
  "Wait": 0,
  "NextEval": "",
  "PreviousEval": "",
  "BlockedEval": "",
  "FailedTGAllocs": null,
  "ClassEligibility": null,
  "EscapedComputedClass": false,
  "AnnotatePlan": false,
  "SnapshotIndex": 53,
  "QueuedAllocations": {
    "cache": 0
  },
  "CreateIndex": 53,
  "ModifyIndex": 55
}
```

## List Allocations for Evaluation

This endpoint lists the allocations created or modified for the given
evaluation.

| Method | Path                                  | Produces                   |
| ------ | ------------------------------------- | -------------------------- |
| `GET`  | `/v1/evaluation/:eval_id/allocations` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `YES`            | `none`       |

### Parameters

- `:eval_id` `(string: <required>)`- Specifies the UUID of the evaluation. This
  must be the full UUID, not the short 8-character one. This is specified as
  part of the path.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/evaluation/5456bd7a-9fc0-c0dd-6131-cbee77f57577/allocations
```

### Sample Response

```json
[
  {
    "ID": "a8198d79-cfdb-6593-a999-1e9adabcba2e",
    "EvalID": "5456bd7a-9fc0-c0dd-6131-cbee77f57577",
    "Name": "example.cache[0]",
    "NodeID": "fb2170a8-257d-3c64-b14d-bc06cc94e34c",
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
        "Events": [
          {
            "Type": "Received",
            "Time": 1495747371795703800,
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
            "Type": "Driver",
            "Time": 1495747371798867200,
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
            "DriverMessage": "Downloading image redis:3.2"
          },
          {
            "Type": "Started",
            "Time": 1495747379525667800,
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
    "CreateIndex": 54,
    "ModifyIndex": 57,
    "CreateTime": 1495747371794276400
  }
]
```
