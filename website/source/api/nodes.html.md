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
| `YES`            | `node:read`  |

### Parameters

- `prefix` `(string: "")`- Specifies a string to filter nodes on based on an
  index prefix. This is specified as a querystring parameter.

### Sample Request

```text
$ curl \
    http://localhost:4646/v1/nodes
```

```text
$ curl \
    http://localhost:4646/v1/nodes?prefix=prod
```

### Sample Response

```json
[
  {
    "Address": "10.138.0.5",
    "CreateIndex": 6,
    "Datacenter": "dc1",
    "Drain": false,
    "Drivers": {
      "java": {
        "Attributes": {
          "driver.java.runtime": "OpenJDK Runtime Environment (build 1.8.0_162-8u162-b12-1~deb9u1-b12)",
          "driver.java.vm": "OpenJDK 64-Bit Server VM (build 25.162-b12, mixed mode)",
          "driver.java.version": "openjdk version \"1.8.0_162"
        },
        "Detected": true,
        "HealthDescription": "",
        "Healthy": true,
        "UpdateTime": "2018-04-11T23:33:48.781948669Z"
      },
      "qemu": {
        "Attributes": null,
        "Detected": false,
        "HealthDescription": "",
        "Healthy": false,
        "UpdateTime": "2018-04-11T23:33:48.7819898Z"
      },
      "rkt": {
        "Attributes": {
          "driver.rkt.appc.version": "0.8.11",
          "driver.rkt.volumes.enabled": "1",
          "driver.rkt.version": "1.29.0"
        },
        "Detected": true,
        "HealthDescription": "Driver rkt is detected: true",
        "Healthy": true,
        "UpdateTime": "2018-04-11T23:34:48.81079772Z"
      },
      "docker": {
        "Attributes": {
          "driver.docker.bridge_ip": "172.17.0.1",
          "driver.docker.version": "18.03.0-ce",
          "driver.docker.volumes.enabled": "1"
        },
        "Detected": true,
        "HealthDescription": "Driver is available and responsive",
        "Healthy": true,
        "UpdateTime": "2018-04-11T23:34:48.713720323Z"
      },
      "exec": {
        "Attributes": {},
        "Detected": true,
        "HealthDescription": "Driver exec is detected: true",
        "Healthy": true,
        "UpdateTime": "2018-04-11T23:34:48.711026521Z"
      },
      "raw_exec": {
        "Attributes": {},
        "Detected": true,
        "HealthDescription": "",
        "Healthy": true,
        "UpdateTime": "2018-04-11T23:33:48.710448534Z"
      }
    },
    "ID": "f7476465-4d6e-c0de-26d0-e383c49be941",
    "ModifyIndex": 2526,
    "Name": "nomad-4",
    "NodeClass": "",
    "SchedulingEligibility": "eligible",
    "Status": "ready",
    "StatusDescription": "",
    "Version": "0.8.0-rc1"
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

| Blocking Queries | ACL Required      |
| ---------------- | ----------------- |
| `YES`            | `node:read`       |

### Parameters

- `:node_id` `(string: <required>)`- Specifies the ID of the node. This must be
  the full UUID, not the short 8-character one. This is specified as part of the
  path.

### Sample Request

```text
$ curl \
    http://localhost:4646/v1/node/f7476465-4d6e-c0de-26d0-e383c49be941
```

### Sample Response

```json
{
  "Attributes": {
    "driver.rkt": "1",
    "driver.docker.bridge_ip": "172.17.0.1",
    "unique.storage.volume": "/dev/sda1",
    "driver.exec": "1",
    "driver.rkt.volumes.enabled": "1",
    "os.signals": "SIGSTOP,SIGTTIN,SIGWINCH,SIGXCPU,SIGXFSZ,SIGIO,SIGKILL,SIGTTOU,SIGINT,SIGHUP,SIGTRAP,SIGALRM,SIGPIPE,SIGURG,SIGABRT,SIGSEGV,SIGIOT,SIGTERM,SIGBUS,SIGPROF,SIGQUIT,SIGTSTP,SIGUSR2,SIGFPE,SIGCONT,SIGILL,SIGSYS,SIGUSR1,SIGCHLD",
    "cpu.totalcompute": "2200",
    "driver.raw_exec": "1",
    "driver.java.version": "openjdk version \"1.8.0_162",
    "kernel.name": "linux",
    "unique.cgroup.mountpoint": "/sys/fs/cgroup",
    "driver.docker.volumes.enabled": "1",
    "cpu.frequency": "2200",
    "consul.datacenter": "dc1",
    "unique.storage.bytestotal": "31637520384",
    "unique.network.ip-address": "10.138.0.5",
    "os.version": "9.4",
    "unique.hostname": "nomad-4",
    "driver.rkt.version": "1.29.0",
    "driver.java.vm": "OpenJDK 64-Bit Server VM (build 25.162-b12, mixed mode)",
    "consul.server": "false",
    "kernel.version": "4.9.0-6-amd64",
    "cpu.numcores": "1",
    "driver.docker.version": "18.03.0-ce",
    "unique.consul.name": "nomad-4",
    "driver.java": "1",
    "consul.revision": "9a494b5f+CHANGES",
    "os.name": "debian",
    "consul.version": "1.0.6",
    "driver.java.runtime": "OpenJDK Runtime Environment (build 1.8.0_162-8u162-b12-1~deb9u1-b12)",
    "nomad.version": "0.8.0-rc1",
    "memory.totalbytes": "3883982848",
    "unique.storage.bytesfree": "26626150400",
    "driver.docker": "1",
    "cpu.modelname": "Intel(R) Xeon(R) CPU @ 2.20GHz",
    "cpu.arch": "amd64",
    "driver.rkt.appc.version": "0.8.11"
  },
  "ComputedClass": "v1:1652208824869124256",
  "CreateIndex": 6,
  "Datacenter": "dc1",
  "Drain": false,
  "DrainStrategy": null,
  "Drivers": {
    "java": {
      "Attributes": {
        "driver.java.runtime": "OpenJDK Runtime Environment (build 1.8.0_162-8u162-b12-1~deb9u1-b12)",
        "driver.java.vm": "OpenJDK 64-Bit Server VM (build 25.162-b12, mixed mode)",
        "driver.java.version": "openjdk version \"1.8.0_162"
      },
      "Detected": true,
      "HealthDescription": "",
      "Healthy": true,
      "UpdateTime": "2018-04-11T23:33:48.781948669Z"
    },
    "qemu": {
      "Attributes": null,
      "Detected": false,
      "HealthDescription": "",
      "Healthy": false,
      "UpdateTime": "2018-04-11T23:33:48.7819898Z"
    },
    "rkt": {
      "Attributes": {
        "driver.rkt.appc.version": "0.8.11",
        "driver.rkt.volumes.enabled": "1",
        "driver.rkt.version": "1.29.0"
      },
      "Detected": true,
      "HealthDescription": "Driver rkt is detected: true",
      "Healthy": true,
      "UpdateTime": "2018-04-11T23:34:48.81079772Z"
    },
    "docker": {
      "Attributes": {
        "driver.docker.bridge_ip": "172.17.0.1",
        "driver.docker.version": "18.03.0-ce",
        "driver.docker.volumes.enabled": "1"
      },
      "Detected": true,
      "HealthDescription": "Driver is available and responsive",
      "Healthy": true,
      "UpdateTime": "2018-04-11T23:34:48.713720323Z"
    },
    "exec": {
      "Attributes": {},
      "Detected": true,
      "HealthDescription": "Driver exec is detected: true",
      "Healthy": true,
      "UpdateTime": "2018-04-11T23:34:48.711026521Z"
    },
    "raw_exec": {
      "Attributes": {},
      "Detected": true,
      "HealthDescription": "",
      "Healthy": true,
      "UpdateTime": "2018-04-11T23:33:48.710448534Z"
    }
  },
  "Events": [
    {
      "CreateIndex": 0,
      "Details": null,
      "Message": "Node registered",
      "Subsystem": "Cluster",
      "Timestamp": "2018-04-10T23:43:17Z"
    }
  ],
  "HTTPAddr": "10.138.0.5:4646",
  "ID": "f7476465-4d6e-c0de-26d0-e383c49be941",
  "Links": {
    "consul": "dc1.nomad-4"
  },
  "Meta": null,
  "ModifyIndex": 2526,
  "Name": "nomad-4",
  "NodeClass": "",
  "Reserved": {
    "CPU": 0,
    "DiskMB": 0,
    "IOPS": 0,
    "MemoryMB": 0,
    "Networks": null
  },
  "Resources": {
    "CPU": 2200,
    "DiskMB": 25392,
    "IOPS": 0,
    "MemoryMB": 3704,
    "Networks": [
      {
        "CIDR": "10.138.0.5/32",
        "Device": "eth0",
        "DynamicPorts": null,
        "IP": "10.138.0.5",
        "MBits": 1000,
        "ReservedPorts": null
      }
    ]
  },
  "SchedulingEligibility": "eligible",
  "SecretID": "",
  "Status": "ready",
  "StatusDescription": "",
  "StatusUpdatedAt": 1523552938,
  "TLSEnabled": false
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

| Blocking Queries | ACL Required                   |
| ---------------- | ------------------------------ |
| `YES`            | `node:read,namespace:read-job` |

### Parameters

- `:node_id` `(string: <required>)`- Specifies the UUID of the node. This must
  be the full UUID, not the short 8-character one. This is specified as part of
  the path.

### Sample Request

```text
$ curl \
    http://localhost:4646/v1/node/e02b6169-83bd-9df6-69bd-832765f333eb/allocations
```

### Sample Response

```json
[
  {
    "AllocModifyIndex": 2555,
    "ClientDescription": "",
    "ClientStatus": "running",
    "CreateIndex": 2555,
    "CreateTime": 1523490066575461000,
    "DeploymentID": "",
    "DeploymentStatus": {
      "Healthy": true,
      "ModifyIndex": 0
    },
    "DesiredDescription": "",
    "DesiredStatus": "run",
    "DesiredTransition": {
      "Migrate": null
    },
    "EvalID": "5129bc74-9785-c39a-08da-bddc8aa778b1",
    "FollowupEvalID": "",
    "ID": "fefe81d0-08b2-4eca-fae6-6560cde46d31",
    "Job": {
      "AllAtOnce": false,
      "Constraints": null,
      "CreateIndex": 2553,
      "Datacenters": [
        "dc1"
      ],
      "ID": "webapp",
      "JobModifyIndex": 2553,
      "Meta": null,
      "ModifyIndex": 2554,
      "Name": "webapp",
      "Namespace": "default",
      "ParameterizedJob": null,
      "ParentID": "",
      "Payload": null,
      "Periodic": null,
      "Priority": 50,
      "Region": "global",
      "Stable": false,
      "Status": "pending",
      "StatusDescription": "",
      "Stop": false,
      "SubmitTime": 1523490066563405000,
      "TaskGroups": [
        {
          "Constraints": null,
          "Count": 9,
          "EphemeralDisk": {
            "Migrate": false,
            "SizeMB": 300,
            "Sticky": false
          },
          "Meta": null,
          "Migrate": {
            "HealthCheck": "checks",
            "HealthyDeadline": 300000000000,
            "MaxParallel": 2,
            "MinHealthyTime": 15000000000
          },
          "Name": "webapp",
          "ReschedulePolicy": {
            "Attempts": 0,
            "Delay": 30000000000,
            "DelayFunction": "exponential",
            "Interval": 0,
            "MaxDelay": 3600000000000,
            "Unlimited": true
          },
          "RestartPolicy": {
            "Attempts": 2,
            "Delay": 15000000000,
            "Interval": 1800000000000,
            "Mode": "fail"
          },
          "Tasks": [
            {
              "Artifacts": null,
              "Config": {
                "args": [
                  "-text",
                  "ok4"
                ],
                "image": "hashicorp/http-echo:0.2.3",
                "port_map": [
                  {
                    "http": 5678
                  }
                ]
              },
              "Constraints": null,
              "DispatchPayload": null,
              "Driver": "docker",
              "Env": null,
              "KillSignal": "",
              "KillTimeout": 5000000000,
              "Leader": false,
              "LogConfig": {
                "MaxFileSizeMB": 10,
                "MaxFiles": 10
              },
              "Meta": null,
              "Name": "webapp",
              "Resources": {
                "CPU": 100,
                "DiskMB": 0,
                "IOPS": 0,
                "MemoryMB": 300,
                "Networks": [
                  {
                    "CIDR": "",
                    "Device": "",
                    "DynamicPorts": [
                      {
                        "Label": "http",
                        "Value": 0
                      }
                    ],
                    "IP": "",
                    "MBits": 10,
                    "ReservedPorts": null
                  }
                ]
              },
              "Services": [
                {
                  "AddressMode": "auto",
                  "Checks": [
                    {
                      "AddressMode": "",
                      "Args": null,
                      "CheckRestart": null,
                      "Command": "",
                      "Header": null,
                      "InitialStatus": "",
                      "Interval": 10000000000,
                      "Method": "",
                      "Name": "http-ok",
                      "Path": "/",
                      "PortLabel": "",
                      "Protocol": "",
                      "TLSSkipVerify": false,
                      "Timeout": 2000000000,
                      "Type": "http"
                    }
                  ],
                  "Name": "webapp",
                  "PortLabel": "http",
                  "Tags": null
                }
              ],
              "ShutdownDelay": 0,
              "Templates": null,
              "User": "",
              "Vault": null
            }
          ],
          "Update": null
        }
      ],
      "Type": "service",
      "Update": {
        "AutoRevert": false,
        "Canary": 0,
        "HealthCheck": "",
        "HealthyDeadline": 0,
        "MaxParallel": 0,
        "MinHealthyTime": 0,
        "Stagger": 0
      },
      "VaultToken": "",
      "Version": 0
    },
    "JobID": "webapp",
    "Metrics": {
      "AllocationTime": 63337,
      "ClassExhausted": null,
      "ClassFiltered": null,
      "CoalescedFailures": 0,
      "ConstraintFiltered": null,
      "DimensionExhausted": null,
      "NodesAvailable": {
        "dc1": 2
      },
      "NodesEvaluated": 2,
      "NodesExhausted": 0,
      "NodesFiltered": 0,
      "QuotaExhausted": null,
      "Scores": {
        "46f1c6c4-a0e5-21f6-fd5c-d76c3d84e806.binpack": 2.6950883117541586,
        "f7476465-4d6e-c0de-26d0-e383c49be941.binpack": 2.6950883117541586
      }
    },
    "ModifyIndex": 2567,
    "ModifyTime": 1523490089807324000,
    "Name": "webapp.webapp[0]",
    "Namespace": "default",
    "NextAllocation": "",
    "NodeID": "f7476465-4d6e-c0de-26d0-e383c49be941",
    "PreviousAllocation": "",
    "RescheduleTracker": null,
    "Resources": {
      "CPU": 100,
      "DiskMB": 300,
      "IOPS": 0,
      "MemoryMB": 300,
      "Networks": [
        {
          "CIDR": "",
          "Device": "eth0",
          "DynamicPorts": [
            {
              "Label": "http",
              "Value": 25920
            }
          ],
          "IP": "10.138.0.5",
          "MBits": 10,
          "ReservedPorts": null
        }
      ]
    },
    "SharedResources": {
      "CPU": 0,
      "DiskMB": 300,
      "IOPS": 0,
      "MemoryMB": 0,
      "Networks": null
    },
    "TaskGroup": "webapp",
    "TaskResources": {
      "webapp": {
        "CPU": 100,
        "DiskMB": 0,
        "IOPS": 0,
        "MemoryMB": 300,
        "Networks": [
          {
            "CIDR": "",
            "Device": "eth0",
            "DynamicPorts": [
              {
                "Label": "http",
                "Value": 25920
              }
            ],
            "IP": "10.138.0.5",
            "MBits": 10,
            "ReservedPorts": null
          }
        ]
      }
    },
    "TaskStates": {
      "webapp": {
        "Events": [
          {
            "Details": {},
            "DiskLimit": 0,
            "DisplayMessage": "Task received by client",
            "DownloadError": "",
            "DriverError": "",
            "DriverMessage": "",
            "ExitCode": 0,
            "FailedSibling": "",
            "FailsTask": false,
            "GenericSource": "",
            "KillError": "",
            "KillReason": "",
            "KillTimeout": 0,
            "Message": "",
            "RestartReason": "",
            "SetupError": "",
            "Signal": 0,
            "StartDelay": 0,
            "TaskSignal": "",
            "TaskSignalReason": "",
            "Time": 1523490066712543500,
            "Type": "Received",
            "ValidationError": "",
            "VaultError": ""
          },
          {
            "Details": {
              "message": "Building Task Directory"
            },
            "DiskLimit": 0,
            "DisplayMessage": "Building Task Directory",
            "DownloadError": "",
            "DriverError": "",
            "DriverMessage": "",
            "ExitCode": 0,
            "FailedSibling": "",
            "FailsTask": false,
            "GenericSource": "",
            "KillError": "",
            "KillReason": "",
            "KillTimeout": 0,
            "Message": "Building Task Directory",
            "RestartReason": "",
            "SetupError": "",
            "Signal": 0,
            "StartDelay": 0,
            "TaskSignal": "",
            "TaskSignalReason": "",
            "Time": 1523490066715208000,
            "Type": "Task Setup",
            "ValidationError": "",
            "VaultError": ""
          },
          {
            "Details": {},
            "DiskLimit": 0,
            "DisplayMessage": "Task started by client",
            "DownloadError": "",
            "DriverError": "",
            "DriverMessage": "",
            "ExitCode": 0,
            "FailedSibling": "",
            "FailsTask": false,
            "GenericSource": "",
            "KillError": "",
            "KillReason": "",
            "KillTimeout": 0,
            "Message": "",
            "RestartReason": "",
            "SetupError": "",
            "Signal": 0,
            "StartDelay": 0,
            "TaskSignal": "",
            "TaskSignalReason": "",
            "Time": 1523490068433051100,
            "Type": "Started",
            "ValidationError": "",
            "VaultError": ""
          }
        ],
        "Failed": false,
        "FinishedAt": "0001-01-01T00:00:00Z",
        "LastRestart": "0001-01-01T00:00:00Z",
        "Restarts": 0,
        "StartedAt": "2018-04-11T23:41:08.445128764Z",
        "State": "running"
      }
    }
  }
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

| Blocking Queries | ACL Required       |
| ---------------- | ------------------ |
| `NO`             | `node:write`       |

### Parameters

- `:node_id` `(string: <required>)`- Specifies the UUID of the node. This must
  be the full UUID, not the short 8-character one. This is specified as part of
  the path.

### Sample Request

```text
$ curl \
    http://localhost:4646/v1/node/fb2170a8-257d-3c64-b14d-bc06cc94e34c/evaluate
```

### Sample Response

```json
{
  "EvalCreateIndex": 3671,
  "EvalIDs": [
    "4dfc2db7-b481-c53b-3072-14479aa44be3"
  ],
  "HeartbeatTTL": 0,
  "Index": 3671,
  "KnownLeader": false,
  "LastContact": 0,
  "LeaderRPCAddr": "10.138.0.2:4647",
  "NodeModifyIndex": 0,
  "NumNodes": 3,
  "Servers": [
    {
      "Datacenter": "dc1",
      "RPCAdvertiseAddr": "10.138.0.2:4647",
      "RPCMajorVersion": 1,
      "RPCMinorVersion": 1
    },
    {
      "Datacenter": "dc1",
      "RPCAdvertiseAddr": "10.138.0.3:4647",
      "RPCMajorVersion": 1,
      "RPCMinorVersion": 1
    },
    {
      "Datacenter": "dc1",
      "RPCAdvertiseAddr": "10.138.0.4:4647",
      "RPCMajorVersion": 1,
      "RPCMinorVersion": 1
    }
  ]
}

```

## Drain Node

This endpoint toggles the drain mode of the node. When draining is enabled, no
further allocations will be assigned to this node, and existing allocations will
be migrated to new nodes. See the [Workload Migration 
Guide](/guides/operations/node-draining.html) for suggested usage.

| Method  | Path                      | Produces                   |
| ------- | ------------------------- | -------------------------- |
| `POST`  | `/v1/node/:node_id/drain` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required       |
| ---------------- | ------------------ |
| `NO`             | `node:write`       |

### Parameters

- `:node_id` `(string: <required>)`- Specifies the UUID of the node. This must
  be the full UUID, not the short 8-character one. This is specified as part of
  the path.

- `DrainSpec` `(object: <optional>)` - Specifies if drain mode should be
  enabled. A missing or null value disables an existing drain.

  - `Deadline` `(int: <required>)` - Specifies how long to wait in nanoseconds
    for allocations to finish migrating before they are force stopped. This is
    also how long batch jobs are given to complete before being migrated.

  - `IgnoreSystemJobs` `(bool: false)` - Specifies whether or not to stop system
    jobs as part of a drain. By default system jobs will be stopped after all
    other allocations have migrated or the deadline is reached. Setting this to
    `true` means system jobs are always left running.

- `MarkEligible` `(bool: false)` - Specifies whether to mark a node as eligible
  for scheduling again when _disabling_ a drain.

### Sample Payload

```json
{
    "DrainSpec": {
         "Deadline": "3600000000000",
         "IgnoreSystemJobs": true
    }
}
```

### Sample Request

```text
$ curl \
    -XPOST \
    --data @drain.json \
    http://localhost:4646/v1/node/fb2170a8-257d-3c64-b14d-bc06cc94e34c/drain
```

### Sample Response

```json
{
  "EvalCreateIndex": 0,
  "EvalIDs": null,
  "Index": 3742,
  "NodeModifyIndex": 3742
}
```

## Purge Node

This endpoint purges a node from the system. Nodes can still join the cluster if
they are alive.

| Method  | Path                      | Produces                   |
| ------- | ------------------------- | -------------------------- |
| `POST`  | `/v1/node/:node_id/purge` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required       |
| ---------------- | ------------------ |
| `NO`             | `node:write`       |

### Parameters

- `:node_id` `(string: <required>)`- Specifies the UUID of the node. This must
  be the full UUID, not the short 8-character one. This is specified as part of
  the path.

### Sample Request

```text
$ curl \
    -XPOST http://localhost:4646/v1/node/f7476465-4d6e-c0de-26d0-e383c49be941/purge
```

### Sample Response

```json
{
  "EvalCreateIndex": 3817,
  "EvalIDs": [
    "71bad787-5ab1-9939-be02-4809441583cd"
  ],
  "HeartbeatTTL": 0,
  "Index": 3816,
  "KnownLeader": false,
  "LastContact": 0,
  "LeaderRPCAddr": "",
  "NodeModifyIndex": 3816,
  "NumNodes": 0,
  "Servers": null
}
```

## Toggle Node Eligibility

This endpoint toggles the scheduling eligibility of the node.

| Method  | Path                            | Produces                   |
| ------- | ------------------------------- | -------------------------- |
| `POST`  | `/v1/node/:node_id/eligibility` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required       |
| ---------------- | ------------------ |
| `NO`             | `node:write`       |

### Parameters

- `:node_id` `(string: <required>)`- Specifies the UUID of the node. This must
  be the full UUID, not the short 8-character one. This is specified as part of
  the path.

- `Eligibility` `(string: <required>)` - Either `eligible` or `ineligible`.

### Sample Payload

```json
{
    "Eligibility": "ineligible"
}
```

### Sample Request

```text
$ curl \
    -XPOST \
    --data @eligibility.json \
    http://localhost:4646/v1/node/fb2170a8-257d-3c64-b14d-bc06cc94e34c/eligibility
```

### Sample Response

```json
{
  "EvalCreateIndex": 0,
  "EvalIDs": null,
  "Index": 3742,
  "NodeModifyIndex": 3742
}
```

#### Field Reference

- Events - A list of the last 10 node events for this node. A node event is a
  high level concept of noteworthy events for a node.

  Each node event has the following fields:

  - `Message` - The specific message for the event, detailing what occurred.

  - `Subsystem` - The subsystem where the node event took place. Subsysystems
    include:

    - `Drain` - The Nomad server draining subsystem.

    - `Driver` - The Nomad client driver subsystem.

    - `Heartbeat` - Either Nomad client or server heartbeating subsystem.

    - `Cluster` - Nomad server cluster management subsystem.

  - `Details` - Any further details about the event, formatted as a key/value
    pair.

  - `Timestamp` - Each node event has an ISO 8601 timestamp.

  - `CreateIndex` - The Raft index at which the event was committed.
