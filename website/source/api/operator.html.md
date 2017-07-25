---
layout: api
page_title: Operator - HTTP API
sidebar_current: api-operator
description: |-
  The /operator endpoints provides cluster-level tools for Nomad operators, such
  as interacting with the Raft subsystem.
---
# /v1/operator

The `/operator` endpoint provides cluster-level tools for Nomad operators, such
as interacting with the Raft subsystem.

~> Use this interface with extreme caution, as improper use could lead to a
Nomad outage and even loss of data.

See the [Outage Recovery](/guides/outage.html) guide for some examples of how
these capabilities are used. For a CLI to perform these operations manually,
please see the documentation for the
[`nomad operator`](/docs/commands/operator.html) command.


## Read Raft Configuration

This endpoint queries the status of a client node registered with Nomad.

| Method | Path                              | Produces                   |
| ------ | --------------------------------- | -------------------------- |
| `GET`  | `/v1/operator/raft/configuration` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

- `stale` - Specifies if the cluster should respond without an active leader.
  This is specified as a querystring parameter.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/operator/raft/configuration
```

### Sample Response

```json
{
  "Index": 1,
  "Servers": [
    {
      "Address": "127.0.0.1:4647",
      "ID": "127.0.0.1:4647",
      "Leader": true,
      "Node": "bacon-mac.global",
      "Voter": true
    }
  ]
}
```

#### Field Reference

- `Index` `(int)` - The `Index` value is the Raft corresponding to this
  configuration. The latest configuration may not yet be committed if changes
  are in flight.

- `Servers` `(array: Server)` - The returned `Servers` array has information
  about the servers in the Raft peer configuration.

  - `ID` `(string)` - The ID of the server. This is the same as the `Address`
    but may be upgraded to a GUID in a future version of Nomad.

  - `Node` `(string)` - The node name of the server, as known to Nomad, or
    `"(unknown)"` if the node is stale and not known.

  - `Address` `(string)` - The `ip:port` for the server.

  - `Leader` `(bool)` - is either "true" or "false" depending on the server's
    role in the Raft configuration.

  - `Voter` `(bool)` - is "true" or "false", indicating if the server has a vote
    in the Raft configuration. Future versions of Nomad may add support for
    non-voting servers.

## Remove Raft Peer

This endpoint removes a Nomad server with given address from the Raft
configuration. The return code signifies success or failure.

| Method   | Path                       | Produces                   |
| -------- | ---------------------------| -------------------------- |
| `DELETE` | `/v1/operator/raft/peer`   | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

- `address` `(string: <required>)` - Specifies the server to remove as
  `ip:port`. This may be provided multiple times and is provided as a
  querystring parameter.

- `stale` - Specifies if the cluster should respond without an active leader.
  This is specified as a querystring parameter.

### Sample Request

```text
$ curl \
    --request DELETE \
    https://nomad.rocks/v1/operator/raft/peer?address=1.2.3.4
```
