---
layout: "http"
page_title: "HTTP API: /v1/client/fs/stat"
sidebar_current: "docs-http-client-fs-stat"
description: |-
  The '/v1/client/fs/stat` endpoint is used to stat a file in an allocation
  directory.
---

# /v1/client/fs/stat

The `/fs/stat` endpoint is used to stat a file in an allocation directory. This
API endpoint is hosted by the Nomad client and requests have to be made to the
Nomad client where the particular allocation was placed.

## GET

<dl>
  <dt>Description</dt>
  <dd>
     Stat a file in an allocation directory.
  </dd>

  <dt>Method</dt>
  <dd>GET</dd>

  <dt>URL</dt>
  <dd>`/v1/client/fs/stat/<ALLOCATION-ID>`</dd>

  <dt>Parameters</dt>
  <dd>
    <ul>
      <li>
        <span class="param">path</span>
        <span class="param-flags">required</span>
        The path of the file relative to the root of the allocation directory.
      </li>
    </ul>
  </dd>

  <dt>Returns</dt>
  <dd>

    ```javascript
    {
      "Name": "redis-syslog-collector.out",
      "IsDir": false,
      "Size": 96,
      "FileMode": "-rw-rw-r--",
      "ModTime": "2016-03-15T15:40:56.822238153-07:00"
    }
    ```

  </dd>

</dl>
