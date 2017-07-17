---
layout: api
page_title: System - HTTP API
sidebar_current: api-system
description: |-
  The /system endpoints are used for system maintenance.
---

# System HTTP API

The `/system` endpoints are used to for system maintenance and should not be
necessary for most users.

## Force GC

This endpoint initializes a garbage collection of jobs, evaluations, allocations, and
nodes. This is an asynchronous operation.

| Method | Path                       | Produces                   |
| ------ | ---------------------------| -------------------------- |
| `PUT`  | `/v1/system/gc`            | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Sample Request

```text
$ curl \
    --request PUT \
    https://nomad.rocks/v1/system/gc
```

## Reconcile Summaries

This endpoint reconciles the summaries of all registered jobs.

| Method | Path                              | Produces                   |
| ------ | --------------------------------- | -------------------------- |
| `PUT`  | `/v1/system/reconcile/summaries`  | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |
### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/system/reconcile/summaries
```
