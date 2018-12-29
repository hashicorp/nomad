---
layout: "guides"
page_title: "Web UI"
sidebar_current: "guides-ui"
description: |-
  The Nomad Web UI is a great companion for both operators and developers.
  It is an easy to use way to inspect jobs, allocations, and nodes.
---

# Web UI

The Nomad Web UI offers an easy to use web experience for inspecting a Nomad cluster.
Jobs, Deployments, Evaluations, Task Groups, Allocations, Logs, Clients, and Servers can all be
monitored from the Web UI. The Web UI also supports the use of ACL tokens for
clusters that are using the [ACL system](/guides/security/acl.html).

## Accessing the Web UI

The Web UI is served on the same address and port as the HTTP API. It is namespaced
under `/ui`, but visiting the root of the Nomad server in your browser will redirect you
to the Web UI. If you are unsure what port the Nomad HTTP API is running under, try the default
port, `4646`.

~> **Live Demo!** For a quick test drive, see our online Web UI demo at [demo.nomadproject.io](https://demo.nomadproject.io).

## Reviewing Jobs

The home page of the Web UI is the jobs list view. This page has a searchable, sortable,
paginated table of all jobs in the cluster, regardless of job status. This page will automatically
update as jobs are started, stopped, and allocations transition through states.

[![Jobs List][img-jobs-list]][img-jobs-list]

To sort the table, click a table column's header cell. To search, type a query into the searchbox.
By default the search will fuzzy-match any job name, but by starting a query with a `/`, the search
will instead be based on a regular expression.

Sort property, sort direction, search term, and page number are all stored as query params to make
sharing links easier.

In addition to name, each job in the table includes details such as current status, type, priority,
number of task groups, and an aggregation of all allocations by allocation status.

[![Job Row Detail][img-jobs-row-detail]][img-jobs-row-detail]

## Inspecting a Job

Clicking on a job will navigate to the Job Detail page. This page shows a list of task groups
for the job as well as the status of each allocation for each task group by allocation status.

This page will automatically update as deployments start and finish, new versions fail to be placed,
allocations transition through states, and evaluations update.

[![Job Detail][img-job-detail]][img-job-detail]

### Reading a Job Definition

The Job Detail page has multiple tabs, one of which is Definition. On the Definition page, the full
job definition is shown as an interactive JSON object.

[![Job Definition][img-job-definition]][img-job-definition]

### Reviewing Past Job Versions

Job Versions can be found on the Versions tab on the Job Detail page. This page has a timeline view of
every tracked version for the job.

This page will automatically update as new versions are submitted to the cluster.

[![Job Versions][img-job-versions]][img-job-versions]

Each version can be toggled to also show the diff between job versions.

[![Job Version Diff][img-job-version-diff]][img-job-version-diff]

### Reviewing Past Job Deployments

Job Deployments are listed on the Deployments tab on the Job Detail page. Every tracked deployment is listed in
a timeline view.

This page will automatically update as new versions are submitted to the cluster and deployments proceed.

[![Job Deployments][img-job-deployments]][img-job-deployments]

Each deployment can be toggle to show information about the deployment, including canaries placed and desired,
allocations placed and desired, healthy and unhealthy allocations, task group metrics, and existing allocations.

[![Job Deployment Detail][img-job-deployment-detail]][img-job-deployment-detail]

### Monitoring a Current Job Deployment

When a deployment is currently running, it is called out on the Job Detail Overview tab. It will update
automatically as canaries and allocations are placed and deemed healthy or unhealthy.

[![Active Job Deployment][img-active-job-deployment]][img-active-job-deployment]

### Inspecting Child Jobs

Child jobs are only listed on the parent periodic or parameterized job detail pages. This leaves the global
jobs list view uncluttered.

[![Periodic Job Detail Showing Children Jobs][img-periodic-job-detail]][img-periodic-job-detail]

### Forcing a Periodic Launch

From the job detail page for a periodic job, a child instance can be launched on demand rather than waiting
the scheduled amount of time as defined by the cron in the job definition.

[![Periodic Force Launch Button][img-periodic-force-launch-button]][img-periodic-force-launch-button]

### Inspecting a Dispatch Payload

When a parameterized job is dispatched, the payload is captured and accessible from the bottom of the
parameterized child job detail page. The Web UI will attempt to parse the payload as JSON for improved
inspecting capabilities.

[![Parameterized Dispatch Payload][img-parameterized-dispatch-payload]][img-parameterized-dispatch-payload]

## Inspecting a Task Group

Clicking on a task group from the Job Detail page will navigate to the Task Group Detail page. This page shows
the aggregated resource metrics for a task group as well as a list of all associated allocations.

This page will automatically update as the allocations in the task group change states.

[![Task Group Detail][img-task-group-detail]][img-task-group-detail]

## Inspecting an Allocation

From the Task Group Detail page, each allocation in the allocations table will report basic information about
the allocation, including utilized CPU and memory.

Each allocation will poll for updates to current stats.

[![Allocation Stats][img-allocation-stats]][img-allocation-stats]

Clicking an allocation will navigate to the Allocation Detail page. From here, the event history for each task
in the allocation can be seen.

This page will automatically update as new task events are emitted.

[![Allocation Detail][img-allocation-detail]][img-allocation-detail]

## Reviewing Logs

Clicking on a task from the allocation detail will navigate to the Task Detail page. The Task Detail page includes
a list of recent events as well as a tab for logs.

On the Logs tab, anything the task writes to `stdout` or `stderr` is accessible. The log stream component allows
for pausing and playing log streaming, jumping to the head or tail of the log, and toggling between `stdout` and
`stderr`.

~> **Note.** The Web UI will first attempt to stream logs directly from the client agent to avoid unnecessary load
~> on server agents. If the client is not accessible, the Web UI will fallback to using the server for convenience.

[![Task Logs][img-task-logs]][img-task-logs]

~> **Note.** Only `stdout` and `stderr` are available in the UI. Any logs written to a file are available from the
~> CLI using the [`nomad alloc fs`](/docs/commands/alloc/fs.html), but filesystem access is not yet in the Web UI.

## Reviewing Clients

Clicking the Clients link in the left-hand menu of the Web UI will navigate to the Clients List page. This page
has a searchable, sortable, paginated table of all clients in the cluster.

Sort property, sort direction, search term, and page number are all stored as query params to make
sharing links easier.

In addition to name, each client in the table includes details such as current status, address, datacenter,
and number of allocations.

This page will automatically update as nodes change states and work is allocated.

[![Clients List][img-clients-list]][img-clients-list]

## Inspecting a Client

Clicking on a client will navigate to the Client Detail page. This page shows the status of the client as
well as the list of all allocations placed on the client. Additionally, all attributes of the machine are
itemized.

This page will automatically update as allocations come and go on the client and the usage stats for an
allocation change.

[![Client Detail][img-client-detail]][img-client-detail]

## Inspecting Servers

Clicking on the Servers link in the left-hand menu of the Web UI will navigate to the Servers List page. This
page lists all servers, including which one is the current leader.

Clicking on a server in the list will open up a table that lists the Tags for the server.

[![Server Detail][img-server-detail]][img-server-detail]

## Using an ACL Token

When the ACL system is enabled for the cluster, tokens can be used to gain elevated permissions to see
otherwise private jobs, nodes, and other details. To register a token with the Web UI, click ACL Tokens on the
right-hand side of the top navigation.

The ACL Tokens page has a two field form for providing a token Secret ID and token Accessor ID. The form
automatically updates as the values change, and once a Secret ID is provided, all future HTTP requests the
Web UI makes will provide the Secret ID as the ACL Token via the `X-Nomad-Token` request header.

[![ACL Tokens][img-acl-tokens]][img-acl-tokens]

[img-jobs-list]: /assets/images/guide-ui-jobs-list.png
[img-jobs-row-detail]: /assets/images/guide-ui-jobs-row-detail.png
[img-job-detail]: /assets/images/guide-ui-job-detail.png
[img-job-definition]: /assets/images/guide-ui-job-definition.png
[img-job-versions]: /assets/images/guide-ui-job-versions.png
[img-job-version-diff]: /assets/images/guide-ui-job-version-diff.png
[img-job-deployments]: /assets/images/guide-ui-job-deployments.png
[img-job-deployment-detail]: /assets/images/guide-ui-job-deployment-detail.png
[img-active-job-deployment]: /assets/images/guide-ui-active-job-deployment.png
[img-task-group-detail]: /assets/images/guide-ui-task-group-detail.png
[img-allocation-stats]: /assets/images/guide-ui-allocation-stats.png
[img-allocation-detail]: /assets/images/guide-ui-allocation-detail.png
[img-clients-list]: /assets/images/guide-ui-clients-list.png
[img-client-detail]: /assets/images/guide-ui-client-detail.png
[img-server-detail]: /assets/images/guide-ui-server-detail.png
[img-acl-tokens]: /assets/images/guide-ui-acl-tokens.png
[img-task-logs]: /assets/images/guide-ui-task-logs.png
[img-parameterized-dispatch-payload]: /assets/images/guide-ui-parameterized-dispatch-payload.png
[img-periodic-force-launch-button]: /assets/images/guide-ui-periodic-force-launch-button.png
[img-periodic-job-detail]: /assets/images/guide-ui-periodic-job-detail.png
