---
layout: "guides"
page_title: "Apache Spark Integration - Monitoring Output"
sidebar_current: "guides-spark-monitoring"
description: |-
  Learn how to monitor Spark application output.
---

# Monitoring Spark Application Output

By default, `spark-submit` in `cluster` mode will submit your application
 to the Nomad cluster and return immediately. You can use the 
 [spark.nomad.cluster.monitorUntil](/guides/spark/configuration.html#spark-nomad-cluster-monitoruntil) configuration property to have 
 `spark-submit` monitor the job continuously. Note that, with this flag set, 
 killing `spark-submit` will *not* stop the spark application, since it will be
  running independently in the Nomad cluster. 

## Spark UI

In cluster mode, if `spark.ui.enabled` is set to `true` (as by default), the 
Spark web UI will be dynamically allocated a port. The Web UI will be exposed by
 Nomad as a service, and the UI’s `URL` will appear in the Spark driver’s log. By 
default, the Spark web UI will terminate when the application finishes. This can 
be problematic when debugging an application. You can delay termination by 
setting `spark.ui.stopDelay` (e.g. `5m` for 5 minutes). Note that this will 
cause the driver process to continue to run. You can force termination
 immediately on the “Jobs” page of the web UI.

## Spark History Server

It is possible to reconstruct the web UI of a completed application using 
Spark’s [history server](https://spark.apache.org/docs/latest/monitoring.html#viewing-after-the-fact). 
The history server requires the event log to have been written to an accessible 
location like [HDFS](/guides/spark/hdfs.html) or Amazon S3.

Sample history server job file:

```hcl
job "spark-history-server" {
  datacenters = ["dc1"]
  type = "service"

  group "server" {
    count = 1

    task "history-server" {
      driver = "docker"
      
      config {
        image = "barnardb/spark"
        command = "/spark/spark-2.1.0-bin-nomad/bin/spark-class"
        args = [ "org.apache.spark.deploy.history.HistoryServer" ]
        port_map {
          ui = 18080
        }
        network_mode = "host"
      }

      env {
        "SPARK_HISTORY_OPTS" = "-Dspark.history.fs.logDirectory=hdfs://hdfs.service.consul/spark-events/"
        "SPARK_PUBLIC_DNS"   = "spark-history.service.consul"
      }

      resources {
        cpu    = 1000
        memory = 1024
        network {
          mbits = 250
          port "ui" {
            static = 18080
          }
        }
      }

      service {
        name = "spark-history"
        tags = ["spark", "ui"]
        port = "ui"
      }
    }

  }
}
```

The job file above can also be found [here](https://github.com/hashicorp/nomad/blob/f-terraform-config/terraform/examples/spark/spark-history-server.nomad).

To run the history server, first [deploy HDFS](/guides/spark/hdfs.html) and then 
create a directory in HDFS to store events:

```shell
$ hdfs dfs -mkdir /spark-events
```

You can then deploy the history server with:

```shell
$ nomad run spark-history-server-hdfs.nomad
```

You can get the private IP for the history server with a Consul DNS lookup:

```shell
$ dig.spark-history.service.consul
```

Find the public IP that corresponds to the private IP returned by the `dig` 
command above. You can access the history server at http://PUBLIC_IP:18080.

Use the `spark.eventLog.enabled` and `spark.eventLog.dir` configuration 
properties in `spark-submit` to log events for a given application:

```shell
$ spark-submit \
    --class org.apache.spark.examples.JavaSparkPi \
    --master nomad \
    --deploy-mode cluster \
    --conf spark.executor.instances=4 \
    --conf spark.nomad.cluster.monitorUntil=complete \
    --conf spark.eventLog.enabled=true \
    --conf spark.eventLog.dir=hdfs://hdfs.service.consul/spark-events \
    --conf spark.nomad.sparkDistribution=https://s3.amazonaws.com/nomad-spark/spark-2.1.0-bin-nomad.tgz \
    https://s3.amazonaws.com/nomad-spark/spark-examples_2.11-2.1.0-SNAPSHOT.jar 100
```

## Logs

Nomad clients collect the `stderr` and `stdout` of running tasks. The CLI or the
 HTTP API can be used to inspect logs, as documented in 
[Accessing Logs](https://www.nomadproject.io/docs/operating-a-job/accessing-logs.html).
In cluster mode, the `stderr` and `stdout` of the `driver` application can be 
accessed in the same way. The [Log Shipper Pattern](https://www.nomadproject.io/docs/operating-a-job/accessing-logs.html#log-shipper-pattern) uses sidecar tasks to forward logs to a central location. This
can be done using a job template as follows:

```hcl
job "template" {
  group "driver" {

    task "driver" {
      meta {
        "spark.nomad.role" = "driver"
      }
    }

    task "log-forwarding-sidecar" {
      # sidecar task definition here
    }
  }

  group "executor" {

    task "executor" {
      meta {
        "spark.nomad.role" = "executor"
      }
    }

    task "log-forwarding-sidecar" {
      # sidecar task definition here
    }
  }
}
```

## Next Steps

Review the Nomad/Spark [configuration properties](/guides/spark/configuration.html).
