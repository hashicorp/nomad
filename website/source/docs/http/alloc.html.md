---
layout: "http"
page_title: "HTTP API: /v1/allocation"
sidebar_current: "docs-http-alloc-"
description: |-
  The '/1/allocation' endpoint is used to query a specific allocation.
---

# /v1/allocation

The `allocation` endpoint is used to query the a specific allocation.
By default, the agent's local region is used; another region can
be specified using the `?region=` query parameter.

## GET

<dl>
  <dt>Description</dt>
  <dd>
    Query a specific allocation.
  </dd>

  <dt>Method</dt>
  <dd>GET</dd>

  <dt>URL</dt>
  <dd>`/v1/allocation/<ID>`</dd>

  <dt>Parameters</dt>
  <dd>
    None
  </dd>

  <dt>Returns</dt>
  <dd>

    ```javascript
    {
    "ID": "3575ba9d-7a12-0c96-7b28-add168c67984",
    "EvalID": "151accaa-1ac6-90fe-d427-313e70ccbb88",
    "Name": "binstore-storagelocker.binsl[3]",
    "NodeID": "",
    "JobID": "binstore-storagelocker",
    "Job": {
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
                "Hard": false,
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
                        "Hard": false,
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
                                "Hard": false,
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
    },
    "TaskGroup": "binsl",
    "Resources": {
        "CPU": 1000,
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
    "TaskResources": null,
    "Metrics": {
        "NodesEvaluated": 0,
        "NodesFiltered": 0,
        "ClassFiltered": null,
        "ConstraintFiltered": null,
        "NodesExhausted": 0,
        "ClassExhausted": null,
        "DimensionExhausted": null,
        "Scores": null,
        "AllocationTime": 9408,
        "CoalescedFailures": 4
    },
    "DesiredStatus": "failed",
    "DesiredDescription": "failed to find a node for placement",
    "ClientStatus": "failed",
    "ClientDescription": "",
    "CreateIndex": 16,
    "ModifyIndex": 16
    }
    ```

  </dd>
</dl>

