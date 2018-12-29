---
layout: api
page_title: HTTP API
sidebar_current: api-overview
description: |-
  Nomad exposes a RESTful HTTP API to control almost every aspect of the
  Nomad agent.
---

# HTTP API

The main interface to Nomad is a RESTful HTTP API. The API can query the current
state of the system as well as modify the state of the system. The Nomad CLI
actually invokes Nomad's HTTP for many commands.

## Version Prefix

All API routes are prefixed with `/v1/`.

This documentation is only for the v1 API.

~> **Backwards compatibility:** At the current version, Nomad does not yet
promise backwards compatibility even with the v1 prefix. We'll remove this
warning when this policy changes. We expect to reach API stability by Nomad
1.0.

## Addressing &amp; Ports

Nomad binds to a specific set of addresses and ports. The HTTP API is served via
the `http` address and port. This `address:port` must be accessible locally. If
you bind to `127.0.0.1:4646`, the API is only available _from that host_. If you
bind to a private internal IP, the API will be available from within that
network. If you bind to a public IP, the API will be available from the public
Internet (not recommended).

The default port for the Nomad HTTP API is `4646`. This can be overridden via
the Nomad configuration block. Here is an example curl request to query a Nomad
server with the default configuration:

```text
$ curl http://127.0.0.1:4646/v1/agent/members
```

The conventions used in the API documentation do not list a port and use the
standard URL `nomad.rocks`. Be sure to replace this with your Nomad agent URL
when using the examples.

## Data Model and Layout

There are five primary nouns in Nomad:

- jobs
- nodes
- allocations
- deployments
- evaluations

[![Nomad Data Model](/assets/images/nomad-data-model.png)](/assets/images/nomad-data-model.png)

Jobs are submitted by users and represent a _desired state_. A job is a
declarative description of tasks to run which are bounded by constraints and
require resources. Nodes are the servers in the clusters that tasks can be
scheduled on. The mapping of tasks in a job to nodes is done using allocations.
An allocation is used to declare that a set of tasks in a job should be run on a
particular node. Scheduling is the process of determining the appropriate
allocations and is done as part of an evaluation. Deployments are objects to
track a rolling update of allocations between two versions of a job.

The API is modeled closely on the underlying data model. Use the links to the
left for documentation about specific endpoints. There are also "Agent" APIs
which interact with a specific agent and not the broader cluster used for
administration.

## ACLs

Several endpoints in Nomad use or require ACL tokens to operate. The token are used to authenticate the request and determine if the request is allowed based on the associated authorizations. Tokens are specified per-request by using the `X-Nomad-Token` request header set to the `SecretID` of an ACL Token.

For more details about ACLs, please see the [ACL Guide](/guides/acl.html).

## Authentication

When ACLs are enabled, a Nomad token should be provided to API requests using the `X-Nomad-Token` header. When using authentication, clients should communicate via TLS.

Here is an example using curl:

```text
$ curl \
    --header "X-Nomad-Token: aa534e09-6a07-0a45-2295-a7f77063d429" \
    https://nomad.rocks/v1/jobs
```

## Blocking Queries

Many endpoints in Nomad support a feature known as "blocking queries". A
blocking query is used to wait for a potential change using long polling. Not
all endpoints support blocking, but each endpoint uniquely documents its support
for blocking queries in the documentation.

Endpoints that support blocking queries return an HTTP header named
`X-Nomad-Index`. This is a unique identifier representing the current state of
the requested resource.

On subsequent requests for this resource, the client can set the `index` query
string parameter to the value of `X-Nomad-Index`, indicating that the client
wishes to wait for any changes subsequent to that index.

When this is provided, the HTTP request will "hang" until a change in the system
occurs, or the maximum timeout is reached. A critical note is that the return of
a blocking request is **no guarantee** of a change. It is possible that the
timeout was reached or that there was an idempotent write that does not affect
the result of the query.

In addition to `index`, endpoints that support blocking will also honor a `wait`
parameter specifying a maximum duration for the blocking request. This is
limited to 10 minutes. If not set, the wait time defaults to 5 minutes. This
value can be specified in the form of "10s" or "5m" (i.e., 10 seconds or 5
minutes, respectively). A small random amount of additional wait time is added
to the supplied maximum `wait` time to spread out the wake up time of any
concurrent requests. This adds up to `wait / 16` additional time to the maximum
duration.

## Consistency Modes

Most of the read query endpoints support multiple levels of consistency. Since
no policy will suit all clients' needs, these consistency modes allow the user
to have the ultimate say in how to balance the trade-offs inherent in a
distributed system.

The two read modes are:

- `default` - If not specified, the default is strongly consistent in almost all
  cases. However, there is a small window in which a new leader may be elected
  during which the old leader may service stale values. The trade-off is fast
  reads but potentially stale values. The condition resulting in stale reads is
  hard to trigger, and most clients should not need to worry about this case.
  Also, note that this race condition only applies to reads, not writes.

- `stale` - This mode allows any server to service the read regardless of
  whether it is the leader. This means reads can be arbitrarily stale; however,
  results are generally consistent to within 50 milliseconds of the leader. The
  trade-off is very fast and scalable reads with a higher likelihood of stale
  values. Since this mode allows reads without a leader, a cluster that is
  unavailable will still be able to respond to queries.

To switch these modes, use the `stale` query parameter on requests.

To support bounding the acceptable staleness of data, responses provide the
`X-Nomad-LastContact` header containing the time in milliseconds that a server
was last contacted by the leader node. The `X-Nomad-KnownLeader` header also
indicates if there is a known leader. These can be used by clients to gauge the
staleness of a result and take appropriate action.

## Cross-Region Requests

By default, any request to the HTTP API will default to the region on which the
machine is servicing the request. If the agent runs in "region1", the request
will query the region "region1". A target region can be explicitly request using
the `?region` query parameter. The request will be transparently forwarded and
serviced by a server in the requested region.

## Compressed Responses

The HTTP API will gzip the response if the HTTP request denotes that the client
accepts gzip compression. This is achieved by passing the accept encoding:

```
$ curl \
    --header "Accept-Encoding: gzip" \
    https://nomad.rocks/v1/...
```

## Formatted JSON Output

By default, the output of all HTTP API requests is minimized JSON. If the client
passes `pretty` on the query string, formatted JSON will be returned.

In general, clients should prefer a client-side parser like `jq` instead of
server-formatted data. Asking the server to format the data takes away
processing cycles from more important tasks.

```
$ curl https://nomad.rocks/v1/page?pretty
```

## HTTP Methods

Nomad's API aims to be RESTful, although there are some exceptions. The API
responds to the standard HTTP verbs GET, PUT, and DELETE. Each API method will
clearly document the verb(s) it responds to and the generated response. The same
path with different verbs may trigger different behavior. For example:

```text
PUT /v1/jobs
GET /v1/jobs
```

Even though these share a path, the `PUT` operation creates a new job whereas
the `GET` operation reads all jobs.
