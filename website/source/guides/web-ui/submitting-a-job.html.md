---
layout: "guides"
page_title: "Submitting a Job from the Web UI"
sidebar_current: "guides-web-ui-submitting-a-job"
description: |-
  Learn how to submit a job from the Web UI.
---

# Submitting a Job

On the Jobs List page of the Web UI (this is the home page), there is a "Run Job" button in the
top-left corner. Clicking this button will take you to the Job Run page.

The first step in running a job is authoring the job HCL or JSON. Code can be authored directly in
the UI, complete with syntax highlighting, or it can be pasted in. After you have authored the job,
the next step is to run the plan.

[![Job Run Page][img-job-run]][img-job-run]

## Nomad plan

It is best practice to run `nomad plan` before running `nomad run`, so the Web UI enforces this
best practice. From the Job Run page, underneath the code editor, there is a Plan button. Clicking
this button will proceed the run process to the second step.

The second step to submitting a job is reviewing the job plan. If you are submitting a new job, the
plan will only show additions. If you are submitting a new version of the job, the plan will include
details on what has been changed, added, and removed.

[![Job Plan Page][img-job-plan]][img-job-plan]

The plan operation will also perform a scheduler dry-run. This dry-run is helpful for catching
potential issues early. Some potential issues are:

1. There is not enough capacity in the cluster to start your job.
2. There is not enough capacity remaining in your quota to start your job.
3. Your job has an unresolvable hard constraint (e.g., required port not available).
4. In order to start your job, other jobs must be preempted.

[![Job Plan Placement Failures][img-job-plan-placement-failures]][img-job-plan-placement-failures]

From the plan step, you can either cancel to make edits, or run the job. When you run the job, you
are redirected to the Job Overview page.

## Placement failures

One class of potential issues when planning a job is a placement failure. This happens when Nomad
can tell ahead of time that a job cannot be started. Since Nomad does bookkeeping on cluster state
and node metadata, Nomad will already know the answer to basic constraints, such as available
capacity, available hardware, and available ports.

Nomad will always let you submit a job to the cluster despite placement failures. The job will just
remain in a queued state until the placement failures are resolved.

Keep in mind that there will always be the possibility that Nomad cannot start a job despite there
being no placement failures (e.g., artifact cannot download or container startup script errors).

## Preemptions

Another class of potential issues when planning a job is
[preemptions](/docs/internals/scheduling/preemption.html). This happens when the cluster does not
have capacity for your job, but your job is a high priority and the cluster has preemptions enabled.

[![Job Plan Preemptions][img-job-plan-preemptions]][img-job-plan-preemptions]

Unlike with placement failures, when you submit a job that has expected preemptions, the job will
start. However, other allocations will be stopped to free up capacity.

~> With Nomad OSS, only system jobs can preempt allocations. Nomad Enterprise allows for both
service and batch type jobs to preempt lower priority allocations.

## Job Overview

Upon submitting a job, you will be redirected to the Job Overview page for the job you submitted.

If this is a new job, the job will start in a queued state. If there are no placement failures,
allocations for the job will naturally transition from a starting to a running or failed state.
Nomad is quick to schedule allocations (i.e., find a client node to start the allocation on), but an
allocation may sit in the starting state for a while if it has to download
[source images](/docs/job-specification/task.html#task-examples) or
[other artifacts](/docs/job-specification/artifact.html). It may also sit in a starting state if the
task fails to start and requires retry attempts.

If this is was an existing job that was resubmitted, the job overview will show old allocations
moving into a completed status before new allocations are spun up. The exact sequence of events
depends on the configuration of the job.

No matter the configuration of the job, the Job Overview page will live-update as the state of the
job and its allocations change.

[![Job Overview][img-job-overview]][img-job-overview]

## Job Definition

From the subnav on any job detail page, you can access the Job Definition page.

The Job Definition page will show the job's underlying JSON representation. This can be useful for
quickly verifying how the job was configured. Many properties from the job configuration will also
be on the Job Overview page, but some deeper properties may only be available in the definition
itself. It can also be convenient to see everything at once rather than traversing through task
groups, allocations, and tasks.

[![Job Definition][img-job-definition]][img-job-definition]

## Job Versions

From the subnav on any job detail page, you can access the Job Versions page.

The Job Versions page will show a timeline view of every version of the job. Each version in the
timeline includes the version number, the time the version was submitted, whether the version is/was
stable, the number of changes, and the job diff.

Reviewing the job diffs version by version can be used to debug issues in a similar manner to `git log`.

[![Jobs Versions][img-job-versions]][img-job-versions]

## Job Deployments

From the subnav on any service job detail page, you can access the Job Deployments page.

The Job Deployments page will show a timeline view of every deployment of the job. Each deployment
in the timeline includes the deployment ID, the deployment status, whether or not the deployment
requires promotion, the associated version number, the relative time the deployment started, and a
detailed allocation breakdown.

The allocation breakdown includes information on allocation placement, including how many canaries
have been placed, how many canaries are expected, how many total allocations have been placed, how
many total allocations are desired, and the health of each allocation.

[![Jobs Deployments][img-job-deployments]][img-job-deployments]

## Job Allocations

From the subnav on any job detail page, you can access the Job Allocations page.

The Job Allocations page will show a complete table of every allocation for a job. Allocations,
being the unit of work in Nomad, are accessible from many places. The Job Overview page lists some
of the recent allocations for a job for convenience and the Job Task Group page will list all
allocations for that task group, but only the Job Allocations page shows every allocation across all
task groups for the job.

[![Jobs Allocations][img-job-allocations]][img-job-allocations]

## Job Evaluations

From the subnav on any job detail page, you can access the Job Evaluations page.

The Job Evaluations page will show the most recent evaluations for the job. Evaluations are an
internal detail of Nomad's inner scheduling process and as such are generally unimportant to
monitor, but an experienced Nomad user can use evaluations to diagnose potential issues.

[![Job Evaluations][img-job-evaluations]][img-job-evaluations]

## Access Control

Depending on the size of your team and the details of your Nomad deployment, you may wish to control
which features different internal users have access to. You can enforce this with Nomad's access
control list system.

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

[img-job-run]: /assets/images/guide-ui-img-job-run.png
[img-job-plan]: /assets/images/guide-ui-img-job-plan.png
[img-job-plan-placement-failures]: /assets/images/guide-ui-img-job-plan-placement-failures.png
[img-job-plan-preemptions]: /assets/images/guide-ui-img-job-plan-preemptions.png
[img-job-overview]: /assets/images/guide-ui-img-job-overview-system.png
[img-job-definition]: /assets/images/guide-ui-img-job-definition.png
[img-job-versions]: /assets/images/guide-ui-img-job-versions.png
[img-job-deployments]: /assets/images/guide-ui-img-job-deployments.png
[img-job-allocations]: /assets/images/guide-ui-img-job-allocations.png
[img-job-evaluations]: /assets/images/guide-ui-img-job-evaluations.png
