---
layout: "guides"
page_title: "Apache Spark Integration - Submitting Applications"
sidebar_current: "guides-spark-submit"
description: |-
  Learn how to submit Spark jobs that run on a Nomad cluster.
---

# Submitting Applications

The [`spark-submit`](https://spark.apache.org/docs/latest/submitting-applications.html) 
script located in Spark’s `bin` directory is used to launch applications on a 
cluster. Spark applications can be submitted to Nomad in either `client` mode 
or `cluster` mode.

## Client Mode

In `client` mode, the application driver runs on a machine that is not 
necessarily in the Nomad cluster. The driver’s `SparkContext` creates a Nomad 
job to run Spark executors. The executors connect to the driver and run Spark 
tasks on behalf of the application. When the driver’s SparkContext is stopped, 
the executors are shut down. Note that the machine running the driver or 
`spark-submit` needs to be reachable from the Nomad clients so that the 
executors can connect to it.

In `client` mode, application resources need to start out present on the 
submitting machine, so JAR files (both the primary JAR and those added with the 
`--jars` option) can not be specified using `http:` or `https:` URLs. You can 
either use files on the submitting machine (either as raw paths or `file:` URLs)
, or use `local:` URLs to indicate that the files are independently available on
 both the submitting machine and all of the Nomad clients where the executors 
 might run.

In this mode, the `spark-submit` invocation doesn’t return until the application 
has finished running, and killing the `spark-submit` process kills the 
application. 

In this example, the `spark-submit` command is used to run the `SparkPi` sample 
application:

```shell
$ spark-submit --class org.apache.spark.examples.SparkPi \
    --master nomad \
    --conf spark.nomad.sparkDistribution=https://s3.amazonaws.com/nomad-spark/spark-2.1.0-bin-nomad.tgz \
    lib/spark-examples*.jar \
    10
```

## Cluster Mode

In cluster mode, the `spark-submit` process creates a Nomad job to run the Spark 
application driver itself. The driver’s `SparkContext` then adds Spark executors
 to the Nomad job. The executors connect to the driver and run Spark tasks on 
 behalf of the application. When the driver’s `SparkContext` is stopped, the 
 executors are shut down.

In cluster mode, application resources need to be hosted somewhere accessible 
to the Nomad cluster, so JARs (both the primary JAR and those added with the 
`--jars` option) can’t be specified using raw paths or `file:` URLs. You can either 
use `http:` or `https:` URLs, or use `local:` URLs to indicate that the files are 
independently available on all of the Nomad clients where the driver and executors 
might run.

Note that in cluster mode, the Nomad master URL needs to be routable from both 
the submitting machine and the Nomad client node that runs the driver. If the 
Nomad cluster is integrated with Consul, you may want to use a DNS name for the 
Nomad service served by Consul.

For example, to submit an application in cluster mode:

```shell
$ spark-submit --class org.apache.spark.examples.SparkPi \
    --master nomad \
    --deploy-mode cluster \
    --conf spark.nomad.sparkDistribution=http://example.com/spark.tgz \
    http://example.com/spark-examples.jar \
    10
```

## Next Steps

Learn how to [customize applications](/guides/spark/customizing.html).
