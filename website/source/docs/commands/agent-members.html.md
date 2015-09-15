---
layout: "docs"
page_title: "Commands: agent-members"
sidebar_current: "docs-commands-agent-members"
description: >
  Display a list of the known cluster members and their status.
---

# Comand: agent-members

The `agent-members` command displays a list of the known servers in the cluster
and their current status. Member information is provided by the gossip protocol,
which is only run on server nodes.

## Usage

```
nomad agent-members [options]
```

## General Options

* `-address`: The address of the Nomad server. Overrides the `NOMAD_ADDR`
  environment variable if set. Defaults to `http://127.0.0.1:4646`.

## Members Options

* `-detailed`: Dump the basic member information as well as the raw set of tags
  for each member. This mode reveals additional information not displayed in the
  standard output format.

## Example Output

```
Name          Addr      Port  Status  Proto  Build     DC   Region
node1.global  10.0.0.8  4648  alive   2      0.1.0dev  dc1  global
node2.global  10.0.0.9  4648  alive   2      0.1.0dev  dc1  global
```
