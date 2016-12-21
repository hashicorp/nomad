---
layout: "docs"
page_title: "Submitting Jobs - Operating a Job"
sidebar_current: "docs-operating-a-job-submitting-jobs"
description: |-
  The job file is the unit of work in Nomad. Upon authoring, the job file is
  submitted to the server for evaluation and scheduling. This section discusses
  some techniques for submitting jobs.
---

# Submitting Jobs

In Nomad, the description of the job and all its requirements are maintained in
a single file called the "job file". This job file resides locally on disk and
it is highly recommended that you check job files into source control.

The general flow for submitting a job in Nomad is:

1. Author a job file according to the job specification
1. Plan and review changes with a Nomad server
1. Submit the job file to a Nomad server
1. (Optional) Review job status and logs

Here is a very basic example to get you started.

## Author a Job File

Authoring a job file is very easy. For more detailed information, please see the
[job specification](/docs/job-specification/index.html). Here is a sample job
file which runs a small docker container web server to get us started.

```hcl
job "docs" {
  datacenters = ["dc1"]

  group "example" {
    task "server" {
      driver = "docker"

      config {
        image = "hashicorp/http-echo"
        args = [
          "-listen", ":5678",
          "-text", "hello world",
        ]
      }

      resources {
        network {
          mbits = 10
          port "http" {
            static = "5678"
          }
        }
      }
    }
  }
}
```

This job file exists on your local workstation in plain text. When you are
satisfied with this job file, you will plan and review the scheduler decision.
It is generally a best practice to commit job files to source control,
especially if you are working in a team.

## Planning the Job

Once the job file is authored, we need to plan out the changes. The `nomad plan`
command invokes a dry-run of the scheduler and inform us of which scheduling
decisions would take place.

```shell
$ nomad plan docs.nomad
```

The resulting output will look like:

```text
+ Job: "docs"
+ Task Group: "example" (1 create)
  + Task: "server" (forces create)

Scheduler dry-run:
- All tasks successfully allocated.

Job Modify Index: 0
To submit the job with version verification run:

nomad run -check-index 0 docs.nomad

When running the job with the check-index flag, the job will only be run if the
server side version matches the job modify index returned. If the index has
changed, another user has modified the job and the plan's results are
potentially invalid.
```

Note that no action was taken. This job is not running. This is a complete
dry-run and no allocations have taken place.

## Submitting the Job

Assuming the output of the plan looks acceptable, we can ask Nomad to execute
this job. This is done via the `nomad run` command. We can optionally supply
the modify index provided to us by the plan command to ensure no changes to this
job have taken place between our plan and now.

```shell
$ nomad run docs.nomad
```

The resulting output will look like:

```text
==> Monitoring evaluation "0d159869"
    Evaluation triggered by job "docs"
    Allocation "5cbf23a1" created: node "1e1aa1e0", group "example"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "0d159869" finished with status "complete"
```

Now that the job is scheduled, it may or may not be running. We need to inspect
the allocation status and logs to make sure the job started correctly. The next
section on [inspecting state](/docs/operating-a-job/inspecting-state.html)
details ways to examine this job's state.

## Updating the Job

When making updates to the job, it is best to always run the plan command and
then the run command. For example:

```diff
@@ -2,6 +2,8 @@ job "docs" {
   datacenters = ["dc1"]

   group "example" {
+    count = "3"
+
     task "server" {
       driver = "docker"
```

After we save these changes to disk, run the plan command:

```text
$ nomad plan docs.nomad
+/- Job: "docs"
+/- Task Group: "example" (2 create, 1 in-place update)
  +/- Count: "1" => "3" (forces create)
      Task: "server"

Scheduler dry-run:
- All tasks successfully allocated.

Job Modify Index: 131
To submit the job with version verification run:

nomad run -check-index 131 docs.nomad

When running the job with the check-index flag, the job will only be run if the
server side version matches the job modify index returned. If the index has
changed, another user has modified the job and the plan's results are
potentially invalid.
```

And then run the run command, assuming the output looks okay. Note that we are
including the "check-index" parameter. This will ensure that no remote changes
have taken place to the job between our plan and run phases.

```text
nomad run -check-index 131 docs.nomad
==> Monitoring evaluation "42d788a3"
    Evaluation triggered by job "docs"
    Allocation "04d9627d" created: node "a1f934c9", group "example"
    Allocation "e7b8d4f5" created: node "012ea79b", group "example"
    Allocation "5cbf23a1" modified: node "1e1aa1e0", group "example"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "42d788a3" finished with status "complete"
```

For more details on advanced job updating strategies such as canary builds and
build-green deployments, please see the documentation on [job update
strategies](/docs/operating-a-job/update-strategies/index.html).
