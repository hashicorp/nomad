---
layout: "docs"
page_title: "Commands: node-status"
sidebar_current: "docs-commands-node-status"
description: >
  Display information about nodes.
---

# Command: node-status

The `node-status` command is used to display information about client nodes. A
node must first be registered with the servers before it will be visible in this
output.

## Usage

```
nomad node-status [options] [node]
```

If no node ID is passed, then the command will enter "list mode" and dump a
high-level list of all known nodes. This list output contains less information
but is a good way to get a bird's-eye view of things. If a node ID is specified,
then that particular node will be queried, and detailed information will be
displayed.

## General Options

* `-address`: The address of the Nomad server. Overrides the `NOMAD_ADDR`
  environment variable if set. Defaults to `http://127.0.0.1:4646`.

## Examples

List view:

```
$ nomad node-status
ID                                    DC   Name   Drain  Status
a72dfba2-c01f-49de-5ac6-e3391de2c50c  dc1  node1  false  ready
1f3f03ea-a420-b64b-c73b-51290ed7f481  dc1  node2  false  ready
```

Single-node view:

```
$ nomad node-status 1f3f03ea-a420-b64b-c73b-51290ed7f481
ID         = 1f3f03ea-a420-b64b-c73b-51290ed7f481
Name       = node2
Class      = 
Datacenter = dc1
Drain      = false
Status     = ready
```
