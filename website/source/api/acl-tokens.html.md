---
layout: api
page_title: ACL Tokens - HTTP API
sidebar_current: api-acl-tokens
description: |-
  The /acl/token/ endpoints are used to configure and manage ACL tokens.
---

# ACL Tokens HTTP API

The `/acl/bootstrap`, `/acl/tokens`, and `/acl/token/` endpoints are used to manage ACL tokens.
For more details about ACLs, please see the [ACL Guide](/guides/acl.html).

## Bootstrap Token

This endpoint is used to bootstrap the ACL system and provide the initial management token.
This request is always forwarded to the authoritative region. It can only be invoked once
until a [bootstrap reset](/guides/acl.html#reseting-acl-bootstrap) is performed.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `POST` | `/acl/bootstrap`             | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required       |
| ---------------- | ------------------ |
| `NO`             | `none`             |

### Sample Request

```text
$ curl \
    --request POST \
    https://nomad.rocks/v1/acl/bootstrap
```

### Sample Response

```json
{
    "AccessorID":"b780e702-98ce-521f-2e5f-c6b87de05b24",
    "SecretID":"3f4a0fcd-7c42-773c-25db-2d31ba0c05fe",
    "Name":"Bootstrap Token",
    "Type":"management",
    "Policies":null,
    "Global":true,
    "CreateTime":"2017-08-23T22:47:14.695408057Z",
    "CreateIndex":7,
    "ModifyIndex":7
}
```

## List Tokens

This endpoint lists all ACL tokens. This lists the local tokens and the global
tokens which have been replicated to the region, and may lag behind the authoritative region.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/acl/tokens`                | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries), [consistency modes](/api/index.html#consistency-modes) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | ACL Required |
| ---------------- | ----------------- | ------------ |
| `YES`            | `all`             | `management` |


### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/acl/tokens
```

### Sample Response

```json
[
  {
    "AccessorID": "b780e702-98ce-521f-2e5f-c6b87de05b24",
    "Name": "Bootstrap Token",
    "Type": "management",
    "Policies": null,
    "Global": true,
    "CreateTime": "2017-08-23T22:47:14.695408057Z",
    "CreateIndex": 7,
    "ModifyIndex": 7
  }
]
```

## Create Token

This endpoint creates an ACL Token. If the token is a global token, the request
is forwarded to the authoritative region.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `POST` | `/acl/token`                 | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required       |
| ---------------- | ------------------ |
| `NO`             | `management`       |

### Parameters

- `Name` `(string: <optional>)` - Specifies the human readable name of the token.

- `Type` `(string: <required>)` - Specifies the type of token. Must be either `client` or `management`.

- `Policies` `(array<string>: <required>)` - Must be null or blank for `management` type tokens, otherwise must specify at least one policy for `client` type tokens.

- `Global` `(bool: <optional>)` - If true, indicates this token should be replicated globally to all regions. Otherwise, this token is created local to the target region.

### Sample Payload

```json
{
    "Name": "Readonly token",
    "Type": "client",
    "Policies": ["readonly"],
    "Global": false
}
```

### Sample Request

```text
$ curl \
    --request POST \
    --data @payload.json \
    https://nomad.rocks/v1/acl/token
```

### Sample Response

```json
{
  "AccessorID": "aa534e09-6a07-0a45-2295-a7f77063d429",
  "SecretID": "8176afd3-772d-0b71-8f85-7fa5d903e9d4",
  "Name": "Readonly token",
  "Type": "client",
  "Policies": [
    "readonly"
  ],
  "Global": false,
  "CreateTime": "2017-08-23T23:25:41.429154233Z",
  "CreateIndex": 52,
  "ModifyIndex": 52
}
```

## Update Token

This endpoint updates an existing ACL Token. If the token is a global token, the request
is forwarded to the authoritative region. Note that a token cannot be switched from global
to local or visa versa.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `POST` | `/acl/token/:accessor_id`    | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required       |
| ---------------- | ------------------ |
| `NO`             | `management`       |

### Parameters

- `AccessorID` `(string: <required>)` - Specifies the token (by accessor) that is being updated. Must match payload body and request path.

- `Name` `(string: <optional>)` - Specifies the human readable name of the token.

- `Type` `(string: <required>)` - Specifies the type of token. Must be either `client` or `management`.

- `Policies` `(array<string>: <required>)` - Must be null or blank for `management` type tokens, otherwise must specify at least one policy for `client` type tokens.

### Sample Payload

```json
{
    "AccessorID": "aa534e09-6a07-0a45-2295-a7f77063d429",
    "Name": "Read-write token",
    "Type": "client",
    "Policies": ["readwrite"],
}
```

### Sample Request

```text
$ curl \
    --request POST \
    --data @payload.json \
    https://nomad.rocks/v1/acl/token/aa534e09-6a07-0a45-2295-a7f77063d429
```

### Sample Response

```json
{
  "AccessorID": "aa534e09-6a07-0a45-2295-a7f77063d429",
  "SecretID": "8176afd3-772d-0b71-8f85-7fa5d903e9d4",
  "Name": "Read-write token",
  "Type": "client",
  "Policies": [
    "readwrite"
  ],
  "Global": false,
  "CreateTime": "2017-08-23T23:25:41.429154233Z",
  "CreateIndex": 52,
  "ModifyIndex": 64
}
```

## Read Token

This endpoint reads an ACL token with the given accessor. If the token is a global token
which has been replicated to the region it may lag behind the authoritative region.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET` | `/acl/token/:accessor_id`     | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries), [consistency modes](/api/index.html#consistency-modes) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | ACL Required |
| ---------------- | ----------------- | ------------ |
| `YES`            | `all`             | `management` |

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/acl/token/aa534e09-6a07-0a45-2295-a7f77063d429
```

### Sample Response

```json
{
  "AccessorID": "aa534e09-6a07-0a45-2295-a7f77063d429",
  "SecretID": "8176afd3-772d-0b71-8f85-7fa5d903e9d4",
  "Name": "Read-write token",
  "Type": "client",
  "Policies": [
    "readwrite"
  ],
  "Global": false,
  "CreateTime": "2017-08-23T23:25:41.429154233Z",
  "CreateIndex": 52,
  "ModifyIndex": 64
}
```

## Delete Token

This endpoint deletes the ACL token by accessor. This request is forwarded to the
authoritative region for global tokens.

| Method   | Path                         | Produces                   |
| -------- | ---------------------------- | -------------------------- |
| `DELETE` | `/acl/token/:accessor_id`    | `(empty body)`             |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required  |
| ---------------- | ------------- |
| `NO`             | `management`  |

### Parameters

- `accessor_id` `(string: <required>)` - Specifies the ACL token accessor ID.

### Sample Request

```text
$ curl \
    --request DELETE \
    https://nomad.rocks/v1/acl/token/aa534e09-6a07-0a45-2295-a7f77063d429
```

