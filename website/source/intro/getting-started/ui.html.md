---
layout: "intro"
page_title: "Nomad Web UI"
sidebar_current: "getting-started-ui"
description: |-
  Visit the Nomad Web UI to inspect jobs, allocations, and more.
---

# Nomad Web UI

At this point we have a fully functioning cluster with a job running in it. We have
learned how to inspect a job using `nomad status`, next we'll learn how to inspect
a job in the web client.

## Opening the Web UI

As long as Nomad is running, the Nomad UI is also running. It is hosted at the same address
and port as the Nomad HTTP API under the `/ui` namespace.

With Nomad running, visit [http://localhost:4646](http://localhost:4646) to open the Nomad UI.

## Inspecting a Job

You should be automatically redirected to `/ui/jobs` upon visiting the UI in your browser. This
pages lists all jobs known to Nomad, regardless of status. Click the `example` job to inspect it.

The job detail page shows pertinent information about the job, including overall status as well as
allocation statuses broken down by task group. It is similar to the `nomad status` CLI command.

Click on the `cache` task group to drill into the task group detail page. This page lists each allocation
for the task group.

Click on the allocation in the allocations table. This page lists all tasks for an allocation as well
as the recent events for each task. It is similar to the `nomad alloc-status` command.

The Nomad UI offers a friendly and visual alternative experience to the CLI.

## Next Steps

We've now concluded the getting started guide, however there are a number
of [next steps](next-steps.html) to get started with Nomad.
