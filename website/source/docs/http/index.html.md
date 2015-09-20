---
layout: "http"
page_title: "HTTP API"
sidebar_current: "docs-http-overview"
description: |-
  Nomad has an HTTP API that can be used to control every aspect of Nomad.
---

# HTTP API

The Nomad HTTP API gives you full access to Nomad via HTTP. Every
aspect of Nomad can be controlled via this API. The Nomad CLI uses
the HTTP API to access Nomad.

## Version Prefix

All API routes are prefixed with `/v1/`.

This documentation is only for the v1 API.

~> **Backwards compatibility:** At the current version, Nomad does
not yet promise backwards compatibility even with the v1 prefix. We'll
remove this warning when this policy changes. We expect we'll reach API
stability by Nomad 0.3.

## Transport

The API is expected to be accessed over a TLS connection at
all times, with a valid certificate that is verified by a well
behaved client. It is possible to disable TLS verification for
listeners, however, so API clients should expect to have to do both
depending on user settings.

## Authentication

Once the Nomad is unsealed, every other operation requires
a _client token_. A user may have a client token explicitly.
The client token must be sent as the `token` cookie or the
`X-Nomad-Token` HTTP header.

Otherwise, a client token can be retrieved via
[authentication backends](/docs/auth/index.html).

Each authentication backend will have one or more unauthenticated
login endpoints. These endpoints can be reached without any authentication,
and are used for authentication itself. These endpoints are specific
to each authentication backend.

Login endpoints for authentication backends that generate an identity
will be sent down with a `Set-Cookie` header as well as via JSON. If you have a
well-behaved HTTP client, then authentication information will
automatically be saved and sent to the Nomad API.

## Reading and Writing Secrets

Reading a secret via the HTTP API is done by issuing a GET using the
following URL:

```text
/v1/secret/foo
```

This maps to `secret/foo` where `foo` is the key in the `secret/` backend/

Here is an example of reading a secret using cURL:

```shell
curl \
  -H "X-Nomad-Token: f3b09679-3001-009d-2b80-9c306ab81aa6" \
  -X GET \
   http://127.0.0.1:8200/v1/secret/foo
```

To write a secret, issue a POST on the following URL:

```text
/v1/secret/foo
```

with a JSON body like:

```javascript
{
  "value": "bar"
}
```

Here is an example of writing a secret using cURL:

```shell
curl \
  -H "X-Nomad-Token: f3b09679-3001-009d-2b80-9c306ab81aa6" \
  -H "Content-Type: application/json" \
  -X POST \
  -d '{"value":"bar"}' \
  http://127.0.0.1:8200/v1/secret/baz
```

For more examples, please look at the Nomad API client.

## Help

To retrieve the help for any API within Nomad, including mounted
backends, credential providers, etc. then append `?help=1` to any
URL. If you have valid permission to access the path, then the help text
will be returned with the following structure:

```javascript
{
  "help": "help text"
}
```

## Error Response

A common JSON structure is always returned to return errors:

```javascript
{
  "errors": [
    "message",
    "another message"
  ]
}
```

This structure will be sent down for any HTTP status greater than
or equal to 400.

## HTTP Status Codes

The following HTTP status codes are used throughout the API.

- `200` - Success with data.
- `204` - Success, no data returned.
- `400` - Invalid request, missing or invalid data. See the
   "validation" section for more details on the error response.
- `401` - Unauthorized, your authentication details are either
   incorrect or you don't have access to this feature.
- `404` - Invalid path. This can both mean that the path truly
   doesn't exist or that you don't have permission to view a
   specific path. We use 404 in some cases to avoid state leakage.
- `429` - Rate limit exceeded. Try again after waiting some period
   of time.
- `500` - Internal server error. An internal error has occurred,
   try again later. If the error persists, report a bug.
- `503` - Nomad is down for maintenance or is currently sealed.
   Try again later.
