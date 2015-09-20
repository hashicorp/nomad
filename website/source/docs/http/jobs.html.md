---
layout: "http"
page_title: "HTTP API: /v1/jobs"
sidebar_current: "docs-http-jobs"
description: |-
  The '/1/jobs' endpoint is used list jobs and register new ones.
---

# /v1/jobs

The `jobs` endpoint is used to query the status of existing jobs in Nomad
and to to register new jobs. By default, the agent's local region is used;
another region can be specified using the `?region=` query parameter.

## GET

<dl>
  <dt>Description</dt>
  <dd>
    Lists all the jobs registered with Nomad.
  </dd>

  <dt>Method</dt>
  <dd>GET</dd>

  <dt>URL</dt>
  <dd>`/v1/jobs`</dd>

  <dt>Parameters</dt>
  <dd>
    None
  </dd>

  <dt>Returns</dt>
  <dd>

    ```javascript
    {
      "aws": {
        "type": "aws",
        "description": "AWS keys"
      },

      "sys": {
        "type": "system",
        "description": "system endpoint"
      }
    }
    ```

  </dd>
</dl>

## PUT / POST

<dl>
  <dt>Description</dt>
  <dd>
    Registers a new job
  </dd>

  <dt>Method</dt>
  <dd>PUT or POST</dd>

  <dt>URL</dt>
  <dd>`/v1/jobs`</dd>

  <dt>Parameters</dt>
  <dd>
    <ul>
      <li>
        <span class="param">type</span>
        <span class="param-flags">required</span>
        The name of the backend type, such as "aws"
      </li>
      <li>
        <span class="param">description</span>
        <span class="param-flags">optional</span>
        A human-friendly description of the mount.
      </li>
    </ul>
  </dd>

  <dt>Returns</dt>
  <dd>`204` response code.
  </dd>
</dl>

