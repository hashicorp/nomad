---
layout: "http"
page_title: "HTTP API"
sidebar_current: "docs-http-overview"
description: |-
  Nomad has an HTTP API that can be used to programmatically use Nomad.
---

# HTTP API

The Nomad HTTP API is the primary interface to using Nomad, and is used
to query the current state of the system as well as to modify it.
The Nomad CLI makes use of the Go HTTP client and invokes the HTTP API.

All API routes are prefixed with `/v1/`. This documentation is only for the v1 API.

## Data Model

There are four primary "nouns" in Nomad, these are jobs, nodes, allocations, and evaluations:

[![Nomad Data Model](/assets/images/nomad-data-model.png)](/assets/images/nomad-data-model.png)

Jobs are submitted by users and represent a _desired state_. A job is a declarative description
of tasks to run which are bounded by constraints and require resources. Nodes are the servers
in the clusters that tasks can be scheduled on. The mapping of tasks in a job to nodes is done
using allocations. An allocation is used to declare that a set of tasks in a job should be run
on a particular node. Scheduling is the process of determining the appropriate allocations and
is done as part of an evaluation.

The API is modeled closely on the underlying data model. Use the links to the left for
documentation about specific endpoints.

There are a set of "Agent" APIs which are used to interact with a specific agent and not the
broader cluster.

