---
layout: api
page_title: Namespace - HTTP API
sidebar_current: api-namespaces
description: |-
  The /namespace endpoints are used to query for and interact with namespaces.
---

# Namespace HTTP API

The `/namespace` endpoints are used to query for and interact with namespaces.

~> **Enterprise Only!** This API endpoint and functionality only exists in
Nomad Enterprise. This is not present in the open source version of Nomad.

## List Namespaces

This endpoint lists all namespaces.

| Method | Path              | Produces           |
| ------ | ----------------- | ------------------ |
| `GET`  | `/v1/namespaces`  | `application/json` |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required  |
| ---------------- | ------------- |
| `YES`            | `namespace:*`<br>Any capability on the namespace authorizes the endpoint |

### Parameters

- `prefix` `(string: "")`- Specifies a string to filter namespaces on based on
  an index prefix. This is specified as a querystring parameter.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/namespaces
```

```text
$ curl \
    https://nomad.rocks/v1/namespaces?prefix=prod
```

### Sample Response

```json
[
    {
        "CreateIndex": 31,
        "Description": "Production API Servers",
        "ModifyIndex": 31,
        "Name": "api-prod"
    },
    {
        "CreateIndex": 5,
        "Description": "Default shared namespace",
        "ModifyIndex": 5,
        "Name": "default"
    }
]
```

## Read Namespace

This endpoint reads information about a specific namespace.

| Method | Path                        | Produces                   |
| ------ | --------------------------- | -------------------------- |
| `GET`  | `/v1/namespace/:namespace`  | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required         |
| ---------------- | -------------------- |
| `YES`            | `namespace:*`<br>Any capability on the namespace authorizes the endpoint |

### Parameters

- `:namespace` `(string: <required>)`- Specifies the namespace to query.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/namespace/api-prod
```

### Sample Response

```json
{
    "CreateIndex": 31,
    "Description": "Production API Servers",
    "Hash": "N8WvePwqkp6J354eLJMKyhvsFdPELAos0VuBfMoVKoU=",
    "ModifyIndex": 31,
    "Name": "api-prod"
}
```

## Create or Update Namespace

This endpoint is used to create or update a namespace.

| Method  | Path                                            | Produces                   |
| ------- | ----------------------------------------------- | -------------------------- |
| `POST`  | `/v1/namespace/:namespace` <br> `/v1/namespace` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `management` |

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
