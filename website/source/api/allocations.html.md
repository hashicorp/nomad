---
layout: api
page_title: Allocations - HTTP API
sidebar_current: api-allocations
description: |-
  The /allocation endpoints are used to query for and interact with allocations.
---

# Allocations HTTP API

The `/allocation` endpoints are used to query for and interact with allocations.

## List Allocations

This endpoint lists all allocations.

| Method | Path                      | Produces                   |
| ------ | ------------------------- | -------------------------- |
| `GET`  | `/v1/allocations`         | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `YES`            | `none`       |

### Parameters

- `prefix` `(string: "")`- Specifies a string to filter allocations on based on
  an index prefix. This is specified as a querystring parameter.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/allocations
```

```text
$ curl \
    https://nomad.rocks/v1/allocations?prefix=a8198d79
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
        "FinishedAt": "0001-01-01T00:00:00Z",
        "LastRestart": "0001-01-01T00:00:00Z",
        "Restarts": 0,
        "StartedAt": "2017-07-25T23:36:26.106431265Z",
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

## Read Allocation

This endpoint reads information about a specific allocation.

| Method | Path                       | Produces                   |
| ------ | -------------------------- | -------------------------- |
| `GET`  | `/v1/allocation/:alloc_id` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `YES`            | `none`       |

### Parameters

- `:alloc_id` `(string: <required>)`- Specifies the UUID of the allocation. This
  must be the full UUID, not the short 8-character one. This is specified as
  part of the path.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/allocation/5456bd7a-9fc0-c0dd-6131-cbee77f57577
```

### Sample Response

```json
{
  "ID": "a8198d79-cfdb-6593-a999-1e9adabcba2e",
  "EvalID": "5456bd7a-9fc0-c0dd-6131-cbee77f57577",
  "Name": "example.cache[0]",
  "NodeID": "fb2170a8-257d-3c64-b14d-bc06cc94e34c",
  "JobID": "example",
  "Job": {
    "Region": "global",
    "ID": "example",
    "ParentID": "",
    "Name": "example",
    "Type": "service",
    "Priority": 50,
    "AllAtOnce": false,
    "Datacenters": [
      "dc1"
    ],
    "Constraints": null,
    "TaskGroups": [
      {
        "Name": "cache",
        "Count": 1,
        "Constraints": null,
        "RestartPolicy": {
          "Attempts": 10,
          "Interval": 300000000000,
          "Delay": 25000000000,
          "Mode": "delay"
        },
        "Tasks": [
          {
            "Name": "redis",
            "Driver": "docker",
            "User": "",
            "Config": {
              "port_map": [
                {
                  "db": 6379
                }
              ],
              "image": "redis:3.2"
            },
            "Env": null,
            "Services": [
              {
                "Name": "global-redis-check",
                "PortLabel": "db",
                "Tags": [
                  "global",
                  "cache"
                ],
                "Checks": [
                  {
                    "Name": "alive",
                    "Type": "tcp",
                    "Command": "",
                    "Args": null,
                    "Path": "",
                    "Protocol": "",
                    "PortLabel": "",
                    "Interval": 10000000000,
                    "Timeout": 2000000000,
                    "InitialStatus": ""
                  }
                ]
              }
            ],
            "Vault": null,
            "Templates": null,
            "Constraints": null,
            "Resources": {
              "CPU": 500,
              "MemoryMB": 10,
              "DiskMB": 0,
              "IOPS": 0,
              "Networks": [
                {
                  "Device": "",
                  "CIDR": "",
                  "IP": "",
                  "MBits": 10,
                  "ReservedPorts": null,
                  "DynamicPorts": [
                    {
                      "Label": "db",
                      "Value": 0
                    }
                  ]
                }
              ]
            },
            "DispatchPayload": null,
            "Meta": null,
            "KillTimeout": 5000000000,
            "LogConfig": {
              "MaxFiles": 10,
              "MaxFileSizeMB": 10
            },
            "Artifacts": null,
            "Leader": false
          }
        ],
        "EphemeralDisk": {
          "Sticky": false,
          "SizeMB": 300,
          "Migrate": false
        },
        "Meta": null
      }
    ],
    "Update": {
      "Stagger": 10000000000,
      "MaxParallel": 0
    },
    "Periodic": null,
    "ParameterizedJob": null,
    "Payload": null,
    "Meta": null,
    "VaultToken": "",
    "Status": "pending",
    "StatusDescription": "",
    "CreateIndex": 52,
    "ModifyIndex": 52,
    "JobModifyIndex": 52
  },
  "TaskGroup": "cache",
  "Resources": {
    "CPU": 500,
    "MemoryMB": 10,
    "DiskMB": 300,
    "IOPS": 0,
    "Networks": [
      {
        "Device": "lo0",
        "CIDR": "",
        "IP": "127.0.0.1",
        "MBits": 10,
        "ReservedPorts": null,
        "DynamicPorts": [
          {
            "Label": "db",
            "Value": 23116
          }
        ]
      }
    ]
  },
  "SharedResources": {
    "CPU": 0,
    "MemoryMB": 0,
    "DiskMB": 300,
    "IOPS": 0,
    "Networks": null
  },
  "TaskResources": {
    "redis": {
      "CPU": 500,
      "MemoryMB": 10,
      "DiskMB": 0,
      "IOPS": 0,
      "Networks": [
        {
          "Device": "lo0",
          "CIDR": "",
          "IP": "127.0.0.1",
          "MBits": 10,
          "ReservedPorts": null,
          "DynamicPorts": [
            {
              "Label": "db",
              "Value": 23116
            }
          ]
        }
      ]
    }
  },
  "Metrics": {
    "NodesEvaluated": 1,
    "NodesFiltered": 0,
    "NodesAvailable": {
      "dc1": 1
    },
    "ClassFiltered": null,
    "ConstraintFiltered": null,
    "NodesExhausted": 0,
    "ClassExhausted": null,
    "DimensionExhausted": null,
    "Scores": {
      "fb2170a8-257d-3c64-b14d-bc06cc94e34c.binpack": 0.6205732522109244
    },
    "AllocationTime": 31729,
    "CoalescedFailures": 0
  },
  "DesiredStatus": "run",
  "DesiredDescription": "",
  "ClientStatus": "running",
  "ClientDescription": "",
  "TaskStates": {
    "redis": {
      "State": "running",
      "Failed": false,
      "FinishedAt": "0001-01-01T00:00:00Z",
      "LastRestart": "0001-01-01T00:00:00Z",
      "Restarts": 0,
      "StartedAt": "2017-07-25T23:36:26.106431265Z",
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
  "PreviousAllocation": "",
  "CreateIndex": 54,
  "ModifyIndex": 57,
  "AllocModifyIndex": 54,
  "CreateTime": 1495747371794276400
}
```

#### Field Reference

- `TaskStates` - A map of tasks to their current state and the latest events
  that have effected the state. `TaskState` objects contain the following
  fields:

    - `State`: The task's current state. It can have one of the following
      values:

        - `TaskStatePending` - The task is waiting to be run, either for the first
          time or due to a restart.

        - `TaskStateRunning` - The task is currently running.

        - `TaskStateDead` - The task is dead and will not run again.

    - `StartedAt`: The time the task was last started at. Can be updated through
      restarts.

    - `FinishedAt`: The time the task was finished at.

    - `LastRestart`: The last time the task was restarted.

    - `Restarts`: The number of times the task has restarted.

    - `Events` - An event contains metadata about the event. The latest 10 events
      are stored per task. Each event is timestamped (Unix nanoseconds) and has one
      of the following types:

        - `Setup Failure` - The task could not be started because there was a
        failure setting up the task prior to it running.

        - `Driver Failure` - The task could not be started due to a failure in the
        driver.

        - `Started` - The task was started; either for the first time or due to a
        restart.

        - `Terminated` - The task was started and exited.

        - `Killing` - The task has been sent the kill signal.

        - `Killed` - The task was killed by a user.

        - `Received` - The task has been pulled by the client at the given timestamp.

        - `Failed Validation` - The task was invalid and as such it didn't run.

        - `Restarting` - The task terminated and is being restarted.

        - `Not Restarting` - the task has failed and is not being restarted because
        it has exceeded its restart policy.

        - `Downloading Artifacts` - The task is downloading the artifact(s)
        - specified in the task.

        - `Failed Artifact Download` - Artifact(s) specified in the task failed to
        download.

        - `Restart Signaled` - The task was singled to be restarted.

        - `Signaling` - The task was is being sent a signal.

        - `Sibling Task Failed` - A task in the same task group failed.

        - `Leader Task Dead` - The group's leader task is dead.

        - `Driver` - A message from the driver.

        - `Task Setup` - Task setup messages.

        - `Building Task Directory` - Task is building its file system.

        Depending on the type the event will have applicable annotations.
