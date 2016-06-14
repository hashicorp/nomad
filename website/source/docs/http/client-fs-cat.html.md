---
layout: "http"
page_title: "HTTP API: /v1/client/fs/cat"
sidebar_current: "docs-http-client-fs-cat"
description: |-
  The '/v1/client/fs/cat` endpoint is used to read the contents of a file in an
  allocation directory.
---

# /v1/client/fs/cat

The `/fs/cat` endpoint is used to read the contents of a file in an allocation
directory. This API endpoint is hosted by the Nomad client and requests have to
be made to the Nomad client where the particular allocation was placed.

## GET

<dl>
  <dt>Description</dt>
  <dd>
     Read contents of a file in an allocation directory.
  </dd>

  <dt>Method</dt>
  <dd>GET</dd>

  <dt>URL</dt>
  <dd>`/v1/client/fs/cat/<ALLOCATION-ID>`</dd>

  <dt>Parameters</dt>
  <dd>
    <ul>
      <li>
        <span class="param">path</span>
        <span class="param-flags">required</span>
         The path relative to the root of the allocation directory. It 
         defaults to `/`
      </li>
    </ul>
  </dd>

  <dt>Returns</dt>
  <dd>

    ```
    2016/03/15 15:40:56 [DEBUG] sylog-server: launching syslog server on addr:
    /tmp/plugin096499590

    ```

  </dd>

</dl>
