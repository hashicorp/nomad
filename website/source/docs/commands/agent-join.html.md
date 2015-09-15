---
layout: "docs"
page_title: "Commands: agent-info"
sidebar_current: "docs-commands-agent-info"
description: >
  Display information and status of a running agent.
---

# Agent Info

The `agent-info` command dumps metrics and status information of a running
agent. The infomation displayed pertains to the specific agent the CLI
connected to. Useful for troubleshooting and performance monitoring.

## Usage

```
nomad agent-info [options]
```

## General Options

* `-address`: The address of the Nomad server. Overrides the `NOMAD_ADDR`
  environment variable if set. Defaults to `http://127.0.0.1:4646`.
