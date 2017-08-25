---
layout: api
page_title: Jobs - HTTP API
sidebar_current: api-jobs
description: |-
  The /jobs endpoints are used to query for and interact with jobs.
---

# Jobs HTTP API

The `/jobs` endpoints are used to query for and interact with jobs.

## List Jobs

This endpoint lists all known jobs in the system registered with Nomad.

| Method | Path                      | Produces                   |
| ------ | ------------------------- | -------------------------- |
| `GET`  | `/v1/jobs`                | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `YES`            | `none`       |

### Parameters

- `prefix` `(string: "")` - Specifies a string to filter jobs on based on
  an index prefix. This is specified as a querystring parameter.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/jobs
```

```text
$ curl \
    https://nomad.rocks/v1/jobs?prefix=team
```

### Sample Response

```json
[
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
]
```

## Create Job

This endpoint creates (aka "registers") a new job in the system.

| Method  | Path                      | Produces                   |
| ------- | ------------------------- | -------------------------- |
| `POST`  | `/v1/jobs`                | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

There are no parameters, but the request _body_ contains the entire job file.

### Sample Payload

```json
{
    "Job": {
        "ID": "example",
        "Name": "example",
        "Type": "service",
        "Priority": 50,
        "Datacenters": [
            "dc1"
        ],
        "TaskGroups": [{
            "Name": "cache",
            "Count": 1,
            "Tasks": [{
                "Name": "redis",
                "Driver": "docker",
                "User": "",
                "Config": {
                    "image": "redis:3.2",
                    "port_map": [{
                        "db": 6379
                    }]
                },
                "Services": [{
                    "Id": "",
                    "Name": "global-redis-check",
                    "Tags": [
                        "global",
                        "cache"
                    ],
                    "PortLabel": "db",
                    "AddressMode": "",
                    "Checks": [{
                        "Id": "",
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
                    }]
                }],
                "Resources": {
                    "CPU": 500,
                    "MemoryMB": 256,
                    "Networks": [{
                        "Device": "",
                        "CIDR": "",
                        "IP": "",
                        "MBits": 10,
                        "DynamicPorts": [{
                            "Label": "db",
                            "Value": 0
                        }]
                    }]
                },
                "Leader": false
            }],
            "RestartPolicy": {
                "Interval": 300000000000,
                "Attempts": 10,
                "Delay": 25000000000,
                "Mode": "delay"
            },
            "EphemeralDisk": {
                "SizeMB": 300
            }
        }],
        "Update": {
            "MaxParallel": 1,
            "MinHealthyTime": 10000000000,
            "HealthyDeadline": 180000000000,
            "AutoRevert": false,
            "Canary": 0
        }
    }
}
```

### Sample Request

```text
$ curl \
    --request POST \
    --data @my-job.nomad \
    https://nomad.rocks/v1/jobs
```

### Sample Response

```json
{
  "EvalID": "",
  "EvalCreateIndex": 0,
  "JobModifyIndex": 109,
  "Warnings": "",
  "Index": 0,
  "LastContact": 0,
  "KnownLeader": false
}
```

## Read Job

This endpoint reads information about a single job for its specification and
status.

| Method | Path                      | Produces                   |
| ------ | ------------------------- | -------------------------- |
| `GET`  | `/v1/job/:job_id`         | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `YES`            | `none`       |

### Parameters

- `:job_id` `(string: <required>)` - Specifies the ID of the job (as specified in
  the job file during submission). This is specified as part of the path.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/job/my-job
```

### Sample Response

```json
{
  "Region": "global",
  "ID": "example",
  "ParentID": "",
  "Name": "example",
  "Type": "batch",
  "Priority": 50,
  "AllAtOnce": false,
  "Datacenters": [
    "dc1"
  ],
  "Constraints": [
    {
      "LTarget": "${attr.kernel.name}",
      "RTarget": "linux",
      "Operand": "="
    }
  ],
  "TaskGroups": [
    {
      "Name": "cache",
      "Count": 1,
      "Constraints": [
        {
          "LTarget": "${attr.os.signals}",
          "RTarget": "SIGUSR1",
          "Operand": "set_contains"
        }
      ],
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
          "User": "foo-user",
          "Config": {
            "image": "redis:latest",
            "port_map": [
              {
                "db": 6379
              }
            ]
          },
          "Env": {
            "foo": "bar",
            "baz": "pipe"
          },
          "Services": [
            {
              "Name": "cache-redis",
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
          "Templates": [
            {
              "SourcePath": "local/config.conf.tpl",
              "DestPath": "local/config.conf",
              "EmbeddedTmpl": "",
              "ChangeMode": "signal",
              "ChangeSignal": "SIGUSR1",
              "Splay": 5000000000,
              "Perms": ""
            }
          ],
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
                "ReservedPorts": [
                  {
                    "Label": "rpc",
                    "Value": 25566
                  }
                ],
                "DynamicPorts": [
                  {
                    "Label": "db",
                    "Value": 0
                  }
                ]
              }
            ]
          },
          "DispatchPayload": {
            "File": "config.json"
          },
          "Meta": {
            "foo": "bar",
            "baz": "pipe"
          },
          "KillTimeout": 5000000000,
          "LogConfig": {
            "MaxFiles": 10,
            "MaxFileSizeMB": 10
          },
          "Artifacts": [
            {
              "GetterSource": "http://foo.com/artifact.tar.gz",
              "GetterOptions": {
                "checksum": "md5:c4aa853ad2215426eb7d70a21922e794"
              },
              "RelativeDest": "local/"
            }
          ],
          "Leader": false
        }
      ],
      "EphemeralDisk": {
        "Sticky": false,
        "SizeMB": 300,
        "Migrate": false
      },
      "Meta": {
        "foo": "bar",
        "baz": "pipe"
      }
    }
  ],
  "Update": {
    "Stagger": 10000000000,
    "MaxParallel": 1
  },
  "Periodic": {
    "Enabled": true,
    "Spec": "* * * * *",
    "SpecType": "cron",
    "ProhibitOverlap": true
  },
  "ParameterizedJob": {
    "Payload": "required",
    "MetaRequired": [
      "foo"
    ],
    "MetaOptional": [
      "bar"
    ]
  },
  "Payload": null,
  "Meta": {
    "foo": "bar",
    "baz": "pipe"
  },
  "VaultToken": "",
  "Status": "running",
  "StatusDescription": "",
  "CreateIndex": 7,
  "ModifyIndex": 7,
  "JobModifyIndex": 7
}
```

## List Job Versions

This endpoint reads information about all versions of a job.

| Method | Path                       | Produces                   |
| ------ | -------------------------- | -------------------------- |
| `GET`  | `/v1/job/:job_id/versions` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `YES`            | `none`       |

### Parameters

- `:job_id` `(string: <required>)` - Specifies the ID of the job (as specified in
  the job file during submission). This is specified as part of the path.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/job/my-job/versions
```

### Sample Response

```json
[
  {
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
          "Stagger": 0,
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
    "CreateIndex": 7,
    "ModifyIndex": 7,
    "JobModifyIndex": 7
  }
]
```

## List Job Allocations

This endpoint reads information about a single job's allocations.

| Method | Path                          | Produces                   |
| ------ | ----------------------------- | -------------------------- |
| `GET`  | `/v1/job/:job_id/allocations` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `YES`            | `none`       |

### Parameters

- `:job_id` `(string: <required>)` - Specifies the ID of the job (as specified in
  the job file during submission). This is specified as part of the path.

- `all` `(bool: false)` - Specifies whether the list of allocations should
  include allocations from a previously registered job with the same ID. This is
  possible if the job is deregistered and reregistered.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/job/my-job/allocations
```

### Sample Response

```json
[
  {
    "ID": "ed344e0a-7290-d117-41d3-a64f853ca3c2",
    "EvalID": "a9c5effc-2242-51b2-f1fe-054ee11ab189",
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
        "StartedAt": "2017-05-25T23:41:23.240184101Z",
        "FinishedAt": "0001-01-01T00:00:00Z",
        "Events": [
          {
            "Type": "Received",
            "Time": 1495755675956923000,
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
            "Time": 1495755675957466400,
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
            "Type": "Driver",
            "Time": 1495755675970286800,
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
            "Time": 1495755683227522000,
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
    "CreateIndex": 9,
    "ModifyIndex": 13,
    "CreateTime": 1495755675944527600
  }
]
```

## List Job Evaluations

This endpoint reads information about a single job's evaluations

| Method | Path                          | Produces                   |
| ------ | ----------------------------- | -------------------------- |
| `GET`  | `/v1/job/:job_id/evaluations` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `YES`            | `none`       |

### Parameters

- `:job_id` `(string: <required>)` - Specifies the ID of the job (as specified in
  the job file during submission). This is specified as part of the path.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/job/my-job/evaluations
```

### Sample Response

```json
[
  {
    "ID": "a9c5effc-2242-51b2-f1fe-054ee11ab189",
    "Priority": 50,
    "Type": "service",
    "TriggeredBy": "job-register",
    "JobID": "example",
    "JobModifyIndex": 7,
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
    "QueuedAllocations": {
      "cache": 0
    },
    "SnapshotIndex": 8,
    "CreateIndex": 8,
    "ModifyIndex": 10
  }
]
```

## List Job Deployments

This endpoint lists a single job's deployments

| Method | Path                          | Produces                   |
| ------ | ----------------------------- | -------------------------- |
| `GET`  | `/v1/job/:job_id/deployments` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `YES`            | `none`       |

### Parameters

- `:job_id` `(string: <required>)` - Specifies the ID of the job (as specified in
  the job file during submission). This is specified as part of the path.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/job/my-job/deployments
```

### Sample Response

```json
[
  {
    "ID": "85ee4a9a-339f-a921-a9ef-0550d20b2c61",
    "JobID": "my-job",
    "JobVersion": 1,
    "JobModifyIndex": 19,
    "JobCreateIndex": 7,
    "TaskGroups": {
      "cache": {
        "AutoRevert": true,
        "Promoted": false,
        "PlacedCanaries": [
          "d0ad0808-2765-abf6-1e15-79fb7fe5a416",
          "38c70cd8-81f2-1489-a328-87bb29ec0e0f"
        ],
        "DesiredCanaries": 2,
        "DesiredTotal": 3,
        "PlacedAllocs": 2,
        "HealthyAllocs": 2,
        "UnhealthyAllocs": 0
      }
    },
    "Status": "running",
    "StatusDescription": "Deployment is running",
    "CreateIndex": 21,
    "ModifyIndex": 25
  },
  {
    "ID": "fb6070fb-4a44-e255-4e6f-8213eba3871a",
    "JobID": "my-job",
    "JobVersion": 0,
    "JobModifyIndex": 7,
    "JobCreateIndex": 7,
    "TaskGroups": {
      "cache": {
        "AutoRevert": true,
        "Promoted": false,
        "PlacedCanaries": null,
        "DesiredCanaries": 0,
        "DesiredTotal": 3,
        "PlacedAllocs": 3,
        "HealthyAllocs": 3,
        "UnhealthyAllocs": 0
      }
    },
    "Status": "successful",
    "StatusDescription": "Deployment completed successfully",
    "CreateIndex": 9,
    "ModifyIndex": 17
  }
]
```


## Read Job's Most Recent Deployment

This endpoint returns a single job's most recent deployment.

| Method | Path                          | Produces                   |
| ------ | ----------------------------- | -------------------------- |
| `GET`  | `/v1/job/:job_id/deployment`  | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `YES`            | `none`       |

### Parameters

- `:job_id` `(string: <required>)` - Specifies the ID of the job (as specified in
  the job file during submission). This is specified as part of the path.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/job/my-job/deployment
```

### Sample Response

```json
{
  "ID": "85ee4a9a-339f-a921-a9ef-0550d20b2c61",
  "JobID": "my-job",
  "JobVersion": 1,
  "JobModifyIndex": 19,
  "JobCreateIndex": 7,
  "TaskGroups": {
    "cache": {
      "AutoRevert": true,
      "Promoted": false,
      "PlacedCanaries": [
        "d0ad0808-2765-abf6-1e15-79fb7fe5a416",
        "38c70cd8-81f2-1489-a328-87bb29ec0e0f"
      ],
      "DesiredCanaries": 2,
      "DesiredTotal": 3,
      "PlacedAllocs": 2,
      "HealthyAllocs": 2,
      "UnhealthyAllocs": 0
    }
  },
  "Status": "running",
  "StatusDescription": "Deployment is running",
  "CreateIndex": 21,
  "ModifyIndex": 25
}
```


## Read Job Summary

This endpoint reads summary information about a job.

| Method | Path                       | Produces                   |
| ------ | -------------------------- | -------------------------- |
| `GET`  | `/v1/job/:job_id/summary`  | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `YES`            | `none`       |

### Parameters

- `:job_id` `(string: <required>)` - Specifies the ID of the job (as specified in
  the job file during submission). This is specified as part of the path.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/job/my-job/summary
```

### Sample Response

```json
{
  "JobID": "example",
  "Summary": {
    "cache": {
      "Queued": 0,
      "Complete": 0,
      "Failed": 0,
      "Running": 1,
      "Starting": 0,
      "Lost": 0
    }
  },
  "Children": {
    "Pending": 0,
    "Running": 0,
    "Dead": 0
  },
  "CreateIndex": 7,
  "ModifyIndex": 13
}
```

## Update Existing Job

This endpoint registers a new job or updates an existing job.

| Method  | Path                       | Produces                   |
| ------- | -------------------------- | -------------------------- |
| `POST`  | `/v1/job/:job_id`          | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

- `:job_id` `(string: <required>)` - Specifies the ID of the job (as specified in
  the job file during submission). This is specified as part of the path.

- `Job` `(Job: <required>)` - Specifies the JSON definition of the job.

- `EnforceIndex` `(int: 0)` - If set, the job will only be registered if the
  passed `JobModifyIndex` matches the current job's index. If the index is zero,
  the register only occurs if the job is new. This paradigm allows check-and-set
  style job updating.

- `JobModifyIndex` `(int: 0)` - Specifies the `JobModifyIndex` to enforce the
  current job is at.

### Sample Payload

```javascript
{
  "Job": {
    // ...
  },
  "EnforceIndex": 1,
  "JobModifyIndex": 4
}
```      

### Sample Request

```text
$ curl \
    --request POST \
    --data @payload.json \
    https://nomad.rocks/v1/job/my-job
```

### Sample Response

```json
{
  "EvalID": "d092fdc0-e1fd-2536-67d8-43af8ca798ac",
  "EvalCreateIndex": 35,
  "JobModifyIndex": 34,
}
```

## Dispatch Job

This endpoint dispatches a new instance of a parameterized job.

| Method  | Path                       | Produces                   |
| ------- | -------------------------- | -------------------------- |
| `POST`  | `/v1/job/:job_id/dispatch` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

- `:job_id` `(string: <required>)` - Specifies the ID of the job (as specified
  in the job file during submission). This is specified as part of the path.

- `Payload` `(string: "")` - Specifies a base64 encoded string containing the
  payload. This is limited to 15 KB.

- `Meta` `(meta<string|string>: nil)` - Specifies arbitrary metadata to pass to
  the job.

### Sample Payload

```json
{
  "Payload": "A28C3==",
  "Meta": {
    "key": "Value"
  }
}
```

### Sample Request

```text
$ curl \
    --request POST \
    --payload @payload.json \
    https://nomad.rocks/v1/job/my-job/dispatch
```

### Sample Response

```json
{
  "Index": 13,
  "JobCreateIndex": 12,
  "EvalCreateIndex": 13,
  "EvalID": "e5f55fac-bc69-119d-528a-1fc7ade5e02c",
  "DispatchedJobID": "example/dispatch-1485408778-81644024"
}
```

## Revert to older Job Version

This endpoint reverts the job to an older version.

| Method  | Path                       | Produces                   |
| ------- | -------------------------- | -------------------------- |
| `POST`  | `/v1/job/:job_id/revert` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

- `JobID` `(string: <required>)` - Specifies the ID of the job (as specified
  in the job file during submission). This is specified as part of the path.

- `JobVersion` `(integer: 0)` - Specifies the job version to revert to.

- `EnforcePriorVersion` `(integer: nil)` - Optional value specifying the current
  job's version. This is checked and acts as a check-and-set value before
  reverting to the specified job.

### Sample Payload

```json
{
  "JobID": "my-job",
  "JobVersion": 2,
}
```

### Sample Request

```text
$ curl \
    --request POST \
    --payload @payload.json \
    https://nomad.rocks/v1/job/my-job/revert
```

### Sample Response

```json
{
  "EvalID": "d092fdc0-e1fd-2536-67d8-43af8ca798ac",
  "EvalCreateIndex": 35,
  "JobModifyIndex": 34,
}
```


## Set Job Stability

This endpoint sets the job's stability.

| Method  | Path                       | Produces                   |
| ------- | -------------------------- | -------------------------- |
| `POST`  | `/v1/job/:job_id/stable`   | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

- `JobID` `(string: <required>)` - Specifies the ID of the job (as specified
  in the job file during submission). This is specified as part of the path.

- `JobVersion` `(integer: 0)` - Specifies the job version to set the stability on.

- `Stable` `(bool: false)` - Specifies whether the job should be marked as
  stable or not.

### Sample Payload

```json
{
  "JobID": "my-job",
  "JobVersion": 2,
  "Stable": true
}
```

### Sample Request

```text
$ curl \
    --request POST \
    --payload @payload.json \
    https://nomad.rocks/v1/job/my-job/stable
```

### Sample Response

```json
{
  "JobModifyIndex": 34,
}
```


## Create Job Evaluation

This endpoint creates a new evaluation for the given job. This can be used to
force run the scheduling logic if necessary.

| Method  | Path                       | Produces                   |
| ------- | -------------------------- | -------------------------- |
| `POST`  | `/v1/job/:job_id/evaluate` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

- `:job_id` `(string: <required>)` - Specifies the ID of the job (as specified in
  the job file during submission). This is specified as part of the path.

### Sample Request

```text
$ curl \
    --request POST \
    https://nomad.rocks/v1/job/my-job/evaluate
```

### Sample Response

```json
{
  "EvalID": "d092fdc0-e1fd-2536-67d8-43af8ca798ac",
  "EvalCreateIndex": 35,
  "JobModifyIndex": 34,
}
```

## Create Job Plan

This endpoint invokes a dry-run of the scheduler for the job.

| Method  | Path                       | Produces                   |
| ------- | -------------------------- | -------------------------- |
| `POST`  | `/v1/job/:job_id/plan`     | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

- `:job_id` `(string: <required>)` - Specifies the ID of the job (as specified in
- the job file during submission). This is specified as part of the path.

- `Job` `(string: <required>)` - Specifies the JSON definition of the job.

- `Diff` `(bool: false)` - Specifies whether the diff structure between the
  submitted and server side version of the job should be included in the
  response.

### Sample Payload

```json
{
  "Job": "...",
  "Diff": true
}
```

### Sample Request

```text
$ curl \
    --request POST \
    --payload @payload.json \
    https://nomad.rocks/v1/job/my-job/plan
```

### Sample Response

```json
{
  "Index": 0,
  "NextPeriodicLaunch": "0001-01-01T00:00:00Z",
  "Warnings": "",
  "Diff": {
    "Type": "Added",
    "TaskGroups": [
      {
        "Updates": {
          "create": 1
        },
        "Type": "Added",
        "Tasks": [
          {
            "Type": "Added",
            "Objects": [
              "..."
            ],
            "Name": "redis",
            "Fields": [
              {
                "Type": "Added",
                "Old": "",
                "New": "docker",
                "Name": "Driver",
                "Annotations": null
              },
              {
                "Type": "Added",
                "Old": "",
                "New": "5000000000",
                "Name": "KillTimeout",
                "Annotations": null
              }
            ],
            "Annotations": [
              "forces create"
            ]
          }
        ],
        "Objects": [
          "..."
        ],
        "Name": "cache",
        "Fields": [
          "..."
        ]
      }
    ],
    "Objects": [
      {
        "Type": "Added",
        "Objects": null,
        "Name": "Datacenters",
        "Fields": [
          "..."
        ]
      },
      {
        "Type": "Added",
        "Objects": null,
        "Name": "Constraint",
        "Fields": [
          "..."
        ]
      },
      {
        "Type": "Added",
        "Objects": null,
        "Name": "Update",
        "Fields": [
          "..."
        ]
      }
    ],
    "ID": "example",
    "Fields": [
      "..."
    ]
  },
  "CreatedEvals": [
    {
      "ModifyIndex": 0,
      "CreateIndex": 0,
      "SnapshotIndex": 0,
      "AnnotatePlan": false,
      "EscapedComputedClass": false,
      "NodeModifyIndex": 0,
      "NodeID": "",
      "JobModifyIndex": 0,
      "JobID": "example",
      "TriggeredBy": "job-register",
      "Type": "batch",
      "Priority": 50,
      "ID": "312e6a6d-8d01-0daf-9105-14919a66dba3",
      "Status": "blocked",
      "StatusDescription": "created to place remaining allocations",
      "Wait": 0,
      "NextEval": "",
      "PreviousEval": "80318ae4-7eda-e570-e59d-bc11df134817",
      "BlockedEval": "",
      "FailedTGAllocs": null,
      "ClassEligibility": {
        "v1:7968290453076422024": true
      }
    }
  ],
  "JobModifyIndex": 0,
  "FailedTGAllocs": {
    "cache": {
      "CoalescedFailures": 3,
      "AllocationTime": 46415,
      "Scores": null,
      "NodesEvaluated": 1,
      "NodesFiltered": 0,
      "NodesAvailable": {
        "dc1": 1
      },
      "ClassFiltered": null,
      "ConstraintFiltered": null,
      "NodesExhausted": 1,
      "ClassExhausted": null,
      "DimensionExhausted": {
        "cpu exhausted": 1
      }
    }
  },
  "Annotations": {
    "DesiredTGUpdates": {
      "cache": {
        "DestructiveUpdate": 0,
        "InPlaceUpdate": 0,
        "Stop": 0,
        "Migrate": 0,
        "Place": 11,
        "Ignore": 0
      }
    }
  }
}
```

#### Field Reference

- `Diff` - A diff structure between the submitted job and the server side
  version. The top-level object is a Job Diff which contains Task Group Diffs,
  which in turn contain Task Diffs. Each of these objects then has Object and
  Field Diff structures embedded.

- `NextPeriodicLaunch` - If the job being planned is periodic, this field will
  include the next launch time for the job.

- `CreatedEvals` - A set of evaluations that were created as a result of the
  dry-run. These evaluations can signify a follow-up rolling update evaluation
  or a blocked evaluation.

- `JobModifyIndex` - The `JobModifyIndex` of the server side version of this job.

- `FailedTGAllocs` - A set of metrics to understand any allocation failures that
  occurred for the Task Group.

- `Annotations` - Annotations include the `DesiredTGUpdates`, which tracks what
- the scheduler would do given enough resources for each Task Group.


## Force New Periodic Instance

This endpoint forces a new instance of the periodic job. A new instance will be
created even if it violates the job's
[`prohibit_overlap`](/docs/job-specification/periodic.html#prohibit_overlap)
settings. As such, this should be only used to immediately run a periodic job.

| Method  | Path                             | Produces                   |
| ------- | -------------------------------- | -------------------------- |
| `POST`  | `/v1/job/:job_id/periodic/force` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

- `:job_id` `(string: <required>)` - Specifies the ID of the job (as specified in
  the job file during submission). This is specified as part of the path.

### Sample Request

```text
$ curl \
    --request POST \
    https://nomad.rocks/v1/job/my-job/periodic/force
```

### Sample Response

```json
{
  "EvalCreateIndex": 7,
  "EvalID": "57983ddd-7fcf-3e3a-fd24-f699ccfb36f4"
}
```

## Stop a Job

This endpoint deregisters a job, and stops all allocations part of it.

| Method   | Path                       | Produces                   |
| -------- | -------------------------- | -------------------------- |
| `DELETE` | `/v1/job/:job_id`          | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

- `:job_id` `(string: <required>)` - Specifies the ID of the job (as specified in
  the job file during submission). This is specified as part of the path.

- `Purge` `(bool: false)` - Specifies that the job should stopped and purged
  immediately. This means the job will not be queryable after being stopped. If
  not set, the job will be purged by the garbage collector.

### Sample Request

```text
$ curl \
    --request DELETE \
    https://nomad.rocks/v1/job/my-job?purge=true
```

### Sample Response

```json
{
  "EvalID": "d092fdc0-e1fd-2536-67d8-43af8ca798ac",
  "EvalCreateIndex": 35,
  "JobModifyIndex": 34,
}
```
