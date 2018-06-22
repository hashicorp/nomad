---
layout: "guides"
page_title: "Job Lifecycle"
sidebar_current: "guides-operating-a-job"
description: |-
  Learn how to deploy and manage a Nomad Job.
---

# Job Lifecycle

The general flow for operating a job in Nomad is:

1. Author the job file according to the [job specification](/docs/job-specification/index.html)
1. Plan and review the changes with a Nomad server
1. Submit the job file to a Nomad server
1. (Optional) Review job status and logs

When updating a job, there are a number of built-in update strategies which may
be defined in the job file. The general flow for updating an existing job in
Nomad is:

1. Modify the existing job file with the desired changes
1. Plan and review the changes with a Nomad server
1. Submit the job file to a Nomad server
1. (Optional) Review job status and logs

Because the job file defines the update strategy (blue-green, rolling updates,
etc.), the workflow remains the same regardless of whether this is an initial
deployment or a long-running job.

This section provides some best practices and guidance for operating jobs under
Nomad. Please navigate the appropriate sub-sections for more information.
