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
