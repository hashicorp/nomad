---
layout: "docs"
page_title: "Rolling Upgrades - Operating a Job"
sidebar_current: "docs-operating-a-job-updating-rolling-upgrades"
description: |-
  In order to update a service while reducing downtime, Nomad provides a
  built-in mechanism for rolling upgrades. Rolling upgrades allow for a subset
  of applications to be updated at a time, with a waiting period between to
  reduce downtime.
---

# Rolling Upgrades

In order to update a service while reducing downtime, Nomad provides a built-in
mechanism for rolling upgrades. Jobs specify their "update strategy" using the
`update` block in the job specification as shown here:

```hcl
job "docs" {
  update {
    stagger      = "30s"
    max_parallel = 3
  }

  group "example" {
    task "server" {
      # ...
    }
  }
}
```

In this example, Nomad will only update 3 task groups at a time (`max_parallel =
3`) and will wait 30 seconds (`stagger = "30s"`) before moving on to the next
set of task groups.

## Planning Changes

Suppose we make a change to a file to upgrade the version of a Docker container
that is configured with the same rolling update strategy from above.

```diff
@@ -2,6 +2,8 @@ job "docs" {
   group "example" {
     task "server" {
       driver = "docker"

       config {
-        image = "nginx:1.10"
+        image = "nginx:1.11"
```

The [`nomad plan` command](http://localhost:4567/docs/commands/plan.html) allows
us to visualize the series of steps the scheduler would perform. We can analyze
this output to confirm it is correct:

```shell
$ nomad plan docs.nomad
```

Here is some sample output:

```text
+/- Job: "my-web"
+/- Task Group: "web" (3 create/destroy update)
  +/- Task: "web" (forces create/destroy update)
    +/- Config {
      +/- image: "nginx:1.10" => "nginx:1.11"
    }

Scheduler dry-run:
- All tasks successfully allocated.
- Rolling update, next evaluation will be in 30s.

Job Modify Index: 7
To submit the job with version verification run:

nomad run -check-index 7 my-web.nomad

When running the job with the check-index flag, the job will only be run if the
server side version matches the job modify index returned. If the index has
changed, another user has modified the job and the plan's results are
potentially invalid.
```

Here we can see that Nomad will destroy the 3 existing tasks and create 3
replacements but it will occur with a rolling update with a stagger of `30s`.

For more details on the `update` block, see the
[job specification documentation](/docs/job-specification/update.html).
