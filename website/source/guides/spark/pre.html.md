---
layout: "guides"
page_title: "Apache Spark Integration - Getting Started"
sidebar_current: "guides-spark-pre"
description: |-
  Get started with the Nomad/Spark integration.
---

# Getting Started

To get started, you can use Nomad's example Terraform configuration to 
automatically provision an environment in AWS, or you can manually provision a 
cluster.

## Provision a Cluster in AWS

Nomad's [Terraform configuration](https://github.com/hashicorp/nomad/tree/master/terraform) 
can be used to quickly provision a Spark-enabled Nomad environment in
 AWS. The embedded [Spark example](https://github.com/hashicorp/nomad/tree/master/terraform/examples/spark)
 provides for a quickstart experience that can be used in conjunction with 
 this guide. When you have a cluster up and running, you can proceed to 
[Submitting applications](/guides/spark/submit.html).

## Manually Provision a Cluster

To manually configure provision a cluster, see the Nomad 
[Getting Started](/intro/getting-started/install.html) guide. There are two 
basic prerequisites to using the Spark integration once you have a cluster up 
and running:

- Access to a [Spark distribution](https://s3.amazonaws.com/nomad-spark/spark-2.1.0-bin-nomad.tgz) 
built with Nomad support. This is required for the machine that will submit 
applications as well as the Nomad tasks that will run the Spark executors.

- A Java runtime environment (JRE) for the submitting machine and the executors.

The subsections below explain further.

### Configure the Submitting Machine

To run Spark applications on Nomad, the submitting machine must have access to 
the cluster and have the Nomad-enabled Spark distribution installed. The code 
snippets below walk through installing Java and Spark on Ubuntu:

Install Java:

```shell
$ sudo add-apt-repository -y ppa:openjdk-r/ppa
$ sudo apt-get update 
$ sudo apt-get install -y openjdk-8-jdk
$ JAVA_HOME=$(readlink -f /usr/bin/java | sed "s:bin/java::")
```

Install Spark:


```shell
$ wget -O - https://s3.amazonaws.com/nomad-spark/spark-2.1.0-bin-nomad.tgz \
  | sudo tar xz -C /usr/local
$ export PATH=$PATH:/usr/local/spark-2.1.0-bin-nomad/bin
```

Export NOMAD_ADDR to point Spark to your Nomad cluster:

```shell
$ export NOMAD_ADDR=http://NOMAD_SERVER_IP:4646
```

### Executor Access to the Spark Distribution

When running on Nomad, Spark creates Nomad tasks to run executors for use by the 
application's driver program. The executor tasks need access to a JRE, a Spark 
distribution built with Nomad support, and (in cluster mode) the Spark 
application itself. By default, Nomad will only place Spark executors on client 
nodes that have the Java runtime installed (version 7 or higher).

In this example, the Spark distribution and the Spark application JAR file are
being pulled from Amazon S3:

```shell
$ spark-submit \
    --class org.apache.spark.examples.JavaSparkPi \
    --master nomad \
    --deploy-mode cluster \
    --conf spark.executor.instances=4 \
    --conf spark.nomad.sparkDistribution=https://s3.amazonaws.com/nomad-spark/spark-2.1.0-bin-nomad.tgz \
    https://s3.amazonaws.com/nomad-spark/spark-examples_2.11-2.1.0-SNAPSHOT.jar 100
```

### Using a Docker Image

An alternative to installing the JRE on every client node is to set the 
[spark.nomad.dockerImage](/guides/spark/configuration.html#spark-nomad-dockerimage)
 configuration property to the URL of a Docker image that has the Java runtime 
installed. If set, Nomad will use the `docker` driver to run Spark executors in 
a container created from the image. The 
[spark.nomad.dockerAuth](/guides/spark/configuration.html#spark-nomad-dockerauth) 
 configuration property can be set to a JSON object to provide Docker repository
 authentication configuration.

When using a Docker image, both the Spark distribution and the application 
itself can be included (in which case local URLs can be used for `spark-submit`).

Here, we include [spark.nomad.dockerImage](/guides/spark/configuration.html#spark-nomad-dockerimage) 
and use local paths for 
[spark.nomad.sparkDistribution](/guides/spark/configuration.html#spark-nomad-sparkdistribution) 
and the application JAR file:

```shell
$ spark-submit \
    --class org.apache.spark.examples.JavaSparkPi \
    --master nomad \
    --deploy-mode cluster \
    --conf spark.nomad.dockerImage=rcgenova/spark \
    --conf spark.executor.instances=4 \
    --conf spark.nomad.sparkDistribution=/spark-2.1.0-bin-nomad.tgz \
    /spark-examples_2.11-2.1.0-SNAPSHOT.jar 100
```

## Next Steps

Learn how to [submit applications](/guides/spark/submit.html).
