---
layout: "http"
page_title: "HTTP API: /v1/job"
sidebar_current: "docs-http-job-"
description: |-
  The '/1/job' endpoint is used for CRUD on a single job.
---

# /v1/job/\<ID\>

The `job` endpoint is used for CRUD on a singel job. By default, the agent's local
region is used; another region can be specified using the `?region=` query parameter.

## GET

<dl>
  <dt>Description</dt>
  <dd>
    Query a single job for its specification and status.
  </dd>

  <dt>Method</dt>
  <dd>GET</dd>

  <dt>URL</dt>
  <dd>`/v1/job/<id>`</dd>

  <dt>Parameters</dt>
  <dd>
    None
  </dd>

  <dt>Returns</dt>
  <dd>

    ```javascript
    {
    "Region": "global",
    "ID": "binstore-storagelocker",
    "Name": "binstore-storagelocker",
    "Type": "service",
    "Priority": 50,
    "AllAtOnce": false,
    "Datacenters": [
        "us2",
        "eu1"
    ],
    "Constraints": [
        {
            "Hard": true,
            "LTarget": "kernel.os",
            "RTarget": "windows",
            "Operand": "=",
            "Weight": 0
        }
    ],
    "TaskGroups": [
        {
            "Name": "binsl",
            "Count": 5,
            "Constraints": [
                {
                    "Hard": true,
                    "LTarget": "kernel.os",
                    "RTarget": "linux",
                    "Operand": "=",
                    "Weight": 0
                }
            ],
            "Tasks": [
                {
                    "Name": "binstore",
                    "Driver": "docker",
                    "Config": {
                        "image": "hashicorp/binstore"
                    },
                    "Constraints": null,
                    "Resources": {
                        "CPU": 500,
                        "MemoryMB": 0,
                        "DiskMB": 0,
                        "IOPS": 0,
                        "Networks": [
                            {
                                "Device": "",
                                "CIDR": "",
                                "IP": "",
                                "MBits": 100,
                                "ReservedPorts": null,
                                "DynamicPorts": 0
                            }
                        ]
                    },
                    "Meta": null
                },
                {
                    "Name": "storagelocker",
                    "Driver": "java",
                    "Config": {
                        "image": "hashicorp/storagelocker"
                    },
                    "Constraints": [
                        {
                            "Hard": true,
                            "LTarget": "kernel.arch",
                            "RTarget": "amd64",
                            "Operand": "=",
                            "Weight": 0
                        }
                    ],
                    "Resources": {
                        "CPU": 500,
                        "MemoryMB": 0,
                        "DiskMB": 0,
                        "IOPS": 0,
                        "Networks": null
                    },
                    "Meta": null
                }
            ],
            "Meta": {
                "elb_checks": "3",
                "elb_interval": "10",
                "elb_mode": "tcp"
            }
        }
    ],
    "Update": {
        "Stagger": 0,
        "MaxParallel": 0
    },
    "Meta": {
        "foo": "bar"
    },
    "Status": "",
    "StatusDescription": "",
    "CreateIndex": 14,
    "ModifyIndex": 14
    }
    ```

  </dd>
</dl>

## PUT / POST

<dl>
  <dt>Description</dt>
  <dd>
    Registers a new job or updates an existing job
  </dd>

  <dt>Method</dt>
  <dd>PUT or POST</dd>

  <dt>URL</dt>
  <dd>`/v1/job/<ID>`</dd>

  <dt>Parameters</dt>
  <dd>
    <ul>
      <li>
        <span class="param">Job</span>
        <span class="param-flags">required</span>
        The JSON definition of the job. The general structure is given
        by the [job specification](/docs/jobspec/index.html), and matches
        the return response of GET.
      </li>
    </ul>
  </dd>

  <dt>Returns</dt>
  <dd>

    ```javascript
    {
    "EvalID": "d092fdc0-e1fd-2536-67d8-43af8ca798ac",
    "EvalCreateIndex": 35,
    "JobModifyIndex": 34,
    }
    ```

  </dd>
</dl>

## DELETE

<dl>
  <dt>Description</dt>
  <dd>
    Deregisters a job, and stops all allocations part of it.
  </dd>

  <dt>Method</dt>
  <dd>DELETE</dd>

  <dt>URL</dt>
  <dd>`/v1/job/<ID>`</dd>

  <dt>Parameters</dt>
  <dd>
    None
  </dd>

  <dt>Returns</dt>
  <dd>

    ```javascript
    {
    "EvalID": "d092fdc0-e1fd-2536-67d8-43af8ca798ac",
    "EvalCreateIndex": 35,
    "JobModifyIndex": 34,
    }
    ```

  </dd>
</dl>

# /v1/job/\<ID\>/evaluate
## PUT / POST

<dl>
  <dt>Description</dt>
  <dd>
    Creates a new evaluation for the given job. This can be used to force
    run the scheduling logic if necessary.
  </dd>

  <dt>Method</dt>
  <dd>PUT or POST</dd>

  <dt>URL</dt>
  <dd>`/v1/job/<ID>/evaluate`</dd>

  <dt>Parameters</dt>
  <dd>
    None
  </dd>

  <dt>Returns</dt>
  <dd>

    ```javascript
    {
    "EvalID": "d092fdc0-e1fd-2536-67d8-43af8ca798ac",
    "EvalCreateIndex": 35,
    "JobModifyIndex": 34,
    }
    ```

  </dd>
</dl>

# /v1/job/\<ID\>/allocations
## GET

<dl>
  <dt>Description</dt>
  <dd>
    Query the allocations belonging to a single job.
  </dd>

  <dt>Method</dt>
  <dd>GET</dd>

  <dt>URL</dt>
  <dd>`/v1/job/<id>/allocations`</dd>

  <dt>Parameters</dt>
  <dd>
    None
  </dd>

  <dt>Returns</dt>
  <dd>

    ```javascript
    [
    {
        "ID": "3575ba9d-7a12-0c96-7b28-add168c67984",
        "EvalID": "151accaa-1ac6-90fe-d427-313e70ccbb88",
        "Name": "binstore-storagelocker.binsl[0]",
        "NodeID": "a703c3ca-5ff8-11e5-9213-970ee8879d1b",
        "JobID": "binstore-storagelocker",
        "TaskGroup": "binsl",
        "DesiredStatus": "run",
        "DesiredDescription": "",
        "ClientStatus": "running",
        "ClientDescription": "",
        "CreateIndex": 16,
        "ModifyIndex": 16
    },
    ...
    ]
    ```

  </dd>
</dl>

# /v1/job/\<ID\>/evaluations
## GET

<dl>
  <dt>Description</dt>
  <dd>
    Query the evaluations belonging to a single job.
  </dd>

  <dt>Method</dt>
  <dd>GET</dd>

  <dt>URL</dt>
  <dd>`/v1/job/<id>/evaluations`</dd>

  <dt>Parameters</dt>
  <dd>
    None
  </dd>

  <dt>Returns</dt>
  <dd>

    ```javascript
    [
    {
        "ID": "151accaa-1ac6-90fe-d427-313e70ccbb88",
        "Priority": 50,
        "Type": "service",
        "TriggeredBy": "job-register",
        "JobID": "binstore-storagelocker",
        "JobModifyIndex": 14,
        "NodeID": "",
        "NodeModifyIndex": 0,
        "Status": "complete",
        "StatusDescription": "",
        "Wait": 0,
        "NextEval": "",
        "PreviousEval": "",
        "CreateIndex": 15,
        "ModifyIndex": 17
    },
    ...
    ]
    ```

  </dd>
</dl>

