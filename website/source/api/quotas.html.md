---
layout: api
page_title: Quotas - HTTP API
sidebar_current: api-quotas
description: |-
  The /quota endpoints are used to query for and interact with quotas.
---

# Quota HTTP API

The `/quota` endpoints are used to query for and interact with quotas.

~> **Enterprise Only!** This API endpoint and functionality only exists in
Nomad Enterprise. This is not present in the open source version of Nomad.

## List Quota Specifications

This endpoint lists all quota specifications.

| Method | Path              | Produces           |
| ------ | ----------------- | ------------------ |
| `GET`  | `/v1/quotas`  | `application/json` |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required  |
| ---------------- | ------------- |
| `YES`            | `quota:read`<br>`namespace:*` if namespace has quota attached|

### Parameters

- `prefix` `(string: "")`- Specifies a string to filter quota specifications on
  based on an index prefix. This is specified as a querystring parameter.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/quotas
```

```text
$ curl \
    https://nomad.rocks/v1/quotas?prefix=sha
```

### Sample Response

```json
[
  {
    "CreateIndex": 8,
    "Description": "Limit the shared default namespace",
    "Hash": "SgDCH7L5ZDqNSi2NmJlqdvczt/Q6mjyVwVJC0XjWglQ=",
    "Limits": [
      {
        "Hash": "NLOoV2WBU8ieJIrYXXx8NRb5C2xU61pVVWRDLEIMxlU=",
        "Region": "global",
        "RegionLimit": {
          "CPU": 2500,
          "DiskMB": 0,
          "IOPS": 0,
          "MemoryMB": 2000,
          "Networks": null
        }
      }
    ],
    "ModifyIndex": 56,
    "Name": "shared-quota"
  }
]
```

## Read Quota Specification

This endpoint reads information about a specific quota specification.

| Method | Path                | Produces                   |
| ------ | ------------------- | -------------------------- |
| `GET`  | `/v1/quota/:quota`  | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required         |
| ---------------- | -------------------- |
| `YES`            | `quota:read`<br>`namespace:*` if namespace has quota attached|

### Parameters

- `:namespace` `(string: <required>)`- Specifies the namespace to query.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/quota/shared-quota
```

### Sample Response

```json
{
  "CreateIndex": 8,
  "Description": "Limit the shared default namespace",
  "Hash": "SgDCH7L5ZDqNSi2NmJlqdvczt/Q6mjyVwVJC0XjWglQ=",
  "Limits": [
    {
      "Hash": "NLOoV2WBU8ieJIrYXXx8NRb5C2xU61pVVWRDLEIMxlU=",
      "Region": "global",
      "RegionLimit": {
        "CPU": 2500,
        "DiskMB": 0,
        "IOPS": 0,
        "MemoryMB": 2000,
        "Networks": null
      }
    }
  ],
  "ModifyIndex": 56,
  "Name": "shared-quota"
}
```

## Create or Update Quota Specification

This endpoint is used to create or update a quota specification.

| Method  | Path                                | Produces                   |
| ------- | ----------------------------------- | -------------------------- |
| `POST`  | `/v1/quota/:quota` <br> `/v1/quota` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `quota:write` |

### Parameters

- `Namespace` `(string: <required>)`- Specifies the namespace to create or
  update.

- `Description` `(string: "")` - Specifies an optional human-readable
  description of the namespace.

### Sample Payload

```javascript
{
  "Namespace": "api-prod",
  "Description": "Production API Servers"
}
```      

### Sample Request

```text
$ curl \
    --request POST \
    --data @namespace.json \
    https://nomad.rocks/v1/namespace/api-prod
```

```text
$ curl \
    --request POST \
    --data @namespace.json \
    https://nomad.rocks/v1/namespace
```

## Delete Namespace

This endpoint is used to delete a namespace.

| Method   | Path                       | Produces                   |
| -------  | -------------------------- | -------------------------- |
| `DELETE` | `/v1/namespace/:namespace` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `management` |

### Parameters

- `:namespace` `(string: <required>)`- Specifies the namespace to delete.

### Sample Request

```text
$ curl \
    --request DELETE \
    https://nomad.rocks/v1/namespace/api-prod
```
