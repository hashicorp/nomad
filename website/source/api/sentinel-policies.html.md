---
layout: api
page_title: Sentinel Policies - HTTP API
sidebar_current: api-sentinel-policies
description: |-
  The /sentinel/policy/ endpoints are used to configure and manage Sentinel policies.
---

# Sentinel Policies HTTP API

The `/sentinel/policies` and `/sentinel/policy/` endpoints are used to manage Sentinel policies.
For more details about Sentinel policies, please see the [Sentinel Policy Guide](/guides/security/sentinel-policy.html).

Sentinel endpoints are only available when ACLs are enabled. For more details about ACLs, please see the [ACL Guide](/guides/security/acl.html).

~> **Enterprise Only!** This API endpoint and functionality only exists in
Nomad Enterprise. This is not present in the open source version of Nomad.

## List Policies

This endpoint lists all Sentinel policies. This lists the policies that have been replicated
to the region, and may lag behind the authoritative region.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/sentinel/policies`         | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries), [consistency modes](/api/index.html#consistency-modes) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | ACL Required |
| ---------------- | ----------------- | ------------ |
| `YES`            | `all`             | `management` |


### Sample Request

```text
$ curl \
    https://localhost:4646/v1/sentinel/policies
```

### Sample Response

```json
[
  {
    "Name": "foo",
    "Description": "test policy",
    "Scope": "submit-job",
    "EnforcementLevel": "advisory",
    "Hash": "CIs8aNX5OfFvo4D7ihWcQSexEJpHp+Za+dHSncVx5+8=",
    "CreateIndex": 8,
    "ModifyIndex": 8
  }
]
```

## Create or Update Policy

This endpoint creates or updates an Sentinel Policy. This request is always forwarded to the
authoritative region.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `POST` | `/sentinel/policy/:policy_name`   | `(empty body)`             |

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

- `Scope` `(string: <required>)` - Specifies the scope of when this policy applies. Only `submit-job` is currently supported.

- `EnforcementLevel` `(string: <required>)` - Specifies the enforcement level of the policy. Can be `advisory` which warns on failure,
    `hard-mandatory` which prevents an operation on failure, and `soft-mandatory` which is like `hard-mandatory` but can be overridden.

- `Policy` `(string: <required>)` - Specifies the Sentinel policy itself.

### Sample Payload

```json
{
    "Name": "my-policy",
    "Description": "This is a great policy",
    "Scope": "submit-job",
    "EnforcementLevel": "advisory",
    "Policy": "main = rule { true }",
}
```

### Sample Request

```text
$ curl \
    --request POST \
    --data @payload.json \
    https://localhost:4646/v1/sentinel/policy/my-policy
```

## Read Policy

This endpoint reads a Sentinel policy with the given name. This queries the policy that have been
replicated to the region, and may lag behind the authoritative region.


| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET` | `/sentinel/policy/:policy_name`   | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries), [consistency modes](/api/index.html#consistency-modes) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | ACL Required |
| ---------------- | ----------------- | ------------ |
| `YES`            | `all`             | `management` |

### Sample Request

```text
$ curl \
    https://localhost:4646/v1/sentinel/policy/foo
```

### Sample Response

```json
{
  "Name": "foo",
  "Description": "test policy",
  "Scope": "submit-job",
  "EnforcementLevel": "advisory",
  "Policy": "main = rule { true }\n",
  "Hash": "CIs8aNX5OfFvo4D7ihWcQSexEJpHp+Za+dHSncVx5+8=",
  "CreateIndex": 8,
  "ModifyIndex": 8
}
```

## Delete Policy

This endpoint deletes the named Sentinel policy. This request is always forwarded to the
authoritative region.

| Method   | Path                         | Produces                   |
| -------- | ---------------------------- | -------------------------- |
| `DELETE` | `/sentinel/policy/:policy_name`   | `(empty body)`             |

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
    https://localhost:4646/v1/sentinel/policy/foo
```

