---
layout: "docs"
page_title: "Drivers: Raw Exec"
sidebar_current: "docs-drivers-raw-exec"
description: |-
  The Raw Exec task driver simply fork/execs and provides no isolation.
---

# Raw Fork/Exec Driver

Name: `raw_exec`

The `raw_exec` driver is used to execute a command for a task without any
isolation. Further, the task is started as the same user as the Nomad process.
As such, it should be used with extreme care and is disabled by default.

## Task Configuration

```hcl
task "webservice" {
  driver = "raw_exec"

  config {
    command = "my-binary"
    args    = ["-flag", "1"]
  }
}  
```

The `raw_exec` driver supports the following configuration in the job spec:

* `command` - The command to execute. Must be provided. If executing a binary
  that exists on the host, the path must be absolute. If executing a binary that
  is downloaded from an [`artifact`](/docs/job-specification/artifact.html), the
  path can be relative from the allocations's root directory.

* `args` - (Optional) A list of arguments to the `command`. References
  to environment variables or any [interpretable Nomad
  variables](/docs/runtime/interpolation.html) will be interpreted before
  launching the task.

## Examples

To run a binary present on the Node:

```
task "example" {
  driver = "raw_exec"

  config {
    # When running a binary that exists on the host, the path must be absolute/
    command = "/bin/sleep"
    args    = ["1"]
  }
}
```

To execute a binary downloaded from an [`artifact`](/docs/job-specification/artifact.html):

```
task "example" {
  driver = "raw_exec"

  config {
    command = "name-of-my-binary"
  }

  artifact {
    source = "https://internal.file.server/name-of-my-binary"
    options {
      checksum = "sha256:abd123445ds4555555555"
    }
  }
}
```

## Client Requirements

The `raw_exec` driver can run on all supported operating systems. For security
reasons, it is disabled by default. To enable raw exec, the Nomad client
configuration must explicitly enable the `raw_exec` driver in the client's
[options](/docs/agent/configuration/client.html#options):

```
client {
  options = {
    "driver.raw_exec.enable" = "1"
  }
}
```

## Client Attributes

The `raw_exec` driver will set the following client attributes:

* `driver.raw_exec` - This will be set to "1", indicating the driver is available.

## Resource Isolation

The `raw_exec` driver provides no isolation.
