---
layout: "http"
page_title: "HTTP API: /v1/periodic"
sidebar_current: "docs-http-periodic"
description: >
  The '/v1/periodic' endpoint is used to interact with periodic jobs.
---

# /v1/periodic

The `periodic` endpoint is used to interact with a single periodic job. By
default, the agent's local region is used; another region can be specified using
the `?region=` query parameter.

## PUT / POST

<dl>
  <dt>Description</dt>
  <dd>
    Forces a new instance of the periodic job. A new instance will be created
    even if it violates the job's
    [`prohibit_overlap`](/docs/jobspec/index.html#prohibit_overlap) settings. As
    such, this should be only used to immediately run a periodic job.
  </dd>

  <dt>Method</dt>
  <dd>PUT or POST</dd>

  <dt>URL</dt>
  <dd>`/v1/periodic/<ID>/force`</dd>

  <dt>Parameters</dt>
  <dd>
    None
  </dd>

  <dt>Returns</dt>
  <dd>

    ```javascript
    {
    "EvalCreateIndex": 7,
    "EvalID": "57983ddd-7fcf-3e3a-fd24-f699ccfb36f4"
    }
    ```

  </dd>
</dl>
