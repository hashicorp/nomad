---
layout: "docs"
page_title: "check_restart Stanza - Job Specification"
sidebar_current: "docs-job-specification-check_restart"
description: |-
  The "check_restart" stanza instructs Nomad when to restart tasks with
  unhealthy service checks.
---

# `check_restart` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>job -> group -> task -> service -> check -> **check_restart**</code>
    </td>
  </tr>
</table>

As of Nomad 0.7 the `check_restart` stanza instructs Nomad when to restart
tasks with unhealthy service checks.  When a health check in Consul has been
unhealthy for the `limit` specified in a `check_restart` stanza, it is
restarted according to the task group's [`restart` policy][restart_stanza]. The
`check_restart` settings apply to [`check`s][check_stanza].

```hcl
job "mysql" {
  group "mysqld" {

    restart {
      attempts = 3
      delay    = "10s"
      interval = "10m"
      mode     = "fail"
    }

    task "server" {
      service {
        tags = ["leader", "mysql"]

        port = "db"

        check {
          type     = "tcp"
          port     = "db"
          interval = "10s"
          timeout  = "2s"
        }

        check {
          type     = "script"
          name     = "check_table"
          command  = "/usr/local/bin/check_mysql_table_status"
          args     = ["--verbose"]
          interval = "60s"
          timeout  = "5s"

          check_restart {
            limit = 3
            grace = "90s"
            ignore_warnings = false
          }
        }
      }
    }
  }
}
```

- `limit` `(int: 0)` - Restart task when a health check has failed `limit`
  times.  For example 1 causes a restart on the first failure. The default,
  `0`, disables health check based restarts. Failures must be consecutive. A
  single passing check will reset the count, so flapping services may not be
  restarted.

- `grace` `(string: "1s")` - Duration to wait after a task starts or restarts
  before checking its health.

- `ignore_warnings` `(bool: false)` - By default checks with both `critical`
  and `warning` statuses are considered unhealthy. Setting `ignore_warnings =
  true` treats a `warning` status like `passing` and will not trigger a restart.

## Example Behavior

Using the example `mysql` above would have the following behavior:

```hcl
check_restart {
  # ...
  grace = "90s"
  # ...
}
```

When the `server` task first starts and is registered in Consul, its health
will not be checked for 90 seconds. This gives the server time to startup.

```hcl
check_restart {
  limit = 3
  # ...
}
```

After the grace period if the script check fails, it has 180 seconds (`60s
interval * 3 limit`) to pass before a restart is triggered. Once a restart is
triggered the task group's [`restart` policy][restart_stanza] takes control:

```hcl
restart {
  # ...
  delay    = "10s"
  # ...
}
```

The [`restart` stanza][restart_stanza] controls the restart behavior of the
task. In this case it will stop the task and then wait 10 seconds before
starting it again.

Once the task restarts Nomad waits the `grace` period again before starting to
check the task's health.


```hcl
restart {
  attempts = 3
  # ...
  interval = "10m"
  mode     = "fail"
}
```

If the check continues to fail, the task will be restarted up to `attempts`
times within an `interval`. If the `restart` attempts are reached within the
`limit` then the `mode` controls the behavior. In this case the task would fail
and not be restarted again. See the [`restart` stanza][restart_stanza] for
details.

[check_stanza]:  /docs/job-specification/service.html#check-parameters "check stanza"
[restart_stanza]: /docs/job-specification/restart.html "restart stanza"
