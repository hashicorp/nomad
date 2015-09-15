---
layout: "docs"
page_title: "Commands: status"
sidebar_current: "docs-commands-status"
description: >
  Display information and status of jobs.
---

# Command: status

The `status` command displays status information for jobs.

## Usage

```
nomad status [options] [job]
```

This command accepts an option job ID as the sole argument. If the job ID is
provided, information about the specific job is queried and displayed. If the ID
is omitted, the command lists out all of the existing jobs and a few of the most
useful status fields for each.

## General Options

* `-address`: The address of the Nomad server. Overrides the `NOMAD_ADDR`
  environment variable if set. Defaults to `http://127.0.0.1:4646`.

## Examples

Short list of all jobs:

```
$ nomad status
ID     Type     Priority  Status
job1   service  3         pending
job2   service  3         running
job3   service  2         pending
job4   service  1         complete
```

Detailed information of a single job:

```
$ nomad status job1
ID          = job1
Name        = Test Job
Type        = service
Priority    = 3
Datacenters = dc1,dc2,dc3
Status      = pending
```
