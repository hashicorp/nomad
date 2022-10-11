# Logging Plugins

_Note: This branch is currently experimental and not merged to main! It's being used to explore the design space for logging plugins._

The diagrams below describe the relationships between the Nomad client, the proposed logging plugins, tasks, and the resulting logs.

## Legacy Logmon

The existing logmon spawns a separate `logmon` process for each task via
go-plugin.

```mermaid
flowchart TB

    subgraph task1
    task11(task PID1)-->task1N(task PIDn)
    end

    subgraph task2
    task21(task PID1)-->task2N(task PIDn)
    end

    subgraph client
    c(client agent)
    c-- driver plugin -->task11
    c-- driver plugin -->task21
    end

    task11-- fifo -->lm1(logmon)
    task21-- fifo -->lm2(logmon)
    c-- go-plugin -->lm1
    c-- go-plugin -->lm2
```

## New Logmon (Proposed)

Under this approach, logmon is an internal plugin and spawns a separate
`logshipper` process via a task-driver-like fork interface. This will let us
isolate the log shipper tasks.

```mermaid
flowchart TB


    subgraph task2
    task21(task PID1)-->task2N(task PIDn)
    end

    subgraph task1
    task11(task PID1)-->task1N(task PIDn)
    end

    subgraph client
    c(client agent)
    c-- go-plugin internal -->L(logmon)
    c-- driver plugin -->task11
    c-- driver plugin -->task21
    end

    task11-- fifo -->lm1(logshipper)
    task21-- fifo -->lm2(logshipper)

    L -- fork with driver library -->lm1
    L -- fork with driver library -->lm2

```

## rotatelogs

The `rotatelogs` plugin is an example of using the driver library code from the
new `logmon` and using a different `logshipper` process. In this case, using
Apache `rotatelogs` to significantly reduce resource usage for supported
platforms.

```mermaid
flowchart TB


    subgraph task2
    task21(task PID1)-->task2N(task PIDn)
    end

    subgraph task1
    task11(task PID1)-->task1N(task PIDn)
    end

    subgraph client
    c(client agent)
    c-- go-plugin internal -->L(rotatelogs plugin)
    c-- driver plugin -->task11
    c-- driver plugin -->task21
    end

    task11-- fifo -->rl1(rotatelogs)
    task21-- fifo -->rl2(rotatelogs)

    L -- fork with driver library -->rl1
    L -- fork with driver library -->rl2

```


## onelogger

The `onelogger` plugin is an example of using the driver library code in an
external log plugin process that doesn't spawn per-task log shipping
processes. This can be useful for environments where noisy neighbors are not
expected to create interference.

```mermaid
flowchart TB

    subgraph task2
    task21(task PID1)-->task2N(task PIDn)
    end

    subgraph task1
    task11(task PID1)-->task1N(task PIDn)
    end

    subgraph client
    c(client agent)
    c-- driver plugin -->task11
    c-- driver plugin -->task21
    end

    c-- go-plugin  --> L

    L(onelogger plugin)
    task11-- fifo -->L
    task21-- fifo -->L

    L -- write to files -->f1(log files)
    L -- write to files -->f2(log files)

```


## Log Rotation Disabled

By disabling log rotation and removing the log shipping process, tasks can have
their stdout/stderr write directly to files in the allocation log directory
without giving up `nomad alloc logs` functionality. This is especially valuable
for batch tasks, which rarely live long enough for log rotation to be useful and
tend to get used on densely-packed clients.

```mermaid
flowchart TB

    subgraph task2
    task21(task PID1)-->task2N(task PIDn)
    end

    subgraph task1
    task11(task PID1)-->task1N(task PIDn)
    end

    subgraph client
    c(client agent)
    c-- driver plugin -->task11
    c-- driver plugin -->task21
    end

    task11-- file write -->f1(log files)
    task21-- file write -->f2(log files)

```

## Null Logging

For a more extreme version of the above, tasks can be configured to write their
stdout/stderr to `/dev/null`. This assumes that tasks never output any valuable
logs or their logs are being shipped off-host by the task itself.

```mermaid
flowchart TB
    subgraph task2
    task21(task PID1)-->task2N(task PIDn)
    end

    subgraph task1
    task11(task PID1)-->task1N(task PIDn)
    end

    subgraph client
    c(client agent)
    c-- driver plugin -->task11
    c-- driver plugin -->task21
    end

    null(/dev/null)
    task11-- file write -->null
    task21-- file write -->null

```
