---
layout: "docs"
page_title: "Rolling Upgrades - Operating a Job"
sidebar_current: "docs-operating-a-job-updating-rolling-upgrades"
description: |-
  In order to update a service while reducing downtime, Nomad provides a
  built-in mechanism for rolling upgrades. Rolling upgrades incrementally
  transistions jobs between versions and using health check information to
  reduce downtime.
---

# Rolling Upgrades

Nomad supports rolling updates as a first class feature. To enable rolling
updates a job or task group is annotated with a high-level description of the
update strategy using the [`update` stanza][update]. Under the hood, Nomad
handles limiting parallelism, interfacing with Consul to determine service
health and even automatically reverting to an older, healthy job when a
deployment fails.  

## Enabling Rolling Updates

Rolling updates are enabled by adding the [`update` stanza][update] to the job
specification. The `update` stanza may be placed at the job level or in an
individual task group. When placed at the job level, the update strategy is
inherited by all task groups in the job. When placed at both the job and group
level, the 'update` stanzas are merged, with group stanzas taking precedance
over job level stanzas. See the [`update` stanza
documentation](/docs/job-specification/update.html#upgrade-stanza-inheritance)
for an example.

```hcl
job "geo-api-server" {
  # ...

  group "api-server" {
    count = 6

    # Add an update stanza to enable rolling updates of the service
    update {
      max_parallel = 2
      min_healthy_time = "30s"
      healthy_deadline = "10m"
    }

    task "server" {
      driver = "docker"

      config {
        image = "geo-api-server:0.1"
      }

      # ...
    }
  }
}
```

In this example, by adding the simple `update` stanza to the "api-server" task
group, we inform Nomad that updates to the group should be handled with a
rolling update strategy.

Thus when a change is made to the job file that requires new allocations to be
made, Nomad will deploy 2 allocations at a time and require that the allocations
running in a healthy state for 30 seconds before deploying more versions of the
new group.

By default Nomad determines allocation health by ensuring that all tasks in the
group are running and that any [service
check](/docs/job-specification/service.html#check-parameters) the tasks register
are passing.

## Planning Changes

Suppose we make a change to a file to upgrade the version of a Docker container
that is configured with the same rolling update strategy from above.

```diff
@@ -2,6 +2,8 @@ job "geo-api-server" {
   group "api-server" {
     task "server" {
       driver = "docker"

       config {
-        image = "geo-api-server:0.1"
+        image = "geo-api-server:0.2"
```

The [`nomad plan` command](/docs/commands/plan.html) allows
us to visualize the series of steps the scheduler would perform. We can analyze
this output to confirm it is correct:

```text
$ nomad plan geo-api-server.nomad
```

Here is some sample output:

```text
+/- Job: "geo-api-server"
+/- Task Group: "api-server" (2 create/destroy update, 4 ignore)
  +/- Task: "server" (forces create/destroy update)
    +/- Config {
      +/- image: "geo-api-server:0.1" => "geo-api-server:0.2"
    }

Scheduler dry-run:
- All tasks successfully allocated.

Job Modify Index: 7
To submit the job with version verification run:

nomad run -check-index 7 my-web.nomad

When running the job with the check-index flag, the job will only be run if the
server side version matches the job modify index returned. If the index has
changed, another user has modified the job and the plan's results are
potentially invalid.
```

Here we can see that Nomad will begin a rolling update by creating and
destroying 2 allocations first and for the time being ignoring 4 of the old
allocations, matching our configured `max_parallel`.

## Inspecting a Deployment

After running the plan we can submit the updated job by simply running `nomad
run`. Once run, Nomad will begin the rolling upgrade of our service by placing
2 allocations at a time of the new job and taking two of the old jobs down.

We can inspect the current state of a rolling deployment using `nomad status`:

```text
$ nomad status geo-api-server
ID            = geo-api-server
Name          = geo-api-server
Submit Date   = 07/26/17 18:08:56 UTC
Type          = service
Priority      = 50
Datacenters   = dc1
Status        = running
Periodic      = false
Parameterized = false

Summary
Task Group  Queued  Starting  Running  Failed  Complete  Lost
api-server  0       0         6        0       4         0

Latest Deployment
ID          = c5b34665
Status      = running
Description = Deployment is running

Deployed
Task Group  Desired  Placed  Healthy  Unhealthy
api-server  6        4       2        0

Allocations
ID        Node ID   Task Group  Version  Desired  Status    Created At
14d288e8  f7b1ee08  api-server  1        run      running   07/26/17 18:09:17 UTC
a134f73c  f7b1ee08  api-server  1        run      running   07/26/17 18:09:17 UTC
a2574bb6  f7b1ee08  api-server  1        run      running   07/26/17 18:08:56 UTC
496e7aa2  f7b1ee08  api-server  1        run      running   07/26/17 18:08:56 UTC
9fc96fcc  f7b1ee08  api-server  0        run      running   07/26/17 18:04:30 UTC
2521c47a  f7b1ee08  api-server  0        run      running   07/26/17 18:04:30 UTC
6b794fcb  f7b1ee08  api-server  0        stop     complete  07/26/17 18:04:30 UTC
9bc11bd7  f7b1ee08  api-server  0        stop     complete  07/26/17 18:04:30 UTC
691eea24  f7b1ee08  api-server  0        stop     complete  07/26/17 18:04:30 UTC
af115865  f7b1ee08  api-server  0        stop     complete  07/26/17 18:04:30 UTC
```

Here we can see that Nomad has created a deployment to conduct the rolling
upgrade from job version 0 to 1 and has placed 4 instances of the new job and
has stopped 4 of the old instances. If we look at the deployed allocations, we
also can see that Nomad has placed 4 instances of job version 1 but only
considers 2 of them healthy. This is because the 2 newest placed allocations
haven't been healthy for the required 30 seconds yet.

If we wait for the deployment to complete and re-issue the command, we get the
following:

```text
$ nomad status geo-api-server
ID            = geo-api-server
Name          = geo-api-server
Submit Date   = 07/26/17 18:08:56 UTC
Type          = service
Priority      = 50
Datacenters   = dc1
Status        = running
Periodic      = false
Parameterized = false

Summary
Task Group  Queued  Starting  Running  Failed  Complete  Lost
cache       0       0         6        0       6         0

Latest Deployment
ID          = c5b34665
Status      = successful
Description = Deployment completed successfully

Deployed
Task Group  Desired  Placed  Healthy  Unhealthy
cache       6        6       6        0

Allocations
ID        Node ID   Task Group  Version  Desired  Status    Created At
d42a1656  f7b1ee08  api-server  1        run      running   07/26/17 18:10:10 UTC
401daaf9  f7b1ee08  api-server  1        run      running   07/26/17 18:10:00 UTC
14d288e8  f7b1ee08  api-server  1        run      running   07/26/17 18:09:17 UTC
a134f73c  f7b1ee08  api-server  1        run      running   07/26/17 18:09:17 UTC
a2574bb6  f7b1ee08  api-server  1        run      running   07/26/17 18:08:56 UTC
496e7aa2  f7b1ee08  api-server  1        run      running   07/26/17 18:08:56 UTC
9fc96fcc  f7b1ee08  api-server  0        stop     complete  07/26/17 18:04:30 UTC
2521c47a  f7b1ee08  api-server  0        stop     complete  07/26/17 18:04:30 UTC
6b794fcb  f7b1ee08  api-server  0        stop     complete  07/26/17 18:04:30 UTC
9bc11bd7  f7b1ee08  api-server  0        stop     complete  07/26/17 18:04:30 UTC
691eea24  f7b1ee08  api-server  0        stop     complete  07/26/17 18:04:30 UTC
af115865  f7b1ee08  api-server  0        stop     complete  07/26/17 18:04:30 UTC
```

Nomad has successfully transitioned the group to running the updated canary and
did so with no downtime to our service by ensuring only two allocations were
changed at a time and the newly placed allocations ran successfully. Had any of
the newly placed allocations failed their health check, Nomad would have aborted
the deployment and stopped placing new allocations. If configured, Nomad can
automatically revert back to the old job definition when the deployment fails.

## Auto Reverting on Failed Deployments

In the case we do a deployment in which the new allocations are unhealthy, Nomad
will fail the deployment and stop placing new instances of the job. It
optionally supports automatically reverting back to the last stable job version
on deployment failure. Nomad keeps a history of submitted jobs and whether the
job version was stable.  A job is considered stable if all its allocations are
healthy.

To enable this we simply add the `auto_revert` parameter to the `update` stanza:

```
update {
  max_parallel = 2
  min_healthy_time = "30s"
  healthy_deadline = "10m"

  # Enable automatically reverting to the last stable job on a failed
  # deployment. 
  auto_revert = true
}
```

Now imagine we want to update our image to "geo-api-server:0.3" but we instead
update it to the below and run the job:

```diff
@@ -2,6 +2,8 @@ job "geo-api-server" {
   group "api-server" {
     task "server" {
       driver = "docker"

       config {
-        image = "geo-api-server:0.2"
+        image = "geo-api-server:0.33"
```

If we run `nomad job deployments` we can see that the deployment fails and Nomad
auto-reverts to the last stable job:

```text
$ nomad job deployments geo-api-server
ID        Job ID          Job Version  Status      Description
0c6f87a5  geo-api-server  3            successful  Deployment completed successfully
b1712b7f  geo-api-server  2            failed      Failed due to unhealthy allocations - rolling back to job version 1
3eee83ce  geo-api-server  1            successful  Deployment completed successfully
72813fcf  geo-api-server  0            successful  Deployment completed successfully
```

Nomad job versions increment monotonically, so even though Nomad reverted to the
job specification at version 1, it creates a new job version. We can see the
differences between a jobs versions and how Nomad auto-reverted the job using
the `job history` command:

```text
$ nomad job history -p geo-api-server
Version     = 3
Stable      = true
Submit Date = 07/26/17 18:44:18 UTC
Diff        =
+/- Job: "geo-api-server"
+/- Task Group: "api-server"
  +/- Task: "server"
    +/- Config {
      +/- image: "geo-api-server:0.33" => "geo-api-server:0.2"
        }

Version     = 2
Stable      = false
Submit Date = 07/26/17 18:45:21 UTC
Diff        =
+/- Job: "geo-api-server"
+/- Task Group: "api-server"
  +/- Task: "server"
    +/- Config {
      +/- image: "geo-api-server:0.2" => "geo-api-server:0.33"
        }

Version     = 1
Stable      = true
Submit Date = 07/26/17 18:44:18 UTC
Diff        =
+/- Job: "geo-api-server"
+/- Task Group: "api-server"
  +/- Task: "server"
    +/- Config {
      +/- image: "geo-api-server:0.1" => "geo-api-server:0.2"
        }

Version     = 0
Stable      = true
Submit Date = 07/26/17 18:43:43 UTC
```

We can see that Nomad considered the job running "geo-api-server:0.1" and
"geo-api-server:0.2" as stable but job Version 2 that submitted the incorrect
image is marked as unstable. This is because the placed allocations failed to
start. Nomad detected the deployment failed and as such, created job Version 3
that reverted back to the last healthy job.

[update]: /docs/job-specification/update.html "Nomad update Stanza"
