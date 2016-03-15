---
layout: "http"
page_title: "HTTP API: /v1/client/fs/ls"
sidebar_current: "docs-http-client-fs-ls"
description: |-
  The '/1/client/fs/ls` endpoint is used to list files in an allocation
  directory.
---

# /v1/client/fs/ls

The `fs/ls` endpoint is used to list files in an allocation directory. This API
endpoint is hosted by the Nomad client and requests have to be made to the Nomad
client where the particular allocation is running.

## GET

<dl>
  <dt>Description</dt>
  <dd>
     List files in an allocation directory.
  </dd>

  <dt>Method</dt>
  <dd>GET</dd>

  <dt>URL</dt>
  <dd>`/v1/client/fs/ls`</dd>

  <dt>Parameters</dt>
  <dd>
    Path - The path relative to the root of the allocation directory. It
    defaults to `/`
  </dd>

  <dt>Returns</dt>
  <dd>
    ```javascript
    [
      {
        "Name": "alloc",
        "IsDir": true,
        "Size": 4096,
        "FileMode": "drwxrwxr-x",
        "ModTime": "2016-03-15T15:40:00.414236712-07:00"
      },
      {
        "Name": "redis",
        "IsDir": true,
        "Size": 4096,
        "FileMode": "drwxrwxr-x",
        "ModTime": "2016-03-15T15:40:56.810238153-07:00"
      }
    ]
    ```
  </dd>

</dl>
