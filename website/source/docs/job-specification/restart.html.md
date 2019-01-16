---
layout: "docs"
page_title: "restart Stanza - Job Specification"
sidebar_current: "docs-job-specification-restart"
description: |-
  The "restart" stanza configures a group's behavior on task failure.
---

# `restart` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>job -> group -> **restart**</code>
    </td>
  </tr>
</table>

The `restart` stanza configures a group's behavior on task failure. Restarts
happen on the client that is running the task.

```hcl
job "docs" {
  group "example" {
    restart {
      attempts = 3
      delay    = "30s"
    }
  }
}
```

## `restart` Parameters

- `attempts` `(int: <varies>)` - Specifies the number of restarts allowed in the
  configured interval. Defaults vary by job type, see below for more
  information.

- `delay` `(string: "15s")` - Specifies the duration to wait before restarting a
  task. This is specified using a label suffix like "30s" or "1h". A random
  jitter of up to 25% is added to the delay.

- `interval` `(string: <varies>)` - Specifies the duration which begins when the
  first task starts and ensures that only `attempts` number of restarts happens
  within it. If more than `attempts` number of failures happen, behavior is
  controlled by `mode`. This is specified using a label suffix like "30s" or
  "1h". Defaults vary by job type, see below for more information.

- `mode` `(string: "delay")` - Controls the behavior when the task fails more
  than `attempts` times in an interval. For a detailed explanation of these
  values and their behavior, please see the [mode values section](#mode-values).

### `restart` Parameter Defaults

The values for many of the `restart` parameters vary by job type. Here are the
defaults by job type:

- The default batch restart policy is:

    ```hcl
    restart {
      attempts = 15
      delay    = "15s"
      interval = "168h"
      mode     = "fail"
    }
    ```

- The default non-batch restart policy is:

    ```hcl
    restart {
      interval = "1m"
      attempts = 2
      delay    = "15s"
      mode     = "fail"
    }
    ```


### `mode` Values

This section details the specific values for the "mode" parameter in the Nomad
job specification for constraints. The mode is always specified as a string

```hcl
restart {
  mode = "..."
}
```

- `"delay"` - Instructs the scheduler to delay the next restart until the next
  `interval` is reached. This is the default behavior.

- `"fail"` - Instructs the scheduler to not attempt to restart the task on
  failure. This mode is useful for non-idempotent jobs which are unlikely to
  succeed after a few failures.
