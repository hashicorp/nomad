---
layout: api
page_title: Nodes - HTTP API
sidebar_current: api-nodes
description: |-
  The /node endpoints are used to query for and interact with client nodes.
---

# Nodes HTTP API

The `/node` endpoints are used to query for and interact with client nodes.

### List Nodes

This endpoint lists all nodes registered with Nomad.

| Method | Path                      | Produces                   |
| ------ | ------------------------- | -------------------------- |
| `GET`  | `/v1/nodes`               | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `YES`            | `none`       |

### Parameters

- `prefix` `(string: "")`- Specifies a string to filter nodes on based on an
  index prefix. This is specified as a querystring parameter.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/nodes
```

```text
$ curl \
    https://nomad.rocks/v1/nodes?prefix=prod
```

### Sample Response

```json
[
  {
    "ID": "fb2170a8-257d-3c64-b14d-bc06cc94e34c",
    "Datacenter": "dc1",
    "Name": "bacon-mac",
    "NodeClass": "",
    "Drain": false,
    "Status": "ready",
    "StatusDescription": "",
    "CreateIndex": 5,
    "ModifyIndex": 45
  }
]
```

## Read Node

This endpoint queries the status of a client node.

| Method | Path                      | Produces                   |
| ------ | ------------------------- | -------------------------- |
| `GET`  | `/v1/node/:node_id`       | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `YES`            | `none`       |

### Parameters

- `:node_id` `(string: <required>)`- Specifies the ID of the node. This must be
  the full UUID, not the short 8-character one. This is specified as part of the
  path.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/node/fb2170a8-257d-3c64-b14d-bc06cc94e34c
```

### Sample Response

```json
{
  "ID": "fb2170a8-257d-3c64-b14d-bc06cc94e34c",
  "SecretID": "",
  "Datacenter": "dc1",
  "Name": "bacon-mac",
  "HTTPAddr": "127.0.0.1:4646",
  "TLSEnabled": false,
  "Attributes": {
    "os.version": "10.12.5",
    "cpu.modelname": "Intel(R) Core(TM) i7-3615QM CPU @ 2.30GHz",
    "nomad.revision": "f551dcb83e3ac144c9dbb90583b6e82d234662e9",
    "driver.docker.volumes.enabled": "1",
    "driver.docker": "1",
    "cpu.frequency": "2300",
    "memory.totalbytes": "17179869184",
    "driver.mock_driver": "1",
    "kernel.version": "16.6.0",
    "unique.network.ip-address": "127.0.0.1",
    "nomad.version": "0.5.5dev",
    "unique.hostname": "bacon-mac",
    "cpu.arch": "amd64",
    "os.name": "darwin",
    "kernel.name": "darwin",
    "unique.storage.volume": "/dev/disk1",
    "driver.docker.version": "17.03.1-ce",
    "cpu.totalcompute": "18400",
    "unique.storage.bytestotal": "249783500800",
    "cpu.numcores": "8",
    "os.signals": "SIGCONT,SIGSTOP,SIGSYS,SIGINT,SIGIOT,SIGXCPU,SIGSEGV,SIGUSR1,SIGTTIN,SIGURG,SIGUSR2,SIGABRT,SIGALRM,SIGCHLD,SIGFPE,SIGTSTP,SIGIO,SIGKILL,SIGQUIT,SIGXFSZ,SIGBUS,SIGHUP,SIGPIPE,SIGPROF,SIGTRAP,SIGTTOU,SIGILL,SIGTERM",
    "driver.raw_exec": "1",
    "unique.storage.bytesfree": "142954643456"
  },
  "Resources": {
    "CPU": 18400,
    "MemoryMB": 16384,
    "DiskMB": 136332,
    "IOPS": 0,
    "Networks": [
      {
        "Device": "lo0",
        "CIDR": "127.0.0.1/32",
        "IP": "127.0.0.1",
        "MBits": 1000,
        "ReservedPorts": null,
        "DynamicPorts": null
      }
    ]
  },
  "Reserved": {
    "CPU": 0,
    "MemoryMB": 0,
    "DiskMB": 0,
    "IOPS": 0,
    "Networks": null
  },
  "Links": null,
  "Meta": null,
  "NodeClass": "",
  "ComputedClass": "v1:10952212473894849978",
  "Drain": false,
  "Status": "ready",
  "StatusDescription": "",
  "StatusUpdatedAt": 1495748907,
  "CreateIndex": 5,
  "ModifyIndex": 45
}
```

## Create Node Evaluation

This endpoint creates a new evaluation for the given node. This can be used to
force a run of the scheduling logic.

| Method  | Path                         | Produces                   |
| ------- | ---------------------------- | -------------------------- |
| `POST`  | `/v1/node/:node_id/evaluate` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

- `:node_id` `(string: <required>)`- Specifies the UUID of the node. This must
  be the full UUID, not the short 8-character one. This is specified as part of
  the path.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/node/fb2170a8-257d-3c64-b14d-bc06cc94e34c/evaluate
```

### Sample Response

```json
{
  "HeartbeatTTL": 0,
  "EvalIDs": [
    "4ff1c7a2-c650-4058-f509-d5028ff9566e"
  ],
  "EvalCreateIndex": 85,
  "NodeModifyIndex": 0,
  "LeaderRPCAddr": "127.0.0.1:4647",
  "NumNodes": 1,
  "Servers": [
    {
      "RPCAdvertiseAddr": "127.0.0.1:4647",
      "RPCMajorVersion": 1,
      "RPCMinorVersion": 1,
      "Datacenter": "dc1"
    }
  ],
  "Index": 85,
  "LastContact": 0,
  "KnownLeader": false
}
```

## Drain Node

This endpoint toggles the drain mode of the node. When draining is enabled, no
further allocations will be assigned to this node, and existing allocations will
be migrated to new nodes.

| Method  | Path                      | Produces                   |
| ------- | ------------------------- | -------------------------- |
| `POST`  | `/v1/node/:node_id/drain` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

- `:node_id` `(string: <required>)`- Specifies the UUID of the node. This must
  be the full UUID, not the short 8-character one. This is specified as part of
  the path.

- `enable` `(bool: <required>)` - Specifies if drain mode should be enabled.
  This is specified as a query string parameter.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/node/fb2170a8-257d-3c64-b14d-bc06cc94e34c/drain?enable=true
```

### Sample Response

```json
{
  "EvalIDs": [
    "253ec083-22a7-76c9-b8b6-2bf3d4b27bfb"
  ],
  "EvalCreateIndex": 91,
  "NodeModifyIndex": 90,
  "Index": 90,
  "LastContact": 0,
  "KnownLeader": false
}
```
