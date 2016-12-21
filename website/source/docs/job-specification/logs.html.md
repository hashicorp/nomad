---
layout: "docs"
page_title: "logs Stanza - Job Specification"
sidebar_current: "docs-job-specification-logs"
description: |-
  The "logs" stanza configures the log rotation policy for a task's stdout and
  stderr. Logging is enabled by default with sane defaults. The "logs" stanza
  allows for finer-grained control over how Nomad handles log files.
---

# `logs` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>job -> group -> task -> **logs**</code>
    </td>
  </tr>
</table>

The `logs` stanza configures the log rotation policy for a task's `stdout` and
`stderr`. Logging is enabled by default with sane defaults (provided in the
parameters section below), and there is currently no way to disable logging for
tasks. The `logs` stanza allows for finer-grained control over how Nomad handles
log files.

```hcl
job "docs" {
  group "example" {
    task "server" {
      logs {
        max_files     = 10
        max_file_size = 10
      }
    }
  }
}
```

For information on how to interact with logs after they have been configured, please see the [`nomad logs`][logs-command] command.

## `logs` Parameters

- `max_files` `(int: 10)` - Specifies the maximum number of rotated files Nomad
  will retain for `stdout` and `stderr`. Each stream is tracked individually, so
  specifying a value of 2 will create 4 files - 2 for stdout and 2 for stderr

- `max_file_size` `(int: 10)` - Specifies the maximum size of each rotated file
  in `MB`. If the amount of disk resource requested for the task is less than
  the total amount of disk space needed to retain the rotated set of files,
  Nomad will return a validation error when a job is submitted.

## `logs` Examples

The following examples only show the `logs` stanzas. Remember that the
`logs` stanza is only valid in the placements listed above.

### Configure Defaults

This example shows a default logging configuration. Yes, it is empty on purpose.
Nomad automatically enables logging with sane defaults as described in the
parameters section above.

```hcl
```

### Customization

This example asks Nomad to retain 3 rotated files for each of `stderr` and
`stdout`, each a maximum size of 5MB per file. The minimum disk space this
would require is 60MB (3 `stderr` &plus; 3 `stdout` &times; 5MB &equals; 30MB).

```hcl
logs {
  max_files     = 3
  max_file_size = 5
}
```

[logs-command]: /docs/commands/logs.html "Nomad logs command"
