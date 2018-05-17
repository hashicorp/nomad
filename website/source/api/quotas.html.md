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
    https://localhost:4646/v1/quotas
```

```text
$ curl \
    https://localhost:4646/v1/quotas?prefix=sha
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

- `:quota` `(string: <required>)`- Specifies the quota specification to query
  where the identifier is the quota's name.

### Sample Request

```text
$ curl \
    https://localhost:4646/v1/quota/shared-quota
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

### Body

The request body contains a valid, JSON quota specification. View the api
package to see the definition of a [`QuotaSpec`
object](https://github.com/hashicorp/nomad/blob/master/api/quota.go#L100-L131).

### Sample Payload

```javascript
{
  "Name": "shared-quota",
  "Description": "Limit the shared default namespace",
  "Limits": [
    {
      "Region": "global",
      "RegionLimit": {
        "CPU": 2500,
        "MemoryMB": 1000
      }
    }
  ]
}
```      

### Sample Request

```text
$ curl \
    --request POST \
    --data @spec.json \
    https://localhost:4646/v1/quota/shared-quota
```

```text
$ curl \
    --request POST \
    --data @spec.json \
    https://localhost:4646/v1/quota
```

## Delete Quota Specification

This endpoint is used to delete a quota specification.

| Method   | Path                       | Produces                   |
| -------  | -------------------------- | -------------------------- |
| `DELETE` | `/v1/quota/:quota` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `quota:write` |

### Parameters

- `:quota` `(string: <required>)`- Specifies the quota specification to delete
  where the identifier is the quota's name.

### Sample Request

```text
$ curl \
    --request DELETE \
    https://localhost:4646/v1/quota/shared-quota
```

## List Quota Usages

This endpoint lists all quota usages.

| Method | Path              | Produces           |
| ------ | ----------------- | ------------------ |
| `GET`  | `/v1/quota-usages`  | `application/json` |

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
    https://localhost:4646/v1/quota-usages
```

```text
$ curl \
    https://localhost:4646/v1/quota-usages?prefix=sha
```

### Sample Response

```json
[
  {
    "Used": {
      "NLOoV2WBU8ieJIrYXXx8NRb5C2xU61pVVWRDLEIMxlU=": {
        "Region": "global",
        "RegionLimit": {
          "CPU": 500,
          "MemoryMB": 256,
          "DiskMB": 0,
          "IOPS": 0,
          "Networks": null
        },
        "Hash": "NLOoV2WBU8ieJIrYXXx8NRb5C2xU61pVVWRDLEIMxlU="
      }
    },
    "Name": "default",
    "CreateIndex": 8,
    "ModifyIndex": 56
  }
]
```

## Read Quota Usage

This endpoint reads information about a specific quota usage.

| Method | Path                | Produces                   |
| ------ | ------------------- | -------------------------- |
| `GET`  | `/v1/quota/usage/:quota`  | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required         |
| ---------------- | -------------------- |
| `YES`            | `quota:read`<br>`namespace:*` if namespace has quota attached|

### Parameters

- `:quota` `(string: <required>)`- Specifies the quota specification to query
  where the identifier is the quota's name.

### Sample Request

```text
$ curl \
    https://localhost:4646/v1/quota/shared-quota
```

### Sample Response

```json
{
  "Used": {
    "NLOoV2WBU8ieJIrYXXx8NRb5C2xU61pVVWRDLEIMxlU=": {
      "Region": "global",
      "RegionLimit": {
        "CPU": 500,
        "MemoryMB": 256,
        "DiskMB": 0,
        "IOPS": 0,
        "Networks": null
      },
      "Hash": "NLOoV2WBU8ieJIrYXXx8NRb5C2xU61pVVWRDLEIMxlU="
    }
  },
  "Name": "default",
  "CreateIndex": 8,
  "ModifyIndex": 56
}
```
