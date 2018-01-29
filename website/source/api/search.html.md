---
layout: api
page_title: Search - HTTP API
sidebar_current: api-search
description: |-
  The /search endpoint is used to search for Nomad objects
---

# Search HTTP API

The `/search` endpoint returns matches for a given prefix and context, where a
context can be jobs, allocations, evaluations, nodes, or deployments. When using
Nomad Enterprise, the allowed contexts include quotas and namespaces.
Additionally, a prefix can be searched for within every context.

| Method  | Path                         | Produces                   |
| ------- | ---------------------------- | -------------------------- |
| `POST`  | `/v1/search                  | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required                     |
| ---------------- | -------------------------------- |
| `NO`             | `node:read, namespace:read-jobs` |

When ACLs are enabled, requests must have a token valid for `node:read` or
`namespace:read-jobs` roles. If the token is only valid for `node:read`, then
job related results will not be returned. If the token is only valid for
`namespace:read-jobs`, then node results will not be returned.

### Parameters

- `Prefix` `(string: <required>)` - Specifies the identifer against which
  matches will be found. For example, if the given prefix were "a", potential
  matches might be "abcd", or "aabb".
- `Context` `(string: <required>)` - Defines the scope in which a search for a
  prefix operates. Contexts can be: "jobs", "evals", "allocs", "nodes",
  "deployment" or "all", where "all" means every context will be searched.

### Sample Payload (for a specific context)

```javascript
{
  "Prefix": "abc",
  "Context": "evals"
}
```

### Sample Request

```text
$ curl \
    --request POST \
    --data @payload.json \
    https://localhost:4646/v1/search
```

### Sample Response

```json
{ "Matches": {
    "evals": [
      "abc2fdc0-e1fd-2536-67d8-43af8ca798ac"
    ]
  },
  "Truncations": {
    "evals": "false"
  }
}
```

### Sample Payload (for all contexts)

```javascript
{
  "Prefix": "abc",
  "Context": ""
}
```

### Sample Request

```text
$ curl \
    --request POST \
    --data @payload.json \
    https://localhost:4646/v1/search
```

### Sample Response

```json
{ "Matches": {
    "allocs": [],
    "deployment": [],
    "evals": [
      "abc2fdc0-e1fd-2536-67d8-43af8ca798ac"
    ],
    "jobs": [
      "abcde"
    ],
    "nodes": []
  },
  "Truncations": {
    "allocs": "false",
    "deployment": "false",
    "evals": "false",
    "jobs": "false",
    "nodes": "false"
  }
}
```
