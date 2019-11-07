---
layout: api
page_title: Regions - HTTP API
sidebar_current: api-regions
description: |-
  The /regions endpoints list all known regions.
---

# Regions HTTP API

The `/regions` endpoints list all known regions.

## List Regions

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/regions`                   | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Sample Request

```text
$ curl \
    https://localhost:4646/v1/regions
```

### Sample Response

```json
[
  "region1",
  "region2"
]
```
