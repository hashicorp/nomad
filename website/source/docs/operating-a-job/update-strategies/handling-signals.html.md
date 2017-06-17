---
layout: "docs"
page_title: "Handling Signals - Operating a Job"
sidebar_current: "docs-operating-a-job-updating-handling-signals"
description: |-
  Well-behaved applications expose a way to perform cleanup prior to exiting.
  Nomad can optionally send a configurable signal to applications before
  killing them, allowing them to drain connections or gracefully terminate.
---

# Handling Signals

On operating systems that support signals, Nomad will send the application a
configurable signal before killing it. This gives the application time to
gracefully drain connections and conduct other cleanup before shutting down.
Certain applications take longer to drain than others, and thus Nomad allows
specifying the amount of time to wait for the application to exit before
force-killing it.

Before Nomad terminates an application, it will send the `SIGINT` signal to the
process. Processes running under Nomad should respond to this signal to
gracefully drain connections. After a configurable timeout, the application
will be force-terminated.

For more details on the `kill_timeout` option, please see the
[job specification documentation](/docs/job-specification/task.html#kill_timeout).

```hcl
job "docs" {
  group "example" {
    task "server" {
      # ...
      kill_timeout = "45s"
    }
  }
}
```

The behavior is slightly different for Docker-based tasks. Nomad will run the
`docker stop` command with the specified `kill_timeout`. The signal that `docker
stop` sends to your container entrypoint is configurable using the
[`STOPSIGNAL` configuration directive]
(https://docs.docker.com/engine/reference/builder/#stopsignal), however please
note that the default is `SIGTERM`.
