---
layout: api
page_title: Client - HTTP API
sidebar_current: api-client
description: |-
  The /client endpoints interact with the local Nomad agent to interact with
  client members.
---

# Client HTTP API

The `/client` endpoints are used to interact with the Nomad clients. The API
endpoints are hosted by the Nomad client and requests have to be made to the
Client where the particular allocation was placed.

## Read Stats

This endpoint queries the actual resources consumed on a node. The API endpoint
is hosted by the Nomad client and requests have to be made to the nomad client
whose resource usage metrics are of interest.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/client/stats`              | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/client/stats
```

### Sample Response

```json
{
  "AllocDirStats": {
    "Available": 142943150080,
    "Device": "",
    "InodesUsedPercent": 0.05312946180421879,
    "Mountpoint": "",
    "Size": 249783500800,
    "Used": 106578206720,
    "UsedPercent": 42.668233241448746
  },
  "CPU": [
    {
      "CPU": "cpu0",
      "Idle": 80,
      "System": 11,
      "Total": 20,
      "User": 9
    },
    {
      "CPU": "cpu1",
      "Idle": 99,
      "System": 0,
      "Total": 1,
      "User": 1
    },
    {
      "CPU": "cpu2",
      "Idle": 89,
      "System": 7.000000000000001,
      "Total": 11,
      "User": 4
    },
    {
      "CPU": "cpu3",
      "Idle": 100,
      "System": 0,
      "Total": 0,
      "User": 0
    },
    {
      "CPU": "cpu4",
      "Idle": 92.92929292929293,
      "System": 4.040404040404041,
      "Total": 7.07070707070707,
      "User": 3.0303030303030303
    },
    {
      "CPU": "cpu5",
      "Idle": 99,
      "System": 1,
      "Total": 1,
      "User": 0
    },
    {
      "CPU": "cpu6",
      "Idle": 92.07920792079209,
      "System": 4.9504950495049505,
      "Total": 7.920792079207921,
      "User": 2.9702970297029703
    },
    {
      "CPU": "cpu7",
      "Idle": 99,
      "System": 0,
      "Total": 1,
      "User": 1
    }
  ],
  "CPUTicksConsumed": 1126.8044804480448,
  "DiskStats": [
    {
      "Available": 142943150080,
      "Device": "/dev/disk1",
      "InodesUsedPercent": 0.05312946180421879,
      "Mountpoint": "/",
      "Size": 249783500800,
      "Used": 106578206720,
      "UsedPercent": 42.668233241448746
    }
  ],
  "Memory": {
    "Available": 6232244224,
    "Free": 470618112,
    "Total": 17179869184,
    "Used": 10947624960
  },
  "Timestamp": 1495743032992498200,
  "Uptime": 193520
}
```

## Read Allocation

The client `allocation` endpoint is used to query the actual resources consumed
by an allocation. The API endpoint is hosted by the Nomad client and requests
have to be made to the nomad client whose resource usage metrics are of
interest.

| Method | Path                                 | Produces                   |
| ------ | ------------------------------------ | -------------------------- |
| `GET`  | `/client/allocation/:alloc_id/stats` | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

- `:alloc_id` `(string: <required>)` - Specifies the allocation ID to query.
  This is specified as part of the URL. Note, this must be the _full_ allocation
  ID, not the short 8-character one. This is specified as part of the path.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/client/allocation/5fc98185-17ff-26bc-a802-0c74fa471c99/stats
```

### Sample Response

```json
{
  "ResourceUsage": {
    "CpuStats": {
      "Measured": [
        "Throttled Periods",
        "Throttled Time",
        "Percent"
      ],
      "Percent": 0.14159538847117795,
      "SystemMode": 0,
      "ThrottledPeriods": 0,
      "ThrottledTime": 0,
      "TotalTicks": 3.256693934837093,
      "UserMode": 0
    },
    "MemoryStats": {
      "Cache": 1744896,
      "KernelMaxUsage": 0,
      "KernelUsage": 0,
      "MaxUsage": 4710400,
      "Measured": [
        "RSS",
        "Cache",
        "Swap",
        "Max Usage"
      ],
      "RSS": 1486848,
      "Swap": 0
    }
  },
  "Tasks": {
    "redis": {
      "Pids": null,
      "ResourceUsage": {
        "CpuStats": {
          "Measured": [
            "Throttled Periods",
            "Throttled Time",
            "Percent"
          ],
          "Percent": 0.14159538847117795,
          "SystemMode": 0,
          "ThrottledPeriods": 0,
          "ThrottledTime": 0,
          "TotalTicks": 3.256693934837093,
          "UserMode": 0
        },
        "MemoryStats": {
          "Cache": 1744896,
          "KernelMaxUsage": 0,
          "KernelUsage": 0,
          "MaxUsage": 4710400,
          "Measured": [
            "RSS",
            "Cache",
            "Swap",
            "Max Usage"
          ],
          "RSS": 1486848,
          "Swap": 0
        }
      },
      "Timestamp": 1495743243970720000
    }
  },
  "Timestamp": 1495743243970720000
}
```

## Read File

This endpoint reads the contents of a file in an allocation directory.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/client/fs/cat/:alloc_id`   | `text/plain`               |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

- `:alloc_id` `(string: <required>)` - Specifies the allocation ID to query.
  This is specified as part of the URL. Note, this must be the _full_ allocation
  ID, not the short 8-character one. This is specified as part of the path.

- `path` `(string: "/")` - Specifies the path of the file to read, relative to
  the root of the allocation directory.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/client/fs/cat/5fc98185-17ff-26bc-a802-0c74fa471c99
```

```text
$ curl \
    https://nomad.rocks/v1/client/fs/cat/5fc98185-17ff-26bc-a802-0c74fa471c99?path=alloc/file.json
```

### Sample Response

```text
(whatever was in the file...)
```


## Read File at Offset

This endpoint reads the contents of a file in an allocation directory at a
particular offset and limit.

| Method | Path                          | Produces                   |
| ------ | ----------------------------- | -------------------------- |
| `GET`  | `/client/fs/readat/:alloc_id` | `text/plain`               |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

- `:alloc_id` `(string: <required>)` - Specifies the allocation ID to query.
  This is specified as part of the URL. Note, this must be the _full_ allocation
  ID, not the short 8-character one. This is specified as part of the path.

- `path` `(string: "/")` - Specifies the path of the file to read, relative to
  the root of the allocation directory.

- `offset` `(int: <required>)` - Specifies the byte offset from where content
  will be read.

- `limit` `(int: <required>)` - Specifies the number of bytes to read from the
  offset.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/client/fs/readat/5fc98185-17ff-26bc-a802-0c74fa471c99?path=/alloc/foo&offset=1323&limit=19303
```

### Sample Response

```text
(whatever was in the file, starting from offset, up to limit bytes...)
```

## Stream File

This endpoint streams the contents of a file in an allocation directory.

| Method | Path                          | Produces                   |
| ------ | ----------------------------- | -------------------------- |
| `GET`  | `/client/fs/stream/:alloc_id` | `text/plain`               |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

- `:alloc_id` `(string: <required>)` - Specifies the allocation ID to query.
  This is specified as part of the URL. Note, this must be the _full_ allocation
  ID, not the short 8-character one. This is specified as part of the path.

- `path` `(string: "/")` - Specifies the path of the file to read, relative to
  the root of the allocation directory.

- `offset` `(int: <required>)` - Specifies the byte offset from where content
  will be read.

- `origin` `(string: "start|end")` - Applies the relative offset to either the
  start or end of the file.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/client/fs/stream/5fc98185-17ff-26bc-a802-0c74fa471c99?path=/alloc/logs/redis.log
```

### Sample Response

```json
{
  "File": "alloc/logs/redis.log",
  "Offset": 3604480,
  "Data": "NTMxOTMyCjUzMTkzMwo1MzE5MzQKNTMx..."
},
{
  "File": "alloc/logs/redis.log",
  "FileEvent": "file deleted"
}
```

#### Field Reference

The return value is a stream of frames. These frames contain the following
fields:

- `Data` - A base64 encoding of the bytes being streamed.

- `FileEvent` - An event that could cause a change in the streams position. The
  possible values are "file deleted" and "file truncated".

- `Offset` - Offset is the offset into the stream.

- `File` - The name of the file being streamed.

## Stream Logs

This endpoint streams a task's stderr/stdout logs.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/client/fs/logs/:alloc_id`  | `text/plain`               |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

- `:alloc_id` `(string: <required>)` - Specifies the allocation ID to query.
  This is specified as part of the URL. Note, this must be the _full_ allocation
  ID, not the short 8-character one. This is specified as part of the path.

- `task` `(string: <required>)` - Specifies the name of the task inside the
  allocation to stream logs from.

- `follow` `(bool: false)`- Specifies whether to tail the logs.

- `type` `(string: "stderr|stdout")` - Specifies the stream to stream.

- `offset` `(int: 0)` - Specifies the offset to start streaming from.

- `origin` `(string: "start|end")` - Specifies either "start" or "end" and
  applies the offset relative to either the start or end of the logs
  respectively. Defaults to "start".

- `plain` `(bool: false)` - Return just the plain text without framing. This can
  be useful when viewing logs in a browser.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/client/fs/logs/5fc98185-17ff-26bc-a802-0c74fa471c99
```

### Sample Response

```json
{
  "File": "alloc/logs/redis.stdout.0",
  "Offset": 3604480,
  "Data": "NTMxOTMyCjUzMTkzMwo1MzE5MzQKNTMx..."
},
{
  "File": "alloc/logs/redis.stdout.0",
  "FileEvent": "file deleted"
}
```

#### Field Reference

The return value is a stream of frames. These frames contain the following
fields:

- `Data` - A base64 encoding of the bytes being streamed.

- `FileEvent` - An event that could cause a change in the streams position. The
  possible values are "file deleted" and "file truncated".

- `Offset` - Offset is the offset into the stream.

- `File` - The name of the file being streamed.

## List Files

This endpoint lists files in an allocation directory.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/client/fs/ls/:alloc_id`    | `text/plain`               |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

- `:alloc_id` `(string: <required>)` - Specifies the allocation ID to query.
  This is specified as part of the URL. Note, this must be the _full_ allocation
  ID, not the short 8-character one. This is specified as part of the path.

- `path` `(string: "/")` - Specifies the path of the file to read, relative to
  the root of the allocation directory.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/client/fs/ls/5fc98185-17ff-26bc-a802-0c74fa471c99
```

### Sample Response

```json
[
  {
    "Name": "alloc",
    "IsDir": true,
    "Size": 4096,
    "FileMode": "drwxrwxr-x",
    "ModTime": "2016-03-15T15:40:00.414236712-07:00"
  },
  {
    "Name": "redis",
    "IsDir": true,
    "Size": 4096,
    "FileMode": "drwxrwxr-x",
    "ModTime": "2016-03-15T15:40:56.810238153-07:00"
  }
]
```

## Stat File

This endpoint stats a file in an allocation.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/client/fs/stat/:alloc_id`  | `text/plain`               |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries) and
[required ACLs](/api/index.html#acls).

| Blocking Queries | ACL Required |
| ---------------- | ------------ |
| `NO`             | `none`       |

### Parameters

- `:alloc_id` `(string: <required>)` - Specifies the allocation ID to query.
  This is specified as part of the URL. Note, this must be the _full_ allocation
  ID, not the short 8-character one. This is specified as part of the path.

- `path` `(string: "/")` - Specifies the path of the file to read, relative to
  the root of the allocation directory.

### Sample Request

```text
$ curl \
    https://nomad.rocks/v1/client/fs/stat/5fc98185-17ff-26bc-a802-0c74fa471c99
```

### Sample Response

```json
{
  "Name": "redis-syslog-collector.out",
  "IsDir": false,
  "Size": 96,
  "FileMode": "-rw-rw-r--",
  "ModTime": "2016-03-15T15:40:56.822238153-07:00"
}
```
