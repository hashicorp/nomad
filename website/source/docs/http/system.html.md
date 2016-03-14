---
layout: "http"
page_title: "HTTP API: /v1/system/"
sidebar_current: "docs-http-system"
description: |-
  The '/1/system/' endpoints are used to for system maintance.
---

# /v1/system

The `system` endpoint is used to for system maintance and should not be
necessary for most users. By default, the agent's local region is used; another
region can be specified using the `?region=` query parameter.

## PUT

<dl>
  <dt>Description</dt>
  <dd>
    Initiate garbage collection of jobs, evals, allocations and nodes.
  </dd>

  <dt>Method</dt>
  <dd>PUT</dd>

  <dt>URL</dt>
  <dd>`/v1/system/gc`</dd>

  <dt>Parameters</dt>
  <dd>
    None
  </dd>

  <dt>Returns</dt>
  <dd>
    None
  </dd>
</dl>
