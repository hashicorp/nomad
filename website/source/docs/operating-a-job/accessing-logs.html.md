---
layout: "docs"
page_title: "Accessing Logs - Operating a Job"
sidebar_current: "docs-operating-a-job-accessing-logs"
description: |-
  Nomad provides a top-level mechanism for viewing application logs and data
  files via the command line interface. This section discusses the nomad logs
  command and API interface.
---

# Accessing Logs

Viewing application logs is critical for debugging issues, examining performance
problems, or even just verifying the application started correctly. To make this
as simple as possible, Nomad provides:

- Job specification for [log rotation](/docs/job-specification/logs.html)
- CLI command for [log viewing](/docs/commands/logs.html)
- API for programatic [log access](/api/client.html#stream-logs)

This section will utilize the job named "docs" from the [previous
sections](/docs/operating-a-job/submitting-jobs.html), but these operations
and command largely apply to all jobs in Nomad.

As a reminder, here is the output of the run command from the previous example:

```text
$ nomad run docs.nomad
==> Monitoring evaluation "42d788a3"
    Evaluation triggered by job "docs"
    Allocation "04d9627d" created: node "a1f934c9", group "example"
    Allocation "e7b8d4f5" created: node "012ea79b", group "example"
    Allocation "5cbf23a1" modified: node "1e1aa1e0", group "example"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "42d788a3" finished with status "complete"
```

The provided allocation ID (which is also available via the `nomad status`
command) is required to access the application's logs. To access the logs of our
application, we issue the following command:

```shell
$ nomad logs 04d9627d
```

The output will look something like this:

```text
<timestamp> 10.1.1.196:5678 10.1.1.196:33407 "GET / HTTP/1.1" 200 12 "curl/7.35.0" 21.809µs
<timestamp> 10.1.1.196:5678 10.1.1.196:33408 "GET / HTTP/1.1" 200 12 "curl/7.35.0" 20.241µs
<timestamp> 10.1.1.196:5678 10.1.1.196:33409 "GET / HTTP/1.1" 200 12 "curl/7.35.0" 13.629µs
```

By default, this will return the logs of the task. If more than one task is
defined in the job file, the name of the task is a required argument:

```shell
$ nomad logs 04d9627d server
```

The logs command supports both displaying the logs as well as following logs,
blocking for more output, similar to `tail -f`. To follow the logs, use the
appropriately named `-f` flag:

```shell
$ nomad logs -f 04d9627d
```

This will stream logs to our console.

If you wish to see only the tail of a log, use the `-tail` and `-n` flags:

```shell
$ nomad logs -tail -n 25 04d9627d
```
This will show the last 25 lines. If you omit the `-n` flag, `-tail` will
default to 10 lines.

By default, only the logs on stdout are displayed. To show the log output from
stderr, use the `-stderr` flag:

```shell
$ nomad logs -stderr 04d9627d
```

## Log Shipper Pattern

While the logs command works well for quickly accessing application logs, it
generally does not scale to large systems or systems that produce a lot of log
output, especially for the long-term storage of logs. Nomad's retention of log
files is best effort, so chatty applications should use a better log retention
strategy.

Since applications log to the `alloc/` directory, all tasks within the same task
group have access to each others logs. Thus it is possible to have a task group
as follows:

```hcl
group "my-group" {
  task "server" {
    # ...

    # Setting the server task as the leader of the task group allows us to
    # signal the log shipper task to gracefully shutdown when the server exits.
    leader = true
  }

  task "log-shipper" {
    # ...
  }
}
```

In the above example, the `server` task is the application that should be run
and will be producing the logs. The `log-shipper` reads those logs from the
`alloc/logs/` directory and sends them to a longer-term storage solution such as
Amazon S3 or an internal log aggregation system.

When using the log shipper pattern, especially for batch jobs, the main task
should be marked as the [leader task](/docs/job-specification/task.html#leader).
By marking the main task as a leader, when the task completes all other tasks
within the group will be gracefully shutdown. This allows the log shipper to
finish sending any logs and then exiting itself. The log shipper should set a
high enough [`kill_timeout`](/docs/job-specification/task.html#kill_timeout)
such that it can ship any remaining logs before exiting.
