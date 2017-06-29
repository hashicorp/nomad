---
layout: "guides"
page_title: "Apache Spark Integration - Resource Allocation"
sidebar_current: "guides-spark-resource"
description: |-
  Learn how to configure resource allocation for your Spark applications.
---

# Resource Allocation

Resource allocation can be configured using a job template or through 
configuration properties. Here is a sample template in HCL syntax (this would 
need to be converted to JSON):

```hcl
job "template" {
  group "group-name" {

    task "executor" {
      meta {
        "spark.nomad.role" = "executor"
      }

      resources {
        cpu = 2000
        memory = 2048
        network {
          mbits = 100
        }
      }
    }
  }
}
```
Resource-related configuration properties are covered below.

## Memory

The standard Spark memory properties will be propagated to Nomad to control 
task resource allocation: `spark.driver.memory` (set by `--driver-memory`) and 
`spark.executor.memory` (set by `--executor-memory`). You can additionally specify
 [spark.nomad.shuffle.memory](/guides/spark/configuration.html#spark-nomad-shuffle-memory)
  to control how much memory Nomad allocates to  shuffle service tasks.

## CPU

Spark sizes its thread pools and allocates tasks based on the number of CPU 
cores available. Nomad manages CPU allocation in terms of processing speed 
rather than number of cores. When running Spark on Nomad, you can control how 
much CPU share Nomad will allocate to tasks using the 
[spark.nomad.driver.cpu](/guides/spark/configuration.html#spark-nomad-driver-cpu) 
(set by `--driver-cpu`), 
[spark.nomad.executor.cpu](/guides/spark/configuration.html#spark-nomad-executor-cpu) 
(set by `--executor-cpu`) and 
[spark.nomad.shuffle.cpu](/guides/spark/configuration.html#spark-nomad-shuffle-cpu) 
properties. When running on Nomad, executors will be configured to use one core 
by default, meaning they will only pull a single 1-core task at a time. You can 
set the `spark.executor.cores` property (set by `--executor-cores`) to allow 
more tasks to be executed concurrently on a single executor.

## Network

Nomad does not restrict the network bandwidth of running tasks, bit it does 
allocate a non-zero number of Mbit/s to each task and uses this when bin packing 
task groups onto Nomad clients. Spark defaults to requesting the minimum of 1 
Mbit/s per task, but you can change this with the 
[spark.nomad.driver.networkMBits](/guides/spark/configuration.html#spark-nomad-driver-networkmbits), 
[spark.nomad.executor.networkMBits](/guides/spark/configuration.html#spark-nomad-executor-networkmbits), and
[spark.nomad.shuffle.networkMBits](/guides/spark/configuration.html#spark-nomad-shuffle-networkmbits) 
properties.

## Log rotation

Nomad performs log rotation on the `stdout` and `stderr` of its tasks. You can 
configure the number number and size of log files it will keep for driver and 
executor task groups using 
[spark.nomad.driver.logMaxFiles](/guides/spark/configuration.html#spark-nomad-driver-logmaxfiles) 
and [spark.nomad.executor.logMaxFiles](/guides/spark/configuration.html#spark-nomad-executor-logmaxfiles).

## Next Steps

Learn how to [dynamically allocate Spark executors](/guides/spark/dynamic.html).
