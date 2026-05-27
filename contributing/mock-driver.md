# Mock Driver

This repo includes a mock task driver in the [`drivers/mock`][] package that
implements a minimal task driver interface for development work. This driver is
loaded as the other [built-in drivers][] are, but only when Nomad is [not
compiled with the release tag][].

## Task Configuration

```hcl
task "mocktask" {
  driver = "mock_driver"

  config {
    run_for      = "10s"
    exit_code    = 7
    exit_err_msg = "the application crashed"
  }
}
```

The `mock_driver` driver supports the following configuration in the job spec:

- `run_for` `(duration: "0s")` - The duration for which the fake task runs
  for. After this period the driver responds indicating the task has terminated.
- `signal_error` `(string: "")` - The error message the task returns if signalled.
- `stderr_repeat_duration` `(duration: "0s")` - The duration between repeated stderr outputs.
- `stderr_repeat` `(number: 0)` - The number of times the `stderr_string` will be written.
- `stderr_string` `(string: "")` - The string that the task writes to stderr.
- `stdout_repeat_duration` `(string: "")` - The duration between repeated stdout outputs.
- `stdout_repeat` `(number: 0)` - The number of times the `stdout_string` will be written.
- `stdout_string` `(string: "")` - The string that the task writes to stdout.

The driver has configurable startup and shutdown for the tasks:

- `exit_code` `(number: 0)` - The exit code the driver should return for the exiting task.
- `exit_err_msg` `(string: "")` - The error message the task returns while exiting.
- `exit_signal` `(number: 0)` - The signal with which the driver indicates the
  task has been killed.
- `kill_after` `(duration: "0s")` - Duration after which the driver indicates
  the task has exited with `SIGINT`.
- `plugin_exit_after` `(string: "")` - Duration after which the driver indicates
  the plugin exited via the `WaitTask` call.
- `start_block_for` `(duration: "0s")` - Duration to block before returning when started.
- `start_error_recoverable` `(bool: false)` - Marks whether the error returned
  when starting the driver is recoverable.
- `start_error` `(string: "")` - The error that is returned when starting the driver.

The driver can present information to the client about the task as though it had networking:

- `driver_advertise` `(bool: false)` - Returned as `DriverNetwork.AutoAdvertise` from `Start()`
- `driver_ip` `(string: "")` - The address returned as the `DriverNetwork.IP` from `Start()`
- `driver_port_map` `(string: "")` - Parse a label:number pair and return it as
  `DriverNetwork.PortMap` from `Start()`.


## Plugin Options

```hcl
plugin "mock_driver" {
  fs_isolation               = "none"
  shutdown_periodic_after    = false
  shutdown_periodic_duration = 0
}
```

- `fs_isolation` `(string: "none")` - The type of file system isolation to
  report to the client. Must be one of `"none"`, `"chroot"`, or `"image"`.
- `shutdown_periodic_after` `(bool: false)` - A toggle that can be used during
  tests to "stop" a previously-functioning driver, allowing for testing of
  periodic drivers and fingerprinters.
- `shutdown_periodic_duration` `(number: 0)` - The duration after which to stop
  a previously-functioning driver, in seconds.


## Example

An example job that could be used for testing task `kill_timeout`:

```
job "mock" {

  group "group" {

    task "task" {

      driver = "mock_driver"

      kill_timeout = "5s"

      config {
        exit_code = 0
        exit_err_msg = "error on exit"
        exit_signal = 9
        kill_after = "3s"
        run_for = "30s"
        signal_error = "got signal"
        start_block_for = "1s"
        stdout_repeat = 1
        stdout_repeat_duration = "10s"
        stdout_string = "hello, world!\n"
      }

      resources {
        cpu    = 128
        memory = 128
      }

    }
  }
}
```

This results in the following allocation events:

```
Recent Events:
Time                       Type        Description
2023-03-20T16:22:39-04:00  Restarting  Task restarting in 17.426443129s
2023-03-20T16:22:39-04:00  Terminated  Exit Code: 0, Signal: 9, Exit Message: "error on exit"
2023-03-20T16:22:09-04:00  Started     Task started by client
2023-03-20T16:22:07-04:00  Task Setup  Building Task Directory
2023-03-20T16:22:07-04:00  Received    Task received by client
```

[built-in drivers]: https://github.com/hashicorp/nomad/blob/main/helper/pluginutils/catalog/register.go
[not compiled with the release tag]: https://github.com/hashicorp/nomad/blob/main/helper/pluginutils/catalog/register_testing.go
[`drivers/mock`]: https://github.com/hashicorp/nomad/tree/main/drivers/mock
