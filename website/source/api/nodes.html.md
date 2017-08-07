---
layout: api
page_title: Nodes - HTTP API
sidebar_current: api-nodes
description: |-
  The /node endpoints are used to query for and interact with client nodes.
---

# Nodes HTTP API

The `/node` endpoints are used to query for and interact with client nodes.

### List Nodes

This endpoint lists all nodes registered with Nomad.

| Method | Path                      | Produces                   |
| ------ | ------------------------- | -------------------------- |
| `GET`  | `/v1/nodes`               | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `YES`            | `none`       |

### Parameters

- `prefix` `(string: "")`- Specifies a string to filter nodes on based on an
  index prefix. This is specified as a querystring parameter.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/nodes
```

```text
$ curl \
    https://nomad.rocks/v1/nodes?prefix=prod
```

### Sample Response

```json
[
  {
    "ID": "fb2170a8-257d-3c64-b14d-bc06cc94e34c",
    "Datacenter": "dc1",
    "Name": "bacon-mac",
    "NodeClass": "",
    "Drain": false,
    "Status": "ready",
    "StatusDescription": "",
    "CreateIndex": 5,
    "ModifyIndex": 45
  }
]
```

## Read Node

This endpoint queries the status of a client node.

| Method | Path                      | Produces                   |
| ------ | ------------------------- | -------------------------- |
| `GET`  | `/v1/node/:node_id`       | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `YES`            | `none`       |

### Parameters

- `:node_id` `(string: <required>)`- Specifies the ID of the node. This must be
  the full UUID, not the short 8-character one. This is specified as part of the
  path.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/node/fb2170a8-257d-3c64-b14d-bc06cc94e34c
```

### Sample Response

```json
{
  "ID": "fb2170a8-257d-3c64-b14d-bc06cc94e34c",
  "SecretID": "",
  "Datacenter": "dc1",
  "Name": "bacon-mac",
  "HTTPAddr": "127.0.0.1:4646",
  "TLSEnabled": false,
  "Attributes": {
    "os.version": "10.12.5",
    "cpu.modelname": "Intel(R) Core(TM) i7-3615QM CPU @ 2.30GHz",
    "nomad.revision": "f551dcb83e3ac144c9dbb90583b6e82d234662e9",
    "driver.docker.volumes.enabled": "1",
    "driver.docker": "1",
    "cpu.frequency": "2300",
    "memory.totalbytes": "17179869184",
    "driver.mock_driver": "1",
    "kernel.version": "16.6.0",
    "unique.network.ip-address": "127.0.0.1",
    "nomad.version": "0.5.5dev",
    "unique.hostname": "bacon-mac",
    "cpu.arch": "amd64",
    "os.name": "darwin",
    "kernel.name": "darwin",
    "unique.storage.volume": "/dev/disk1",
    "driver.docker.version": "17.03.1-ce",
    "cpu.totalcompute": "18400",
    "unique.storage.bytestotal": "249783500800",
    "cpu.numcores": "8",
    "os.signals": "SIGCONT,SIGSTOP,SIGSYS,SIGINT,SIGIOT,SIGXCPU,SIGSEGV,SIGUSR1,SIGTTIN,SIGURG,SIGUSR2,SIGABRT,SIGALRM,SIGCHLD,SIGFPE,SIGTSTP,SIGIO,SIGKILL,SIGQUIT,SIGXFSZ,SIGBUS,SIGHUP,SIGPIPE,SIGPROF,SIGTRAP,SIGTTOU,SIGILL,SIGTERM",
    "driver.raw_exec": "1",
    "unique.storage.bytesfree": "142954643456"
  },
  "Resources": {
    "CPU": 18400,
    "MemoryMB": 16384,
    "DiskMB": 136332,
    "IOPS": 0,
    "Networks": [
      {
        "Device": "lo0",
        "CIDR": "127.0.0.1/32",
        "IP": "127.0.0.1",
        "MBits": 1000,
        "ReservedPorts": null,
        "DynamicPorts": null
      }
    ]
  },
  "Reserved": {
    "CPU": 0,
    "MemoryMB": 0,
    "DiskMB": 0,
    "IOPS": 0,
    "Networks": null
  },
  "Links": null,
  "Meta": null,
  "NodeClass": "",
  "ComputedClass": "v1:10952212473894849978",
  "Drain": false,
  "Status": "ready",
  "StatusDescription": "",
  "StatusUpdatedAt": 1495748907,
  "CreateIndex": 5,
  "ModifyIndex": 45
}
```

## List Node Allocations

This endpoint lists all of the allocations for the given node. This can be used to 
determine what allocations have been scheduled on the node, their current status,
and the values of dynamically assigned resources, like ports.

| Method  | Path                            | Produces                   |
| ------- | ------------------------------- | -------------------------- |
| `GET`   | `/v1/node/:node_id/allocations` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `YES`            | `none`       |

### Parameters

- `:node_id` `(string: <required>)`- Specifies the UUID of the node. This must
  be the full UUID, not the short 8-character one. This is specified as part of
  the path.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/node/e02b6169-83bd-9df6-69bd-832765f333eb/allocations
```

### Sample Response

```json
[
  {
    "ID": "8dfa702d-0c03-6fd4-ade6-386d72fb8192",
    "EvalID": "a128568e-6cc6-0f95-f37d-3fd4c8123316",
    "Name": "example.cache[0]",
    "NodeID": "05129072-6258-4ea6-79bf-03bd31418ac7",
    "JobID": "example",
    "Job": {
      "Stop": false,
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
          "Update": {
            "Stagger": 10000000000,
            "MaxParallel": 1,
            "HealthCheck": "checks",
            "MinHealthyTime": 10000000000,
            "HealthyDeadline": 300000000000,
            "AutoRevert": false,
            "Canary": 0
          },
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
                "image": "redis:3.2",
                "port_map": [
                  {
                    "db": 6379
                  }
                ]
              },
              "Env": null,
              "Services": [
                {
                  "Name": "global-redis-check",
                  "PortLabel": "db",
                  "AddressMode": "auto",
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
                      "InitialStatus": "",
                      "TLSSkipVerify": false
                    }
                  ]
                }
              ],
              "Vault": null,
              "Templates": null,
              "Constraints": null,
              "Resources": {
                "CPU": 500,
                "MemoryMB": 256,
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
        "MaxParallel": 1,
        "HealthCheck": "",
        "MinHealthyTime": 0,
        "HealthyDeadline": 0,
        "AutoRevert": false,
        "Canary": 0
      },
      "Periodic": null,
      "ParameterizedJob": null,
      "Payload": null,
      "Meta": null,
      "VaultToken": "",
      "Status": "pending",
      "StatusDescription": "",
      "Stable": false,
      "Version": 0,
      "SubmitTime": 1502140975490599700,
      "CreateIndex": 15050,
      "ModifyIndex": 15050,
      "JobModifyIndex": 15050
    },
    "TaskGroup": "cache",
    "Resources": {
      "CPU": 500,
      "MemoryMB": 256,
      "DiskMB": 300,
      "IOPS": 0,
      "Networks": [
        {
          "Device": "eth0",
          "CIDR": "",
          "IP": "10.0.0.226",
          "MBits": 10,
          "ReservedPorts": null,
          "DynamicPorts": [
            {
              "Label": "db",
              "Value": 22908
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
        "MemoryMB": 256,
        "DiskMB": 0,
        "IOPS": 0,
        "Networks": [
          {
            "Device": "eth0",
            "CIDR": "",
            "IP": "10.0.0.226",
            "MBits": 10,
            "ReservedPorts": null,
            "DynamicPorts": [
              {
                "Label": "db",
                "Value": 22908
              }
            ]
          }
        ]
      }
    },
    "Metrics": {
      "NodesEvaluated": 2,
      "NodesFiltered": 0,
      "NodesAvailable": {
        "dc1": 3
      },
      "ClassFiltered": null,
      "ConstraintFiltered": null,
      "NodesExhausted": 0,
      "ClassExhausted": null,
      "DimensionExhausted": null,
      "Scores": {
        "1dabfc7d-a92f-00f2-1cb6-0be3f000e542.binpack": 8.269190730718089
      },
      "AllocationTime": 41183,
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
        "Restarts": 0,
        "LastRestart": "0001-01-01T00:00:00Z",
        "StartedAt": "2017-08-07T21:22:59.326433825Z",
        "FinishedAt": "0001-01-01T00:00:00Z",
        "Events": [
          {
            "Type": "Received",
            "Time": 1502140975648341200,
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
            "Time": 1502140975648601000,
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
            "Time": 1502140979184330000,
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
    "DeploymentID": "ea696568-4518-0099-6811-9c26425c60af",
    "DeploymentStatus": {
      "Healthy": true,
      "ModifyIndex": 15057
    },
    "CreateIndex": 15052,
    "ModifyIndex": 15057,
    "AllocModifyIndex": 15052,
    "CreateTime": 1502140975600438500
  },
...
]
```
    
## Create Node Evaluation

This endpoint creates a new evaluation for the given node. This can be used to
force a run of the scheduling logic.

| Method  | Path                         | Produces                   |
| ------- | ---------------------------- | -------------------------- |
| `POST`  | `/v1/node/:node_id/evaluate` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

- `:node_id` `(string: <required>)`- Specifies the UUID of the node. This must
  be the full UUID, not the short 8-character one. This is specified as part of
  the path.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/node/fb2170a8-257d-3c64-b14d-bc06cc94e34c/evaluate
```

### Sample Response

```json
{
  "HeartbeatTTL": 0,
  "EvalIDs": [
    "4ff1c7a2-c650-4058-f509-d5028ff9566e"
  ],
  "EvalCreateIndex": 85,
  "NodeModifyIndex": 0,
  "LeaderRPCAddr": "127.0.0.1:4647",
  "NumNodes": 1,
  "Servers": [
    {
      "RPCAdvertiseAddr": "127.0.0.1:4647",
      "RPCMajorVersion": 1,
      "RPCMinorVersion": 1,
      "Datacenter": "dc1"
    }
  ],
  "Index": 85,
  "LastContact": 0,
  "KnownLeader": false
}
```

## Drain Node

This endpoint toggles the drain mode of the node. When draining is enabled, no
further allocations will be assigned to this node, and existing allocations will
be migrated to new nodes.

| Method  | Path                      | Produces                   |
| ------- | ------------------------- | -------------------------- |
| `POST`  | `/v1/node/:node_id/drain` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

- `:node_id` `(string: <required>)`- Specifies the UUID of the node. This must
  be the full UUID, not the short 8-character one. This is specified as part of
  the path.

- `enable` `(bool: <required>)` - Specifies if drain mode should be enabled.
  This is specified as a query string parameter.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/node/fb2170a8-257d-3c64-b14d-bc06cc94e34c/drain?enable=true
```

### Sample Response

```json
{
  "EvalIDs": [
    "253ec083-22a7-76c9-b8b6-2bf3d4b27bfb"
  ],
  "EvalCreateIndex": 91,
  "NodeModifyIndex": 90,
  "Index": 90,
  "LastContact": 0,
  "KnownLeader": false
}
```
