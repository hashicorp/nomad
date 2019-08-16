---
layout: "guides"
page_title: "Operating a Job from the Web UI"
sidebar_current: "guides-web-ui-operating-a-job"
description: |-
  Learn how to operate a job from the Web UI.
---

# Reviewing Job and Allocation Status

The Web UI can be a powerful companion when monitoring and debugging jobs running in Nomad. The Web
UI will list all jobs, link jobs to allocations, allocations to client nodes, client nodes to driver
health, and much more. This creates a fluid exploratory experience.

## Reviewing All Jobs

The first page you will see in the Web UI is the Jobs List page. Here you will find every job for a
namespace in a region. The table of jobs is searchable, sortable, and filterable. Each job row in
the table shows basic information, such as job name, status, type, and priority, as well as richer
information such as a visual representation of all allocation statuses.

This view will also live-update as jobs get submitted, get purged, and change status.

~> Screenshot (Jobs Overview)

## Filtering Jobs

If your Nomad cluster has many jobs, it can be useful to filter the list of all jobs down to only
those matching certain facets. The Web UI has four facets you can filter by:

1. **Type:** The type of job, including Batch, Parameterized, Periodic, Service, and System.
2. **Status:** The status of the job, including Pending, Running, and Dead.
3. **Datacenter:** The datacenter the job is running in, including a dynamically generated list
   based on the jobs in the namespace.
4. **Prefix:** The possible common naming prefix for a job, including a dynamically generated list
   based on job names up to the first occurrence of `-`, `.`, and `_`. Only prefixes that match
   multiple jobs are included.

~> Screenshot (Zoom in on job filters, with one open)

## Monitoring an Allocation

In Nomad, allocations are the schedulable units of work. This is where runtime metrics begin to
surface. An allocation is composed of one or more tasks, and the utilization metrics for tasks are
aggregated so they can be observed at the allocation level.

### Resource Utilization

Nomad has APIs for reading point-in-time resource utilization metrics for tasks and allocations. The
Web UI uses these metrics to create time-series graphs for the current session.

When viewing an allocation, resource utilization will automatically start logging.

~> Screenshot (resource tuilization)

### Task Events

When Nomad places, prepares, and starts a task, a series of task events are emitted to help debug
issues in the event that the task fails to start.

Task events are listed on the Task Detail page and live-update as Nomad handles managing the task.

~> Screenshot (task events)

### Rescheduled Allocations

Allocations will be placed on any client node that satisfies the constraints of the job definition.
However, just because a node sounds like a good fit doesn't mean there isn't the possibility of
unforeseen hostility, (e.g., corrupted `/bin`, no access to a container registry).

Allocations can be configured [in the job definition to reschedule](/docs/job-specification/reschedule.html)
to a different client node if the allocation ends in a failed status. This will happen after the
task has exhausted its [local restart attempts](/docs/job-specification/restart.html).

The end result of this automatic procedure is a failed allocation and that failed allocation's
rescheduled successor. Since Nomad handles all of this automatically, the Web UI makes sure to
explain the state of allocations through iconography and linking previous and next allocations in a
reschedule chain.

~> Screenshot (reschedule icon)

~> Screenshot (reschedule section on alloc detail)

### Unhealthy Driver

Given the nature of long-lived processes, it's possible for the state of the client node an
allocation is scheduled on to change during the lifespan of the allocation. Nomad attempts to
monitor pertinent conditions including driver health.

The Web UI denotes when a driver an allocation depends on is unhealthy on the client node the
allocation is running on.

~> Screenshot (unhealthy driver)

### Preempted Allocations

Much like how Nomad will automatically reschedule allocations, Nomad will automatically preempt
allocations when necessary. When monitoring allocations in Nomad, it's useful to know what
allocations were preempted and what job caused the preemption.

The Web UI makes sure to tell this full story by showing which allocation caused an allocation to be
preempted as well as the opposite: what allocations an allocation preempted. This makes it possible
to traverse down from a job to a preempted allocation, to the allocation that caused the preemption,
to the job that the preempting allocation is for.

~> Screenshot (preempter)

~> Screenshot (preemptions)

## Reviewing Logs for a Task

A task will typically emit log information to `stdout` and `stderr`. Nomad captures these logs and
exposes them through an API. The Web UI uses these APIs to offer `head`, `tail`, and streaming logs
from the browser.

The Web UI will first attempt to directly connecto to the client node the task is running on.
Typically, client nodes are not accessible from the public internet. If this is the case, the Web UI
will fall back and proxy to the client node from the server node with no loss of functionality.

~> Screenshot (task logs)

~> Not all browsers support streaming http requests. In the event that streaming is not supported,
logs will still be followed using interval polling.

## Restarting or Stopping an Allocation or Task

Ideally software always runs smoothly and as intended, but this isn't something we can count on.
Sometimes tasks will have a memory leak, sometimes a node will have noisy neighbors, and sometimes
we have no clue what's going so it's time to try turning it off and on again.

For these times, Nomad allows for restarting and stopping individual allocations and tasks. When a
task is restarted, Nomad will perform a local restart of the task. When an allocation is stopped,
Nomad will mark the allocation as complete and perform a reschedule onto a different client node.

Both of these features are also available in the Web UI.

~> Screenshot (allocation stop and restart)

## Forcing a Periodic Instance

Periodic jobs are configured like a cron job. Sometimes, we want to micromanage the job instead of
waiting for the period duration to elapse. Nomad calls this a
[periodic force](/docs/commands/job/periodic-force.html) and it can be done from the Web UI on the
Job Overview page for a periodic job.

~> Screenshot (periodic force)

## Submitting a New Version of a Job

From the Job Definition page, a job can be edited. After clicking the Edit button in the top-right
corner of the code window, the job definition JSON becomes editable. The edits can then be planned
and scheduled.

~> Screenshot (definition edit)

~> Since each job within a namespace must have a unique name, it is possible to submit a new version
of a job from the Run Job screen. Always review the plan output!

## Monitoring a Deployment

When a system or service job includes the [`update` stanza](/docs/job-specification/update.html), a
deployment is created upon job submission. Job deployments can be monitored in realtime from the Web
UI.

The Web UI will show as new allocations become placed, tallying towards the expected total, and
tally allocations as they becme healthy or unhealthy.

Optionally, a job may use canary deployments to allow for additional health checks or manual testing
before a full roll out. If a job uses canaries and is not configured to automatically promote the
canary, the canary promotion operation can be done from the Job Overview page in the Web UI.

~> Screenshot (job deployment with canary link)

## Stopping a Job

Jobs can be stopped from the Job Overview page. Stopping a job will gracefully stop all allocations,
marking them as complete, and freeing up resources in the cluster.

~> Screenshot (job stop)

## Access Control

Depending on the size of your team and the details of your Nomad deployment, you may wish to control
which features different internal users have access to. This includes differentiation between
submitting jobs, restarting allocations, and viewing potentially sensitive logs.

Nomad has an access control list system for doing just that.

By default, all features—read and write—are available to all users of the Web UI. Check out the
[Securing the Web UI with ACLs](/guides/web-ui/securing.html) guide to learn how to prevent
anonymous users from having write permissions as well as how to continue to use Web UI write
features as a privileged user.

## Best Practices

Although the Web UI lets users submit jobs in an ad-hoc manner, Nomad was deliberately designed to
declare jobs using a configuration language. It is recommended to treat your job definitions, like
the rest of your infrastructure, as code.

By checking in your job definition files as source control, you will always have a log of changes to
assist in debugging issues, rolling back versions, and collaborating on changes using development
best practices like code review.
