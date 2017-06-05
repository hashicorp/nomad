---
layout: api
page_title: Agent - HTTP API
sidebar_current: api-agent
description: |-
  The /agent endpoints interact with the local Nomad agent to interact with
  members and servers.
---

# Agent HTTP API

The `/agent` endpoints are used to interact with the local Nomad agent.

## List Members

This endpoint queries the agent for the known peers in the gossip pool. This
endpoint is only applicable to servers. Due to the nature of gossip, this is
eventually consistent.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/agent/members`             | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/agent/members
```

### Sample Response

```json
{
  "ServerName": "bacon-mac",
  "ServerRegion": "global",
  "ServerDC": "dc1",
  "Members": [
    {
      "Name": "bacon-mac.global",
      "Addr": "127.0.0.1",
      "Port": 4648,
      "Tags": {
        "mvn": "1",
        "build": "0.5.5dev",
        "port": "4647",
        "bootstrap": "1",
        "role": "nomad",
        "region": "global",
        "dc": "dc1",
        "vsn": "1"
      },
      "Status": "alive",
      "ProtocolMin": 1,
      "ProtocolMax": 5,
      "ProtocolCur": 2,
      "DelegateMin": 2,
      "DelegateMax": 4,
      "DelegateCur": 4
    }
  ]
}
```

## List Servers

This endpoint lists the known server nodes. The `servers` endpoint is used to
query an agent in client mode for its list of known servers. Client nodes
register themselves with these server addresses so that they may dequeue work.
The servers endpoint can be used to keep this configuration up to date if there
are changes in the cluster.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/agent/servers`             | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/agent/servers
```

### Sample Response

```json
[
  "127.0.0.1:4647"
]
```

## Update Servers

This endpoint updates the list of known servers to the provided list. This
**replaces** all previous server addresses with the new list.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `POST` | `/agent/servers`             | `(empty body)`             |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

- `address` `(string: <required>)` - Specifies the list of addresses in the
  format `ip:port`. This is specified as a query string!

### Sample Request

```text
$ curl \
    --request POST \
    https://nomad.rocks/v1/agent/servers?address=1.2.3.4:4647&addres=5.6.7.8:4647
```

## Query Self

This endpoint queries the state of the target agent (self).

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `POST` | `/agent/self`                | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | ACL Required |
| ---------------- | ----------------- | ------------ |
| `NO`             | `none`            | `none`       |

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/agent/self
```

### Sample Response

```json
{
  "config": {
    "Addresses": {
      "HTTP": "127.0.0.1",
      "RPC": "127.0.0.1",
      "Serf": "127.0.0.1"
    },
    "AdvertiseAddrs": {
      "HTTP": "127.0.0.1:4646",
      "RPC": "127.0.0.1:4647",
      "Serf": "127.0.0.1:4648"
    },
    "BindAddr": "127.0.0.1",
    "Client": {
      "AllocDir": "",
      "ChrootEnv": {},
      "ClientMaxPort": 14512,
      "ClientMinPort": 14000,
      "Enabled": true,
      "GCDiskUsageThreshold": 99,
      "GCInodeUsageThreshold": 99,
      "GCInterval": 600000000000,
      "MaxKillTimeout": "30s",
      "Meta": {},
      "NetworkInterface": "lo0",
      "NetworkSpeed": 0,
      "NodeClass": "",
      "Options": {
        "driver.docker.volumes": "true"
      },
      "Reserved": {
        "CPU": 0,
        "DiskMB": 0,
        "IOPS": 0,
        "MemoryMB": 0,
        "ParsedReservedPorts": null,
        "ReservedPorts": ""
      },
      "Servers": null,
      "StateDir": ""
    },
    "Consul": {
      "Addr": "",
      "Auth": "",
      "AutoAdvertise": true,
      "CAFile": "",
      "CertFile": "",
      "ChecksUseAdvertise": false,
      "ClientAutoJoin": true,
      "ClientServiceName": "nomad-client",
      "EnableSSL": false,
      "KeyFile": "",
      "ServerAutoJoin": true,
      "ServerServiceName": "nomad",
      "Timeout": 5000000000,
      "Token": "",
      "VerifySSL": false
    },
    "DataDir": "",
    "Datacenter": "dc1",
    "DevMode": true,
    "DisableAnonymousSignature": true,
    "DisableUpdateCheck": false,
    "EnableDebug": true,
    "EnableSyslog": false,
    "Files": null,
    "HTTPAPIResponseHeaders": {},
    "LeaveOnInt": false,
    "LeaveOnTerm": false,
    "LogLevel": "DEBUG",
    "NodeName": "",
    "Ports": {
      "HTTP": 4646,
      "RPC": 4647,
      "Serf": 4648
    },
    "Region": "global",
    "Revision": "f551dcb83e3ac144c9dbb90583b6e82d234662e9",
    "Server": {
      "BootstrapExpect": 0,
      "DataDir": "",
      "Enabled": true,
      "EnabledSchedulers": null,
      "HeartbeatGrace": "",
      "NodeGCThreshold": "",
      "NumSchedulers": 0,
      "ProtocolVersion": 0,
      "RejoinAfterLeave": false,
      "RetryInterval": "30s",
      "RetryJoin": [],
      "RetryMaxAttempts": 0,
      "StartJoin": []
    },
    "SyslogFacility": "LOCAL0",
    "TLSConfig": {
      "CAFile": "",
      "CertFile": "",
      "EnableHTTP": false,
      "EnableRPC": false,
      "KeyFile": "",
      "VerifyServerHostname": false
    },
    "Telemetry": {
      "CirconusAPIApp": "",
      "CirconusAPIToken": "",
      "CirconusAPIURL": "",
      "CirconusBrokerID": "",
      "CirconusBrokerSelectTag": "",
      "CirconusCheckDisplayName": "",
      "CirconusCheckForceMetricActivation": "",
      "CirconusCheckID": "",
      "CirconusCheckInstanceID": "",
      "CirconusCheckSearchTag": "",
      "CirconusCheckSubmissionURL": "",
      "CirconusCheckTags": "",
      "CirconusSubmissionInterval": "",
      "CollectionInterval": "1s",
      "DataDogAddr": "",
      "DisableHostname": false,
      "PublishAllocationMetrics": false,
      "PublishNodeMetrics": false,
      "StatsdAddr": "",
      "StatsiteAddr": "",
      "UseNodeName": false
    },
    "Vault": {
      "Addr": "https://vault.service.consul:8200",
      "AllowUnauthenticated": true,
      "ConnectionRetryIntv": 30000000000,
      "Enabled": null,
      "Role": "",
      "TLSCaFile": "",
      "TLSCaPath": "",
      "TLSCertFile": "",
      "TLSKeyFile": "",
      "TLSServerName": "",
      "TLSSkipVerify": null,
      "TaskTokenTTL": "",
      "Token": "root"
    },
    "Version": "0.5.5",
    "VersionPrerelease": "dev"
  },
  "member": {
    "Addr": "127.0.0.1",
    "DelegateCur": 4,
    "DelegateMax": 4,
    "DelegateMin": 2,
    "Name": "bacon-mac.global",
    "Port": 4648,
    "ProtocolCur": 2,
    "ProtocolMax": 5,
    "ProtocolMin": 1,
    "Status": "alive",
    "Tags": {
      "role": "nomad",
      "region": "global",
      "dc": "dc1",
      "vsn": "1",
      "mvn": "1",
      "build": "0.5.5dev",
      "port": "4647",
      "bootstrap": "1"
    }
  },
  "stats": {
    "runtime": {
      "cpu_count": "8",
      "kernel.name": "darwin",
      "arch": "amd64",
      "version": "go1.8",
      "max_procs": "7",
      "goroutines": "79"
    },
    "nomad": {
      "server": "true",
      "leader": "true",
      "leader_addr": "127.0.0.1:4647",
      "bootstrap": "false",
      "known_regions": "1"
    },
    "raft": {
      "num_peers": "0",
      "fsm_pending": "0",
      "last_snapshot_index": "0",
      "last_log_term": "2",
      "commit_index": "144",
      "term": "2",
      "last_log_index": "144",
      "protocol_version_max": "3",
      "snapshot_version_max": "1",
      "latest_configuration_index": "1",
      "latest_configuration": "[{Suffrage:Voter ID:127.0.0.1:4647 Address:127.0.0.1:4647}]",
      "last_contact": "never",
      "applied_index": "144",
      "protocol_version": "1",
      "protocol_version_min": "0",
      "snapshot_version_min": "0",
      "state": "Leader",
      "last_snapshot_term": "0"
    },
    "client": {
      "heartbeat_ttl": "17.79568937s",
      "node_id": "fb2170a8-257d-3c64-b14d-bc06cc94e34c",
      "known_servers": "127.0.0.1:4647",
      "num_allocations": "0",
      "last_heartbeat": "10.107423052s"
    },
    "serf": {
      "event_time": "1",
      "event_queue": "0",
      "encrypted": "false",
      "member_time": "1",
      "query_time": "1",
      "intent_queue": "0",
      "query_queue": "0",
      "members": "1",
      "failed": "0",
      "left": "0",
      "health_score": "0"
    }
  }
}
```

## Join Agent

This endpoint introduces a new member to the gossip pool. This endpoint is only
eligible for servers.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `POST` | `/agent/join`                | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

- `address` `(string: <required>)` - Specifies the address to join in the
  `ip:port` format. This is provided as a query parameter and may be specified
  multiple times to join multiple servers.

### Sample Request

```text
$ curl \
    --request POST \
    https://nomad.rocks/v1/agent/join?address=1.2.3.4&address=5.6.7.8
```

### Sample Response

```json
{
  "error": "",
  "num_joined": 2
}
```

## Force Leave Agent

This endpoint forces a member of the gossip pool from the `"failed"` state to
the `"left"` state. This allows the consensus protocol to remove the peer and
stop attempting replication. This is only applicable for servers.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `POST` | `/agent/force-leave`         | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

- `node` `(string: <required>)` - Specifies the name of the node to force leave.

### Sample Request

```text
$ curl \
    --request POST \
    https://nomad.rocks/v1/agent/force-leave?node=client-ab2e23dc
```
