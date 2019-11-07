---
layout: "docs"
page_title: "dispatch_payload Stanza - Job Specification"
sidebar_current: "docs-job-specification-dispatch-payload"
description: |-
  The "dispatch_payload" stanza allows a task to access dispatch payloads.
  to
---

# `dispatch_payload` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>job -> group -> task -> **dispatch_payload**</code>
    </td>
  </tr>
</table>

The `dispatch_payload` stanza is used in conjunction with a [`parameterized`][parameterized] job
that expects a payload. When the job is dispatched with a payload, the payload
will be made available to any task that has a `dispatch_payload` stanza. The
payload will be written to the configured file before the task is started. This
allows the task to use the payload as input or configuration.

```hcl
job "docs" {
  group "example" {
    task "server" {
      dispatch_payload {
        file = "config.json"
      }
    }
  }
}
```

## `dispatch_payload` Parameters

- `file` `(string: "")` - Specifies the file name to write the content of
  dispatch payload to. The file is written relative to the [task's local
  directory][localdir].

## `dispatch_payload` Examples

The following examples only show the `dispatch_payload` stanzas. Remember that the
`dispatch_payload` stanza is only valid in the placements listed above.

### Write Payload to a File

This example shows a `dispatch_payload` block in a parameterized job that writes
the payload to a `config.json` file.

```hcl
dispatch_payload {
  file = "config.json"
}
```

[localdir]: /docs/runtime/environment.html#local_ "Task Local Directory"
[parameterized]: /docs/job-specification/parameterized.html "Nomad parameterized Job Specification"
