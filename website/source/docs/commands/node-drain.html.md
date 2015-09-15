---
layout: "docs"
page_title: "Commands: node-drain"
sidebar_current: "docs-commands-node-drain"
description: >
  Toggle drain mode for a given node.
---

# Comand: node-drain

The `node-drain` command is used to toggle drain mode on a given node. Drain
mode is used to move work away from a specific node.

## Usage

```
nomad node-drain [options] <node>
```

This command expects exactly one argument to specify the node ID to enable or
disable drain mode for. It is also required to pass one of `-enable` or
`-disable`, depending on which operation is desired.

## General Options

* `-address`: The address of the Nomad server. Overrides the `NOMAD_ADDR`
  environment variable if set. Defaults to `http://127.0.0.1:4646`.
