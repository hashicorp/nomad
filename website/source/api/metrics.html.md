---
layout: api
page_title: Metrics - HTTP API
sidebar_current: metrics-search
description: |-
  The /metrics endpoint is used to view metrics for Nomad
---

# Metrics HTTP API

The `/metrics` endpoint returns metrics for the current Nomad process.

| Method  | Path            | Produces                   |
| ------- | --------------- | -------------------------- |
| `GET`   | `/v1/metrics    | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

- `format` `(string: "")` - Specifies the metrics format to be other than the
  JSON default. Currently, only `prometheus` is supported as an alterntaive
  format. This is specified as a querystring parameter.

### Sample Request

```text
$ curl https://nomad.rocks/v1/metrics
```

```text
$ curl https://nomad.rocks/v1/metrics?format=prometheus
```

### Sample Response

```json
{
  "Counters":[
  {
    "Count":11,
      "Labels":{},
      "Max":1.0,
      "Mean":1.0,
      "Min":1.0,
      "Name":"nomad.nomad.rpc.query",
      "Stddev":0.0,
      "Sum":11.0
  }
  ],
  "Gauges":[
  {
    "Labels":{
      "node_id":"cd7c3e0c-0174-29dd-17ba-ea4609e0fd1f",
      "datacenter":"dc1"
    },
    "Name":"nomad.client.allocations.blocked",
    "Value":0.0
  },
  {
    "Labels":{
      "datacenter":"dc1",
      "node_id":"cd7c3e0c-0174-29dd-17ba-ea4609e0fd1f"
    },
    "Name":"nomad.client.allocations.migrating",
    "Value":0.0
  }
  ],
  "Samples":[
  {
    "Count":20,
    "Labels":{},
    "Max":0.03544100001454353,
    "Mean":0.023678050097078084,
    "Min":0.00956599973142147,
    "Name":"nomad.memberlist.gossip",
    "Stddev":0.005445327799243976,
    "Sum":0.4735610019415617
  },
  {
    "Count":1,
    "Labels":{},
    "Max":0.0964059978723526,
    "Mean":0.0964059978723526,
    "Min":0.0964059978723526,
    "Name":"nomad.nomad.client.update_status",
    "Stddev":0.0,
    "Sum":0.0964059978723526
  }
  ]
}

```

