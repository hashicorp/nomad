---
layout: api
page_title: Validate - HTTP API
sidebar_current: api-validate
description: |-
  The /validate endpoints are used to validate object structs, fields, and
  types.
---

# Validate HTTP API

The `/validate` endpoints are used to validate object structs, fields, and
types.

## Validate Job

This endpoint validates a Nomad job file. The local Nomad agent forwards the
request to a server. In the event a server can't be reached the agent verifies
the job file locally but skips validating driver configurations.

~> This endpoint accepts a **JSON job file**, not an HCL job file.

| Method  | Path                      | Produces                   |
| ------- | ------------------------- | -------------------------- |
| `POST`  | `/v1/validate/job`        | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

There are no parameters, but the request _body_ contains the entire job file.

### Sample Payload

```text
(any valid nomad job IN JSON FORMAT)
```

### Sample Request

```text
$ curl \
    --request POST \
    --data @my-job.nomad \
    https://nomad.rocks/v1/validate/job
```

### Sample Response
```json
{
  "DriverConfigValidated": true,
  "ValidationErrors": [
    "Task group cache validation failed: 1 error(s) occurred:\n\n* Task redis validation failed: 1 error(s) occurred:\n\n* 1 error(s) occurred:\n\n* minimum CPU value is 20; got 1"
  ],
  "Warnings": "1 warning(s):\n\n* Group \"cache\" has warnings: 1 error(s) occurred:\n\n* Update max parallel count is greater than task group count (13 > 1). A destructive change would result in the simultaneous replacement of all allocations.",
  "Error": "1 error(s) occurred:\n\n* Task group cache validation failed: 1 error(s) occurred:\n\n* Task redis validation failed: 1 error(s) occurred:\n\n* 1 error(s) occurred:\n\n* minimum CPU value is 20; got 1"
}
```
