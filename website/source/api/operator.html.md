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

See the [Outage Recovery](/guides/operations/outage.html) guide for some examples of how
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
| `NO`             | `management` |

### Parameters

- `stale` - Specifies if the cluster should respond without an active leader.
  This is specified as a querystring parameter.

### Sample Request

```text
$ curl \
    https://localhost:4646/v1/operator/raft/configuration
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
      "RaftProtocol": 2,
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
| `NO`             | `management` |

### Parameters

- `address` `(string: <optional>)` - Specifies the server to remove as
  `ip:port`. This cannot be provided along with the `id` parameter.

- `id` `(string: <optional>)` - Specifies the server to remove as
  `id`. This cannot be provided along with the `address` parameter.

### Sample Request

```text
$ curl \
    --request DELETE \
    https://localhost:4646/v1/operator/raft/peer?address=1.2.3.4
```

## Read Autopilot Configuration

This endpoint retrieves its latest Autopilot configuration.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/operator/autopilot/configuration` | `application/json` |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes), and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | ACL Required    |
| ---------------- | ----------------- | --------------- |
| `NO`             | `none`            | `operator:read` |

### Sample Request

```text
$ curl \
    https://localhost:4646/operator/autopilot/configuration
```

### Sample Response

```json
{
  "CleanupDeadServers": true,
  "LastContactThreshold": "200ms",
  "MaxTrailingLogs": 250,
  "ServerStabilizationTime": "10s",
  "EnableRedundancyZones": false,
  "DisableUpgradeMigration": false,
  "EnableCustomUpgrades": false,
  "CreateIndex": 4,
  "ModifyIndex": 4
}
```

For more information about the Autopilot configuration options, see the
[agent configuration section](/docs/configuration/autopilot.html).

## Update Autopilot Configuration

This endpoint updates the Autopilot configuration of the cluster.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/operator/autopilot/configuration` | `application/json` |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes), and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | ACL Required     |
| ---------------- | ----------------- | ---------------- |
| `NO`             | `none`            | `operator:write` |

### Parameters

- `cas` `(int: 0)` - Specifies to use a Check-And-Set operation. The update will
  only happen if the given index matches the `ModifyIndex` of the configuration
  at the time of writing.

### Sample Payload

```json
{
  "CleanupDeadServers": true,
  "LastContactThreshold": "200ms",
  "MaxTrailingLogs": 250,
  "ServerStabilizationTime": "10s",
  "EnableRedundancyZones": false,
  "DisableUpgradeMigration": false,
  "EnableCustomUpgrades": false,
  "CreateIndex": 4,
  "ModifyIndex": 4
}
```

- `CleanupDeadServers` `(bool: true)` - Specifies automatic removal of dead
  server nodes periodically and whenever a new server is added to the cluster.

- `LastContactThreshold` `(string: "200ms")` - Specifies the maximum amount of
  time a server can go without contact from the leader before being considered
  unhealthy. Must be a duration value such as `10s`.

- `MaxTrailingLogs` `(int: 250)` specifies the maximum number of log entries
  that a server can trail the leader by before being considered unhealthy.

- `ServerStabilizationTime` `(string: "10s")` - Specifies the minimum amount of
  time a server must be stable in the 'healthy' state before being added to the
  cluster. Only takes effect if all servers are running Raft protocol version 3
  or higher. Must be a duration value such as `30s`.

- `EnableRedundancyZones` `(bool: false)` - (Enterprise-only) Specifies whether 
  to enable redundancy zones.

- `DisableUpgradeMigration` `(bool: false)` - (Enterprise-only) Disables Autopilot's
  upgrade migration strategy in Nomad Enterprise of waiting until enough
  newer-versioned servers have been added to the cluster before promoting any of
  them to voters.

- `EnableCustomUpgrades` `(bool: false)` - (Enterprise-only) Specifies whether to 
  enable using custom upgrade versions when performing migrations.

## Read Health

This endpoint queries the health of the autopilot status.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/operator/autopilot/health` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes), and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | ACL Required    |
| ---------------- | ----------------- | --------------- |
| `NO`             | `none`            | `operator:read` |

### Sample Request

```text
$ curl \
    https://localhost:4646/v1/operator/autopilot/health
```

### Sample response

```json
{
  "Healthy": true,
  "FailureTolerance": 0,
  "Servers": [
    {
      "ID": "e349749b-3303-3ddf-959c-b5885a0e1f6e",
      "Name": "node1",
      "Address": "127.0.0.1:8300",
      "SerfStatus": "alive",
      "Version": "0.8.0",
      "Leader": true,
      "LastContact": "0s",
      "LastTerm": 2,
      "LastIndex": 46,
      "Healthy": true,
      "Voter": true,
      "StableSince": "2017-03-06T22:07:51Z"
    },
    {
      "ID": "e36ee410-cc3c-0a0c-c724-63817ab30303",
      "Name": "node2",
      "Address": "127.0.0.1:8205",
      "SerfStatus": "alive",
      "Version": "0.8.0",
      "Leader": false,
      "LastContact": "27.291304ms",
      "LastTerm": 2,
      "LastIndex": 46,
      "Healthy": true,
      "Voter": false,
      "StableSince": "2017-03-06T22:18:26Z"
    }
  ]
}
```

- `Healthy` is whether all the servers are currently healthy.

- `FailureTolerance` is the number of redundant healthy servers that could be
  fail without causing an outage (this would be 2 in a healthy cluster of 5
  servers).

- `Servers` holds detailed health information on each server:

  - `ID` is the Raft ID of the server.

  - `Name` is the node name of the server.

  - `Address` is the address of the server.

  - `SerfStatus` is the SerfHealth check status for the server.

  - `Version` is the Nomad version of the server.

  - `Leader` is whether this server is currently the leader.

  - `LastContact` is the time elapsed since this server's last contact with the leader.

  - `LastTerm` is the server's last known Raft leader term.

  - `LastIndex` is the index of the server's last committed Raft log entry.

  - `Healthy` is whether the server is healthy according to the current Autopilot configuration.

  - `Voter` is whether the server is a voting member of the Raft cluster.

  - `StableSince` is the time this server has been in its current `Healthy` state.

  The HTTP status code will indicate the health of the cluster. If `Healthy` is true, then a
  status of 200 will be returned. If `Healthy` is false, then a status of 429 will be returned.
