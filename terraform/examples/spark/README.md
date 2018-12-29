# Nomad / Spark integration

The Nomad ecosystem includes a fork of Apache Spark that natively supports using 
a Nomad cluster to run Spark applications. When running on Nomad, the Spark 
executors that run Spark tasks for your application, and optionally the 
application driver itself, run as Nomad tasks in a Nomad job. See the 
[usage guide](./RunningSparkOnNomad.pdf) for more details.

Clusters provisioned with Nomad's Terraform templates are automatically 
configured to run the Spark integration. The sample job files found here are 
also provisioned onto every client and server.

## Setup

To give the Spark integration a test drive, provision a cluster and SSH to any 
one of the clients or servers (the public IPs are displayed when the Terraform 
provisioning process completes):

```bash
$ ssh -i /path/to/key ubuntu@PUBLIC_IP
```

The Spark history server and several of the sample Spark jobs below require 
HDFS. Using the included job file, deploy an HDFS cluster on Nomad: 

```bash
$ cd $HOME/examples/spark
$ nomad run hdfs.nomad
$ nomad status hdfs
```

When the allocations are all in the `running` state (as shown by `nomad status 
hdfs`), query Consul to verify that the HDFS service has been registered:

```bash
$ dig hdfs.service.consul
```

Next, create directories and files in HDFS for use by the history server and the 
sample Spark jobs:

```bash
$ hdfs dfs -mkdir /foo
$ hdfs dfs -put /var/log/apt/history.log /foo
$ hdfs dfs -mkdir /spark-events
$ hdfs dfs -ls /
```

Finally, deploy the Spark history server:

```bash
$ nomad run spark-history-server-hdfs.nomad
```

You can get the private IP for the history server with a Consul DNS lookup:

```bash
$ dig spark-history.service.consul
```

Cross-reference the private IP with the `terraform apply` output to get the 
corresponding public IP. You can access the history server at 
`http://PUBLIC_IP:18080`.

## Sample Spark jobs

The sample `spark-submit` commands listed below demonstrate several of the 
official Spark examples. Features like `spark-sql`, `spark-shell` and `pyspark` 
are included. The commands can be executed from any client or server.

You can monitor the status of a Spark job in a second terminal session with:

```bash
$ nomad status
$ nomad status JOB_ID
$ nomad alloc-status DRIVER_ALLOC_ID
$ nomad logs DRIVER_ALLOC_ID
```

To view the output of the job, run `nomad logs` for the driver's Allocation ID.

### SparkPi (Java)

```bash
spark-submit \
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

### Word count (Java)

```bash
spark-submit \
  --class org.apache.spark.examples.JavaWordCount \
  --master nomad \
  --deploy-mode cluster \
  --conf spark.executor.instances=4 \
  --conf spark.nomad.cluster.monitorUntil=complete \
  --conf spark.eventLog.enabled=true \
  --conf spark.eventLog.dir=hdfs://hdfs.service.consul/spark-events \
  --conf spark.nomad.sparkDistribution=https://s3.amazonaws.com/nomad-spark/spark-2.1.0-bin-nomad.tgz \
  https://s3.amazonaws.com/nomad-spark/spark-examples_2.11-2.1.0-SNAPSHOT.jar \
  hdfs://hdfs.service.consul/foo/history.log
```

### DFSReadWriteTest (Scala)

```bash
spark-submit \
  --class org.apache.spark.examples.DFSReadWriteTest \
  --master nomad \
  --deploy-mode cluster \
  --conf spark.executor.instances=4 \
  --conf spark.nomad.cluster.monitorUntil=complete \
  --conf spark.eventLog.enabled=true \
  --conf spark.eventLog.dir=hdfs://hdfs.service.consul/spark-events \
  --conf spark.nomad.sparkDistribution=https://s3.amazonaws.com/nomad-spark/spark-2.1.0-bin-nomad.tgz \
  https://s3.amazonaws.com/nomad-spark/spark-examples_2.11-2.1.0-SNAPSHOT.jar \
  /etc/sudoers hdfs://hdfs.service.consul/foo
```

### spark-shell

Start the shell:

```bash
spark-shell \
  --master nomad \
  --conf spark.executor.instances=4 \
  --conf spark.nomad.sparkDistribution=https://s3.amazonaws.com/nomad-spark/spark-2.1.0-bin-nomad.tgz
```

Run a few commands:

```bash
$ spark.version

$ val data = 1 to 10000
$ val distData = sc.parallelize(data)
$ distData.filter(_ < 10).collect()
```

### sql-shell

Start the shell:

```bash
spark-sql \
  --master nomad \
  --conf spark.executor.instances=4 \
  --conf spark.nomad.sparkDistribution=https://s3.amazonaws.com/nomad-spark/spark-2.1.0-bin-nomad.tgz jars/spark-sql_2.11-2.1.0-SNAPSHOT.jar
```

Run a few commands:

```bash
$ CREATE TEMPORARY VIEW usersTable
USING org.apache.spark.sql.parquet
OPTIONS (
  path "/usr/local/bin/spark/examples/src/main/resources/users.parquet"
);

$ SELECT * FROM usersTable;
```

### pyspark

Start the shell:

```bash
pyspark \
  --master nomad \
  --conf spark.executor.instances=4 \
  --conf spark.nomad.sparkDistribution=https://s3.amazonaws.com/nomad-spark/spark-2.1.0-bin-nomad.tgz
```

Run a few commands:

```bash
$ df = spark.read.json("/usr/local/bin/spark/examples/src/main/resources/people.json")
$ df.show()
$ df.printSchema()
$ df.createOrReplaceTempView("people")
$ sqlDF = spark.sql("SELECT * FROM people")
$ sqlDF.show()
```
