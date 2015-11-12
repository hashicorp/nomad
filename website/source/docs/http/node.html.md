---
layout: "http"
page_title: "HTTP API: /v1/node"
sidebar_current: "docs-http-node-"
description: |-
  The '/1/node-' endpoint is used to query a specific client node.
---

# /v1/node

The `node` endpoint is used to query the a specific client node.
By default, the agent's local region is used; another region can
be specified using the `?region=` query parameter.

## GET

<dl>
  <dt>Description</dt>
  <dd>
    Query the status of a client node registered with Nomad.
  </dd>

  <dt>Method</dt>
  <dd>GET</dd>

  <dt>URL</dt>
  <dd>`/v1/node/<ID>`</dd>

  <dt>Parameters</dt>
  <dd>
    None
  </dd>

  <dt>Blocking Queries</dt>
  <dd>
    [Supported](/docs/http/index.html#blocking-queries)
  </dd>

  <dt>Returns</dt>
  <dd>

    ```javascript
    {
    "ID": "c9972143-861d-46e6-df73-1d8287bc3e66",
    "Datacenter": "dc1",
    "Name": "Armons-MacBook-Air.local",
    "Attributes": {
        "arch": "amd64",
        "cpu.frequency": "1300.000000",
        "cpu.modelname": "Intel(R) Core(TM) i5-4250U CPU @ 1.30GHz",
        "cpu.numcores": "2",
        "cpu.totalcompute": "2600.000000",
        "driver.exec": "1",
        "driver.java": "1",
        "driver.java.runtime": "Java(TM) SE Runtime Environment (build 1.8.0_05-b13)",
        "driver.java.version": "1.8.0_05",
        "driver.java.vm": "Java HotSpot(TM) 64-Bit Server VM (build 25.5-b02, mixed mode)",
        "hostname": "Armons-MacBook-Air.local",
        "kernel.name": "darwin",
        "kernel.version": "14.4.0",
        "memory.totalbytes": "8589934592",
        "os.name": "darwin",
        "os.version": "14.4.0",
        "storage.bytesfree": "35888713728",
        "storage.bytestotal": "249821659136",
        "storage.volume": "/dev/disk1"
    },
    "Resources": {
        "CPU": 2600,
        "MemoryMB": 8192,
        "DiskMB": 34226,
        "IOPS": 0,
        "Networks": null
    },
    "Reserved": null,
    "Links": {},
    "Meta": {},
    "NodeClass": "",
    "Drain": false,
    "Status": "ready",
    "StatusDescription": "",
    "CreateIndex": 3,
    "ModifyIndex": 4
    }
    ```

  </dd>
</dl>

<dl>
  <dt>Description</dt>
  <dd>
    Query the allocations belonging to a single node.
  </dd>

  <dt>Method</dt>
  <dd>GET</dd>

  <dt>URL</dt>
  <dd>`/v1/node/<id>/allocations`</dd>

  <dt>Parameters</dt>
  <dd>
    None
  </dd>

  <dt>Blocking Queries</dt>
  <dd>
    [Supported](/docs/http/index.html#blocking-queries)
  </dd>

  <dt>Returns</dt>
  <dd>

    ```javascript
    [
      {
        "ID": "8a0c24d9-cdfc-ce67-1208-8d4524b1a9b3",
        "EvalID": "2c699410-8697-6109-86b7-430909b00bb9",
        "Name": "example.cache[0]",
        "NodeID": "12d3409b-9d27-fcad-a03d-b3c18887d153",
        "JobID": "example",
        "Job": {
          "Region": "global",
          "ID": "example",
          "Name": "example",
          "Type": "service",
          "Priority": 50,
          "AllAtOnce": false,
          "Datacenters": [
            "lon1"
          ],
          "Constraints": [
            {
              "Hard": true,
              "LTarget": "$attr.kernel.name",
              "RTarget": "linux",
              "Operand": "=",
              "Weight": 0
            }
          ],
          "TaskGroups": [
            {
              "Name": "cache",
              "Count": 1,
              "Constraints": null,
              "Tasks": [
                {
                  "Name": "redis",
                  "Driver": "docker",
                  "Config": {
                    "image": "redis:latest"
                  },
                  "Env": null,
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
                          "6379"
                        ]
                      }
                    ]
                  },
                  "Meta": null
                }
              ],
              "Meta": null
            }
          ],
          "Update": {
            "Stagger": 0,
            "MaxParallel": 0
          },
          "Meta": null,
          "Status": "",
          "StatusDescription": "",
          "CreateIndex": 6,
          "ModifyIndex": 6
        },
        "TaskGroup": "cache",
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
                "6379"
              ]
            }
          ]
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
                "IP": "10.16.0.222",
                "MBits": 0,
                "ReservedPorts": [
                  23889
                ],
                "DynamicPorts": [
                  "6379"
                ]
              }
            ]
          }
        },
        "Metrics": {
          "NodesEvaluated": 1,
          "NodesFiltered": 0,
          "ClassFiltered": null,
          "ConstraintFiltered": null,
          "NodesExhausted": 0,
          "ClassExhausted": null,
          "DimensionExhausted": null,
          "Scores": {
            "12d3409b-9d27-fcad-a03d-b3c18887d153.binpack": 10.779215064231561
          },
          "AllocationTime": 75232,
          "CoalescedFailures": 0
        },
        "DesiredStatus": "run",
        "DesiredDescription": "",
        "ClientStatus": "pending",
        "ClientDescription": "",
        "CreateIndex": 8,
        "ModifyIndex": 8
      },
    ...
    ]
    ```

  </dd>
</dl>

## PUT / POST

<dl>
  <dt>Description</dt>
  <dd>
    Creates a new evaluation for the given node. This can be used to force
    run the scheduling logic if necessary.
  </dd>

  <dt>Method</dt>
  <dd>PUT or POST</dd>

  <dt>URL</dt>
  <dd>`/v1/node/<ID>/evaluate`</dd>

  <dt>Parameters</dt>
  <dd>
    None
  </dd>

  <dt>Returns</dt>
  <dd>

    ```javascript
    {
    "EvalIDs": ["d092fdc0-e1fd-2536-67d8-43af8ca798ac"],
    "EvalCreateIndex": 35,
    "NodeModifyIndex": 34,
    }
    ```

  </dd>
</dl>

<dl>
  <dt>Description</dt>
  <dd>
    Toggle the drain mode of the node. When enabled, no further
    allocations will be assigned and existing allocations will be
    migrated.
  </dd>

  <dt>Method</dt>
  <dd>PUT or POST</dd>

  <dt>URL</dt>
  <dd>`/v1/node/<ID>/drain`</dd>

  <dt>Parameters</dt>
  <dd>
    <ul>
      <li>
        <span class="param">enable</span>
        <span class="param-flags">required</span>
        Boolean value provided as a query parameter to either set
        enabled to true or false.
      </li>
    </ul>
  </dd>

  <dt>Returns</dt>
  <dd>

    ```javascript
    {
    "EvalID": "d092fdc0-e1fd-2536-67d8-43af8ca798ac",
    "EvalCreateIndex": 35,
    "NodeModifyIndex": 34,
    }
    ```

  </dd>
</dl>
