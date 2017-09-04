---
layout: api
page_title: ACL Policies - HTTP API
sidebar_current: api-acl-policies
description: |-
  The /acl/policy endpoints are used to configure and manage ACL policies.
---

# ACL Policies HTTP API

The `/acl/policies` and `/acl/policy/` endpoints are used to manage ACL policies.
For more details about ACLs, please see the [ACL Guide](/guides/acl.html).

## List Policies

This endpoint lists all ACL policies. This lists the policies that have been replicated
to the region, and may lag behind the authoritative region.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/acl/policies`              | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries), [consistency modes](/api/index.html#consistency-modes) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | ACL Required |
| ---------------- | ----------------- | ------------ |
| `YES`            | `all`             | `management` |


### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/acl/policies
```

### Sample Response

```json
[
  {
    "Name": "foo",
    "Description": "",
    "CreateIndex": 12,
    "ModifyIndex": 13,
  }
]
```

## Create or Update Policy

This endpoint creates or updates an ACL Policy. This request is always forwarded to the
authoritative region.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `POST` | `/acl/policy/:policy_name`   | `(empty body)`             |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required       |
| ---------------- | ------------------ |
| `NO`             | `management`       |

### Parameters

- `Name` `(string: <required>)` - Specifies the name of the policy.
  Creates the policy if the name does not exist, otherwise updates the existing policy.

- `Description` `(string: <optional>)` - Specifies a human readable description.

- `Rules` `(string: <required>)` - Specifies the Policy rules in HCL or JSON format.

### Sample Payload

```json
{
    "Name": "my-policy",
    "Description": "This is a great policy",
    "Rules": ""
}
```

### Sample Request

```text
$ curl \
    --request POST \
    --data @payload.json \
    https://nomad.rocks/v1/acl/policy/my-policy
```

## Read Policy

This endpoint reads an ACL policy with the given name. This queries the policy that have been
replicated to the region, and may lag behind the authoritative region.


| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET` | `/acl/policy/:policy_name`   | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries), [consistency modes](/api/index.html#consistency-modes) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | ACL Required |
| ---------------- | ----------------- | ------------ |
| `YES`            | `all`             | `management` |

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/acl/policy/foo
```

### Sample Response

```json
{
  "Name": "foo",
  "Rules": "",
  "Description": "",
  "CreateIndex": 12,
  "ModifyIndex": 13
}
```

## Delete Policy

This endpoint deletes the named ACL policy. This request is always forwarded to the
authoritative region.

| Method   | Path                         | Produces                   |
| -------- | ---------------------------- | -------------------------- |
| `DELETE` | `/acl/policy/:policy_name`   | `(empty body)`             |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required  |
| ---------------- | ------------- |
| `NO`             | `management`  |

### Parameters

- `policy_name` `(string: <required>)` - Specifies the policy name to delete.

### Sample Request

```text
$ curl \
    --request DELETE \
    https://nomad.rocks/v1/acl/policy/foo
```

