---
layout: "docs"
page_title: "Submitting Jobs - Operating a Job"
sidebar_current: "docs-operating-a-job-submitting"
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
[job specification](/docs/jobspec/index.html). Here is a sample job file which
runs a small docker container web server.

```hcl
job "docs" {
  datacenters = ["dc1"]

  group "example" {
    task "server" {
      driver = "docker"
      config {
        image = "hashicorp/http-echo"
        args  = ["-text", "hello world"]
      }

      resources {
        memory = 32
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
command may be used to perform a dry-run of the scheduler and inform us of
which scheduling decisions would take place.

```shell
$ nomad plan example.nomad
```

The resulting output will look like:

```text
TODO: Output
```

Note that no action has been taken. This is a complete dry-run and no
allocations have taken place.

## Submitting the Job
Assuming the output of the plan looks acceptable, we can ask Nomad to execute
this job. This is done via the `nomad run` command. We can optionally supply
the modify index provided to us by the plan command to ensure no changes to this
job have taken place between our plan and now.

```shell
$ nomad run -check-index=123 example.nomad
```

The resulting output will look like:

```text
TODO: Output
```

Now that the job is scheduled, it may or may not be running. We need to inspect
the allocation status and logs to make sure the job started correctly. The next
section on [inspecting state](/docs/operating-a-job/inspecting-state.html) details ways to
examine this job.
