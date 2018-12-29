---
layout: "guides"
page_title: "Apache Spark Integration - Using HDFS"
sidebar_current: "guides-spark-hdfs"
description: |-
  Learn how to deploy HDFS on Nomad and integrate it with Spark.
---

# Using HDFS

[HDFS](https://en.wikipedia.org/wiki/Apache_Hadoop#Hadoop_distributed_file_system) 
is a distributed, replicated and scalable file system written for the Hadoop 
framework. Spark was designed to read from and write to HDFS, since it is 
common for Spark applications to perform data-intensive processing over large 
datasets. HDFS can be deployed as its own Nomad job.

## Running HDFS on Nomad

A sample HDFS job file can be found [here](https://github.com/hashicorp/nomad/blob/master/terraform/examples/spark/hdfs.nomad).
It has two task groups, one for the HDFS NameNode and one for the 
DataNodes. Both task groups use a [Docker image](https://github.com/hashicorp/nomad/tree/master/terraform/examples/spark/docker/hdfs) that includes Hadoop:

```hcl
  group "NameNode" {

    constraint {
      operator  = "distinct_hosts"
      value     = "true"
    }

    task "NameNode" {

      driver = "docker"

      config {
        image = "rcgenova/hadoop-2.7.3"
        command = "bash"
        args = [ "-c", "hdfs namenode -format && exec hdfs namenode 
          -D fs.defaultFS=hdfs://${NOMAD_ADDR_ipc}/ -D dfs.permissions.enabled=false" ]
        network_mode = "host"
        port_map {
          ipc = 8020
          ui = 50070
        }
      }

      resources {
        cpu    = 1000
        memory = 1024
        network {
          port "ipc" {
            static = "8020"
          }
          port "ui" {
            static = "50070"
          }
        }
      }

      service {
        name = "hdfs"
        port = "ipc"
      }
    }
  }
```

The NameNode task registers itself in Consul as `hdfs`. This enables the 
DataNodes to generically reference the NameNode:

```hcl
  group "DataNode" {

    count = 3

    constraint {
      operator  = "distinct_hosts"
      value     = "true"
    }
    
    task "DataNode" {

      driver = "docker"

      config {
        network_mode = "host"
        image = "rcgenova/hadoop-2.7.3"
        args = [ "hdfs", "datanode"
          , "-D", "fs.defaultFS=hdfs://hdfs.service.consul/"
          , "-D", "dfs.permissions.enabled=false"
        ]
        port_map {
          data = 50010
          ipc = 50020
          ui = 50075
        }
      }

      resources {
        cpu    = 1000
        memory = 1024
        network {
          port "data" {
            static = "50010"
          }
          port "ipc" {
            static = "50020"
          }
          port "ui" {
            static = "50075"
          }
        }
      }

    }
  }
```

Another viable option for DataNode task group is to use a dedicated 
[system](https://www.nomadproject.io/docs/runtime/schedulers.html#system) job. 
This will deploy a DataNode to every client node in the system, which may or may 
not be desirable depending on your use case. 

The HDFS job can be deployed using the `nomad run` command:

```shell
$ nomad run hdfs.nomad
```

## Production Deployment Considerations

A production deployment will typically have redundant NameNodes in an 
active/passive configuration (which requires ZooKeeper). See [HDFS High 
Availability](https://hadoop.apache.org/docs/stable/hadoop-project-dist/hadoop-hdfs/HDFSHighAvailabilityWithNFS.html).

## Next Steps

Learn how to [monitor the output](/guides/spark/monitoring.html) of your 
Spark applications.
