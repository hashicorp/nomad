---
layout: "docs"
page_title: "Blue/Green & Canary Deployments - Operating a Job"
sidebar_current: "docs-operating-a-job-updating-blue-green-deployments"
description: |-
  Nomad has built-in support for doing blue/green and canary deployments to more
  safely update existing applications and services.
---

# Blue/Green &amp; Canary Deployments

Sometimes [rolling
upgrades](/docs/operating-a-job/update-strategies/rolling-upgrades.html) do not
offer the required flexibility for updating an application in production. Often
organizations prefer to put a "canary" build into production or utilize a
technique known as a "blue/green" deployment to ensure a safe application
rollout to production while minimizing downtime.

## Blue/Green Deployments

Blue/Green deployments have several other names including Red/Black or A/B, but
the concept is generally the same. In a blue/green deployment, there are two
application versions. Only one application version is active at a time, except
during the transition phase from one version to the next. The term "active"
tends to mean "receiving traffic" or "in service".

Imagine a hypothetical API server which has five instances deployed to
production at version 1.3, and we want to safely upgrade to version 1.4. We want
to create five new instances at version 1.4 and in the case that they are
operating correctly we want to promote them and take down the five versions
running 1.3. In the event of failure, we can quickly rollback to 1.3.

To start, we examine our job which is running in production:

```hcl
job "docs" {
  # ...

  group "api" {
    count = 5

    update {
      max_parallel     = 1
      canary           = 5
      min_healthy_time = "30s"
      healthy_deadline = "10m"
      auto_revert      = true
    }

    task "api-server" {
      driver = "docker"

      config {
        image = "api-server:1.3"
      }
    }
  }
}
```

We see that it has an `update` stanza that has the `canary` equal to the desired
count. This is what allows us to easily model blue/green deployments. When we
change the job to run the "api-server:1.4" image, Nomad will create 5 new
allocations without touching the original "api-server:1.3" allocations. Below we
can see how this works by changing the image to run the new version:

```diff
@@ -2,6 +2,8 @@ job "docs" {
  group "api" {
    task "api-server" {
      config {
-       image = "api-server:1.3"
+       image = "api-server:1.4"
```

Next we plan and run these changes:

```text
$ nomad plan docs.nomad
+/- Job: "docs"
+/- Task Group: "api" (5 canary, 5 ignore)
  +/- Task: "api-server" (forces create/destroy update)
    +/- Config {
      +/- image: "api-server:1.3" => "api-server:1.4"
        }

Scheduler dry-run:
- All tasks successfully allocated.

Job Modify Index: 7
To submit the job with version verification run:

nomad run -check-index 7 example.nomad

When running the job with the check-index flag, the job will only be run if the
server side version matches the job modify index returned. If the index has
changed, another user has modified the job and the plan's results are
potentially invalid.

$ nomad run docs.nomad
# ...
```

We can see from the plan output that Nomad is going to create 5 canaries that
are running the "api-server:1.4" image and ignore all the allocations running
the older image. Now if we examine the status of the job we can see that both
the blue ("api-server:1.3") and green ("api-server:1.4") set are running.

```text
$ nomad status docs
ID            = docs
Name          = docs
Submit Date   = 07/26/17 19:57:47 UTC
Type          = service
Priority      = 50
Datacenters   = dc1
Status        = running
Periodic      = false
Parameterized = false

Summary
Task Group  Queued  Starting  Running  Failed  Complete  Lost
api         0       0         10       0       0         0

Latest Deployment
ID          = 32a080c1
Status      = running
Description = Deployment is running but requires promotion

Deployed
Task Group  Auto Revert  Promoted  Desired  Canaries  Placed  Healthy  Unhealthy
api         true         false     5        5         5       5        0

Allocations
ID        Node ID   Task Group  Version  Desired  Status   Created At
6d8eec42  087852e2  api         1        run      running  07/26/17 19:57:47 UTC
7051480e  087852e2  api         1        run      running  07/26/17 19:57:47 UTC
36c6610f  087852e2  api         1        run      running  07/26/17 19:57:47 UTC
410ba474  087852e2  api         1        run      running  07/26/17 19:57:47 UTC
85662a7a  087852e2  api         1        run      running  07/26/17 19:57:47 UTC
3ac3fe05  087852e2  api         0        run      running  07/26/17 19:53:56 UTC
4bd51979  087852e2  api         0        run      running  07/26/17 19:53:56 UTC
2998387b  087852e2  api         0        run      running  07/26/17 19:53:56 UTC
35b813ee  087852e2  api         0        run      running  07/26/17 19:53:56 UTC
b53b4289  087852e2  api         0        run      running  07/26/17 19:53:56 UTC
```

Now that we have the new set in production, we can route traffic to it and
validate the new job version is working properly. Based on whether the new
version is functioning properly or improperly we will either want to promote or
fail the deployment.

### Promoting the Deployment

After deploying the new image along side the old version we have determined it
is functioning properly and we want to transistion fully to the new version.
Doing so is as simple as promoting the deployment:

```text
$ nomad deployment promote 32a080c1
==> Monitoring evaluation "61ac2be5"
    Evaluation triggered by job "docs"
    Evaluation within deployment: "32a080c1"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "61ac2be5" finished with status "complete"
```

If we look at the job's status we see that after promotion, Nomad stopped the
older allocations and is only running the new one. This now completes our
blue/green deployment.

```text
$ nomad status docs
ID            = docs
Name          = docs
Submit Date   = 07/26/17 19:57:47 UTC
Type          = service
Priority      = 50
Datacenters   = dc1
Status        = running
Periodic      = false
Parameterized = false

Summary
Task Group  Queued  Starting  Running  Failed  Complete  Lost
api         0       0         5        0       5         0

Latest Deployment
ID          = 32a080c1
Status      = successful
Description = Deployment completed successfully

Deployed
Task Group  Auto Revert  Promoted  Desired  Canaries  Placed  Healthy  Unhealthy
api         true         true      5        5         5       5        0

Allocations
ID        Node ID   Task Group  Version  Desired  Status    Created At
6d8eec42  087852e2  api         1        run      running   07/26/17 19:57:47 UTC
7051480e  087852e2  api         1        run      running   07/26/17 19:57:47 UTC
36c6610f  087852e2  api         1        run      running   07/26/17 19:57:47 UTC
410ba474  087852e2  api         1        run      running   07/26/17 19:57:47 UTC
85662a7a  087852e2  api         1        run      running   07/26/17 19:57:47 UTC
3ac3fe05  087852e2  api         0        stop     complete  07/26/17 19:53:56 UTC
4bd51979  087852e2  api         0        stop     complete  07/26/17 19:53:56 UTC
2998387b  087852e2  api         0        stop     complete  07/26/17 19:53:56 UTC
35b813ee  087852e2  api         0        stop     complete  07/26/17 19:53:56 UTC
b53b4289  087852e2  api         0        stop     complete  07/26/17 19:53:56 UTC
```

### Failing the Deployment

After deploying the new image alongside the old version we have determined it
is not functioning properly and we want to roll back to the old version.  Doing
so is as simple as failing the deployment:

```text
$ nomad deployment fail 32a080c1
Deployment "32a080c1-de5a-a4e7-0218-521d8344c328" failed. Auto-reverted to job version 0.

==> Monitoring evaluation "6840f512"
    Evaluation triggered by job "example"
    Evaluation within deployment: "32a080c1"
    Allocation "0ccb732f" modified: node "36e7a123", group "cache"
    Allocation "64d4f282" modified: node "36e7a123", group "cache"
    Allocation "664e33c7" modified: node "36e7a123", group "cache"
    Allocation "a4cb6a4b" modified: node "36e7a123", group "cache"
    Allocation "fdd73bdd" modified: node "36e7a123", group "cache"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "6840f512" finished with status "complete"
```

If we now look at the job's status we can see that after failing the deployment,
Nomad stopped the new allocations and is only running the old ones and reverted
the working copy of the job back to the original specification running
"api-server:1.3".

```text
$ nomad status docs
ID            = docs
Name          = docs
Submit Date   = 07/26/17 19:57:47 UTC
Type          = service
Priority      = 50
Datacenters   = dc1
Status        = running
Periodic      = false
Parameterized = false

Summary
Task Group  Queued  Starting  Running  Failed  Complete  Lost
api         0       0         5        0       5         0

Latest Deployment
ID          = 6f3f84b3
Status      = successful
Description = Deployment completed successfully

Deployed
Task Group  Auto Revert  Desired  Placed  Healthy  Unhealthy
cache       true         5        5       5        0

Allocations
ID        Node ID   Task Group  Version  Desired  Status    Created At
27dc2a42  36e7a123  api         1        stop     complete  07/26/17 20:07:31 UTC
5b7d34bb  36e7a123  api         1        stop     complete  07/26/17 20:07:31 UTC
983b487d  36e7a123  api         1        stop     complete  07/26/17 20:07:31 UTC
d1cbf45a  36e7a123  api         1        stop     complete  07/26/17 20:07:31 UTC
d6b46def  36e7a123  api         1        stop     complete  07/26/17 20:07:31 UTC
0ccb732f  36e7a123  api         2        run      running   07/26/17 20:06:29 UTC
64d4f282  36e7a123  api         2        run      running   07/26/17 20:06:29 UTC
664e33c7  36e7a123  api         2        run      running   07/26/17 20:06:29 UTC
a4cb6a4b  36e7a123  api         2        run      running   07/26/17 20:06:29 UTC
fdd73bdd  36e7a123  api         2        run      running   07/26/17 20:06:29 UTC

$ nomad job deployments docs
ID        Job ID   Job Version  Status      Description
6f3f84b3  example  2            successful  Deployment completed successfully
32a080c1  example  1            failed      Deployment marked as failed - rolling back to job version 0
c4c16494  example  0            successful  Deployment completed successfully
```

## Canary Deployments

Canary updates are a useful way to test a new version of a job before beginning
a rolling upgrade. The `update` stanza supports setting the number of canaries
the job operator would like Nomad to create when the job changes via the
`canary` parameter. When the job specification is updated, Nomad creates the
canaries without stopping any allocations from the previous job.

This pattern allows operators to achieve higher confidence in the new job
version because they can route traffic, examine logs, etc, to determine the new
application is performing properly.

```hcl
job "docs" {
  # ...

  group "api" {
    count = 5

    update {
      max_parallel     = 1
      canary           = 1
      min_healthy_time = "30s"
      healthy_deadline = "10m"
      auto_revert      = true
    }

    task "api-server" {
      driver = "docker"

      config {
        image = "api-server:1.3"
      }
    }
  }
}
```

In the example above, the `update` stanza tells Nomad to create a single canary
when the job specification is changed. Below we can see how this works by
changing the image to run the new version:

```diff
@@ -2,6 +2,8 @@ job "docs" {
  group "api" {
    task "api-server" {
      config {
-       image = "api-server:1.3"
+       image = "api-server:1.4"
```

Next we plan and run these changes:

```text
$ nomad plan docs.nomad
+/- Job: "docs"
+/- Task Group: "api" (1 canary, 5 ignore)
  +/- Task: "api-server" (forces create/destroy update)
    +/- Config {
      +/- image: "api-server:1.3" => "api-server:1.4"
        }

Scheduler dry-run:
- All tasks successfully allocated.

Job Modify Index: 7
To submit the job with version verification run:

nomad run -check-index 7 example.nomad

When running the job with the check-index flag, the job will only be run if the
server side version matches the job modify index returned. If the index has
changed, another user has modified the job and the plan's results are
potentially invalid.

$ nomad run docs.nomad
# ...
```

We can see from the plan output that Nomad is going to create 1 canary that
will run the "api-server:1.4" image and ignore all the allocations running
the older image. If we inspect the status we see that the canary is running
along side the older version of the job:

```text
$ nomad status docs
ID            = docs
Name          = docs
Submit Date   = 07/26/17 19:57:47 UTC
Type          = service
Priority      = 50
Datacenters   = dc1
Status        = running
Periodic      = false
Parameterized = false

Summary
Task Group  Queued  Starting  Running  Failed  Complete  Lost
api         0       0         6        0       0         0

Latest Deployment
ID          = 32a080c1
Status      = running
Description = Deployment is running but requires promotion

Deployed
Task Group  Auto Revert  Promoted  Desired  Canaries  Placed  Healthy  Unhealthy
api         true         false     5        1         1       1        0

Allocations
ID        Node ID   Task Group  Version  Desired  Status   Created At
85662a7a  087852e2  api         1        run      running  07/26/17 19:57:47 UTC
3ac3fe05  087852e2  api         0        run      running  07/26/17 19:53:56 UTC
4bd51979  087852e2  api         0        run      running  07/26/17 19:53:56 UTC
2998387b  087852e2  api         0        run      running  07/26/17 19:53:56 UTC
35b813ee  087852e2  api         0        run      running  07/26/17 19:53:56 UTC
b53b4289  087852e2  api         0        run      running  07/26/17 19:53:56 UTC
```

Now if we promote the canary, this will trigger a rolling update to replace the
remaining allocations running the older image. The rolling update will happen at
a rate of `max_parallel`, so in this case one allocation at a time:

```text
$ nomad deployment promote 37033151
==> Monitoring evaluation "37033151"
    Evaluation triggered by job "docs"
    Evaluation within deployment: "ed28f6c2"
    Allocation "f5057465" created: node "f6646949", group "cache"
    Allocation "f5057465" status changed: "pending" -> "running"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "37033151" finished with status "complete"

$ nomad status docs
ID            = docs
Name          = docs
Submit Date   = 07/26/17 20:28:59 UTC
Type          = service
Priority      = 50
Datacenters   = dc1
Status        = running
Periodic      = false
Parameterized = false

Summary
Task Group  Queued  Starting  Running  Failed  Complete  Lost
api         0       0         5        0       2         0

Latest Deployment
ID          = ed28f6c2
Status      = running
Description = Deployment is running

Deployed
Task Group  Auto Revert  Promoted  Desired  Canaries  Placed  Healthy  Unhealthy
api         true         true      5        1         2       1        0

Allocations
ID        Node ID   Task Group  Version  Desired  Status    Created At
f5057465  f6646949  api         1        run      running   07/26/17 20:29:23 UTC
b1c88d20  f6646949  api         1        run      running   07/26/17 20:28:59 UTC
1140bacf  f6646949  api         0        run      running   07/26/17 20:28:37 UTC
1958a34a  f6646949  api         0        run      running   07/26/17 20:28:37 UTC
4bda385a  f6646949  api         0        run      running   07/26/17 20:28:37 UTC
62d96f06  f6646949  api         0        stop     complete  07/26/17 20:28:37 UTC
f58abbb2  f6646949  api         0        stop     complete  07/26/17 20:28:37 UTC
```

Alternatively, if the canary was not performing properly, we could abandon the
change using the `nomad deployment fail` command, similar to the blue/green
example.
