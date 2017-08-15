---
layout: "docs"
page_title: "Job Specification"
sidebar_current: "docs-job-specification-syntax"
description: |-
  Learn about the Job specification used to submit jobs to Nomad.
---

# Job Specification

The Nomad job specification (or "jobspec" for short) defines the schema for
Nomad jobs. Nomad jobs are specified in [HCL][], which aims to strike a balance
between human readable and editable, and machine-friendly.

The job specification is broken down into smaller pieces, which you will find
expanded in the navigation menu. We recommend getting started at the [job][]
stanza. Alternatively, you can keep reading to see a few examples.

For machine-friendliness, Nomad can also read JSON-equivalent configurations. In
general, we recommend using the HCL syntax.

The general hierarchy for a job is:

```text
job
  \_ group
        \_ task
```

Each job file has only a single job, however a job may have multiple groups, and
each group may have multiple tasks. Groups contain a set of tasks that are
co-located on a machine.

## Example

This example shows a sample job file. We tried to keep it as simple as possible,
while still showcasing the power of Nomad. For a more detailed explanation of
any of these fields, please use the navigation to dive deeper.

```hcl
# This declares a job named "docs". There can be exactly one
# job declaration per job file.
job "docs" {
  # Specify this job should run in the region named "us". Regions
  # are defined by the Nomad servers' configuration.
  region = "us"

  # Spread the tasks in this job between us-west-1 and us-east-1.
  datacenters = ["us-west-1", "us-east-1"]

  # Run this job as a "service" type. Each job type has different
  # properties. See the documentation below for more examples.
  type = "service"

  # Specify this job to have rolling updates, two-at-a-time, with
  # 30 second intervals.
  update {
    stagger      = "30s"
    max_parallel = 2
  }

  # A group defines a series of tasks that should be co-located
  # on the same client (host). All tasks within a group will be
  # placed on the same host.
  group "webs" {
    # Specify the number of these tasks we want.
    count = 5

    # Create an individual task (unit of work). This particular
    # task utilizes a Docker container to front a web application.
    task "frontend" {
      # Specify the driver to be "docker". Nomad supports
      # multiple drivers.
      driver = "docker"

      # Configuration is specific to each driver.
      config {
        image = "hashicorp/web-frontend"
      }

      # The service block tells Nomad how to register this service
      # with Consul for service discovery and monitoring.
      service {
        # This tells Consul to monitor the service on the port
        # labled "http". Since Nomad allocates high dynamic port
        # numbers, we use labels to refer to them.
        port = "http"

        check {
          type     = "http"
          path     = "/health"
          interval = "10s"
          timeout  = "2s"
        }
      }

      # It is possible to set environment variables which will be
      # available to the job when it runs.
      env {
        "DB_HOST" = "db01.example.com"
        "DB_USER" = "web"
        "DB_PASS" = "loremipsum"
      }

      # Specify the maximum resources required to run the job,
      # include CPU, memory, and bandwidth.
      resources {
        cpu    = 500 # MHz
        memory = 128 # MB

        network {
          mbits = 100

          # This requests a dynamic port named "http". This will
          # be something like "46283", but we refer to it via the
          # label "http".
          port "http" {}

          # This requests a static port on 443 on the host. This
          # will restrict this task to running once per host, since
          # there is only one port 443 on each host.
          port "https" {
            static = 443
          }
        }
      }
    }
  }
}
```

[hcl]: https://github.com/hashicorp/hcl "HashiCorp Configuration Language"
[job]: /docs/job-specification/job.html "Nomad job Job Specification"
