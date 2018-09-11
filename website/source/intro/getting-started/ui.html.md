---
layout: "intro"
page_title: "Web UI"
sidebar_current: "getting-started-ui"
description: |-
  Visit the Nomad Web UI to inspect jobs, allocations, and more.
---

# Web UI

At this point we have a fully functioning cluster with a job running in it. We have
learned how to inspect a job using `nomad status`, next we'll learn how to inspect
a job in the web client.

## Opening the Web UI

As long as Nomad is running, the Nomad UI is also running. It is hosted at the same address
and port as the Nomad HTTP API under the `/ui` namespace.

With Nomad running, visit [http://localhost:4646](http://localhost:4646) to open the Nomad UI.

[![Nomad UI Jobs List][img-jobs-list]][img-jobs-list]

If you can't connect it's possible that Vagrant was unable to properly map the
port from your host to the VM. Your `vagrant up` output will contain the new
port mapping:

```text
==> default: Fixed port collision for 4646 => 4646. Now on port 2200.
```

In the case above you would connect to
[http://localhost:2200](http://localhost:2200) instead.

## Inspecting a Job

You should be automatically redirected to `/ui/jobs` upon visiting the UI in your browser. This
pages lists all jobs known to Nomad, regardless of status. Click the `example` job to inspect it.

[![Nomad UI Job Detail][img-job-detail]][img-job-detail]

The job detail page shows pertinent information about the job, including overall status as well as
allocation statuses broken down by task group. It is similar to the `nomad status` CLI command.

Click on the `cache` task group to drill into the task group detail page. This page lists each allocation
for the task group.

[![Nomad UI Task Group Detail][img-task-group-detail]][img-task-group-detail]

Click on the allocation in the allocations table. This page lists all tasks for an allocation as well
as the recent events for each task. It is similar to the `nomad alloc status` command.

[![Nomad UI Alloc Status][img-alloc-status]][img-alloc-status]

The Nomad UI offers a friendly and visual alternative experience to the CLI.

## Next Steps

We've now concluded the getting started guide, however there are a number
of [next steps](next-steps.html) to get started with Nomad.

[img-jobs-list]: /assets/images/intro-ui-jobs-list.png
[img-job-detail]: /assets/images/intro-ui-job-detail.png
[img-task-group-detail]: /assets/images/intro-ui-task-group-detail.png
[img-alloc-status]: /assets/images/intro-ui-alloc-status.png
