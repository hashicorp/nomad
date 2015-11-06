---
layout: "http"
page_title: "HTTP API: /v1/allocations"
sidebar_current: "docs-http-allocs"
description: |-
  The '/1/allocations' endpoint is used to list the allocations.
---

# /v1/allocations

The `allocations` endpoint is used to query the status of allocations.
By default, the agent's local region is used; another region can
be specified using the `?region=` query parameter.

## GET

<dl>
  <dt>Description</dt>
  <dd>
    Lists all the allocations.
  </dd>

  <dt>Method</dt>
  <dd>GET</dd>

  <dt>URL</dt>
  <dd>`/v1/allocations`</dd>

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
        "Name": "binstore-storagelocker.binsl[3]",
        "NodeID": "c9972143-861d-46e6-df73-1d8287bc3e66",
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
