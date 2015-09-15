---
layout: "docs"
page_title: "Commands: agent-force-leave"
sidebar_current: "docs-commands-agent-force-leave"
description: >
  Force a node into the "left" state.
---

# Command: agent-force-leave

The `agent-force-leave` command forces an agent to enter the "left" state.
This can be used to eject nodes which have failed and will not rejoin the
cluster. Note that if the member is actually still alive, it will eventually
rejoin the cluster again.

## Usage

```
nomad agent-force-leave [options] <node>
```

This command expects only one argument - the node which should be forced
to enter the "left" state.

## General Options

* `-address`: The address of the Nomad server. Overrides the `NOMAD_ADDR`
  environment variable if set. Defaults to `http://127.0.0.1:4646`.
