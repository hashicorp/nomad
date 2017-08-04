---
layout: "guides"
page_title: "Apache Spark Integration - Dynamic Executors"
sidebar_current: "guides-spark-dynamic"
description: |-
  Learn how to dynamically scale Spark executors based the queue of pending 
  tasks.
---

# Dynamically Allocate Spark Executors

By default, the Spark application will use a fixed number of executors. Setting 
`spark.dynamicAllocation` to `true` enables Spark to add and remove executors 
during execution depending on the number of Spark tasks scheduled to run. As 
described in [Dynamic Resource Allocation](http://spark.apache.org/docs/latest/job-scheduling.html#dynamic-resource-allocation), dynamic allocation requires that `spark.shuffle.service.enabled` be set to `true`.

On Nomad, this adds an additional shuffle service task to the executor 
task group. This results in a one-to-one mapping of executors to shuffle 
services.

When the executor exits, the shuffle service continues running so that it can 
serve any results produced by the executor. Due to the nature of resource 
allocation in Nomad, the resources allocated to the executor tasks are not
 freed until the shuffle service (and the application) has finished.

## Next Steps

Learn how to [integrate Spark with HDFS](/guides/spark/hdfs.html).
