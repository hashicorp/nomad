# Nomad / Spark integration

We maintain a fork of Apache Spark that natively supports using a Nomad cluster to run Spark applications. When running on Nomad, the Spark executors that run Spark tasks for your application, and optionally the application driver itself, run as Nomad tasks in a Nomad job. See the [usage guide](./RunningSparkOnNomad.pdf) for more details.

To give the Spark integration a test drive `cd` to `examples/spark/spark` on one of the servers (the `examples/spark/spark` subdirectory will be created when the cluster is provisioned).

A number of sample Spark commands are listed below. These demonstrate some of the official examples as well as features like `spark-sql`, `spark-shell` and dataframes.

You can monitor Nomad status simulaneously with:

```bash
$ nomad status
$ nomad status [JOB_ID]
$ nomad alloc-status [ALLOC_ID]
```

## Sample Spark commands

### SparkPi

Java (client mode)

```bash
$ ./bin/spark-submit --class org.apache.spark.examples.JavaSparkPi --master nomad --conf spark.executor.instances=8 --conf spark.nomad.sparkDistribution=https://s3.amazonaws.com/rcgenova-nomad-spark/spark-2.1.0-bin-nomad-preview-6.tgz examples/jars/spark-examples*.jar 100
```

Java (cluster mode)

```bash
$ ./bin/spark-submit --class org.apache.spark.examples.JavaSparkPi --master nomad --deploy-mode cluster --conf spark.executor.instances=4 --conf spark.nomad.cluster.monitorUntil=complete --conf spark.nomad.sparkDistribution=https://s3.amazonaws.com/rcgenova-nomad-spark/spark-2.1.0-bin-nomad-preview-6.tgz https://s3.amazonaws.com/rcgenova-nomad-spark/spark-examples_2.11-2.1.0-SNAPSHOT.jar 100
```

Python (client mode)

```bash
$ ./bin/spark-submit --master nomad --conf spark.executor.instances=8 --conf spark.nomad.sparkDistribution=https://s3.amazonaws.com/rcgenova-nomad-spark/spark-2.1.0-bin-nomad-preview-6.tgz examples/src/main/python/pi.py 100
```

Python (cluster mode)

```bash
$ ./bin/spark-submit --master nomad --deploy-mode cluster --conf spark.executor.instances=4 --conf spark.nomad.cluster.monitorUntil=complete --conf spark.nomad.sparkDistribution=https://s3.amazonaws.com/rcgenova-nomad-spark/spark-2.1.0-bin-nomad-preview-6.tgz examples/src/main/python/pi.py 100
```

Scala, (client mode)

```bash
$ ./bin/spark-submit --class org.apache.spark.examples.SparkPi --master nomad --conf spark.executor.instances=8 --conf spark.nomad.sparkDistribution=https://s3.amazonaws.com/rcgenova-nomad-spark/spark-2.1.0-bin-nomad-preview-6.tgz examples/jars/spark-examples*.jar 100
```

###  Machine Learning

Python (client mode)

```bash
$ ./bin/spark-submit --master nomad --conf spark.executor.instances=8 --conf spark.nomad.sparkDistribution=https://s3.amazonaws.com/rcgenova-nomad-spark/spark-2.1.0-bin-nomad-preview-6.tgz examples/src/main/python/ml/logistic_regression_with_elastic_net.py
```

Scala (client mode)

```bash
$ ./bin/spark-submit --class org.apache.spark.examples.SparkLR --master nomad --conf spark.executor.instances=8 --conf spark.nomad.sparkDistribution=https://s3.amazonaws.com/rcgenova-nomad-spark/spark-2.1.0-bin-nomad-preview-6.tgz examples/jars/spark-examples*.jar
```

### Streaming

Run these commands simultaneously:

```bash
$ bin/spark-submit --class org.apache.spark.examples.streaming.clickstream.PageViewGenerator --master nomad --deploy-mode cluster --conf spark.executor.instances=4 --conf spark.nomad.cluster.monitorUntil=complete --conf spark.nomad.sparkDistribution=https://s3.amazonaws.com/rcgenova-nomad-spark/spark-2.1.0-bin-nomad-preview-6.tgz https://s3.amazonaws.com/rcgenova-nomad-spark/spark-examples_2.11-2.1.0-SNAPSHOT.jar 44444 10
```

```bash
$ bin/spark-submit --class org.apache.spark.examples.streaming.clickstream.PageViewStream --master nomad --deploy-mode cluster --conf spark.executor.instances=4 --conf spark.nomad.cluster.monitorUntil=complete --conf spark.nomad.sparkDistribution=https://s3.amazonaws.com/rcgenova-nomad-spark/spark-2.1.0-bin-nomad-preview-6.tgz https://s3.amazonaws.com/rcgenova-nomad-spark/spark-examples_2.11-2.1.0-SNAPSHOT.jar errorRatePerZipCode localhost 44444
```

###  pyspark

```bash
$ ./bin/pyspark --master nomad --conf spark.executor.instances=8 --conf spark.nomad.sparkDistribution=https://s3.amazonaws.com/rcgenova-nomad-spark/spark-2.1.0-bin-nomad-preview-6.tgz
```

```bash
$ df = spark.read.json("examples/src/main/resources/people.json")
$ df.show()
$ df.printSchema()
$ df.createOrReplaceTempView("people")
$ sqlDF = spark.sql("SELECT * FROM people")
$ sqlDF.show()
```

### spark-shell

```bash
$ ./bin/spark-shell --master nomad --conf spark.executor.instances=8 --conf spark.nomad.sparkDistribution=https://s3.amazonaws.com/rcgenova-nomad-spark/spark-2.1.0-bin-nomad-preview-6.tgz
```

From spark-shell:

```bash
$ :type spark
$ spark.version

$ val data = 1 to 10000
$ val distData = sc.parallelize(data)
$ distData.filter(_ < 10).collect()
```

### spark-sql

```bash
$ bin/spark-sql --master nomad --conf spark.executor.instances=8 --conf spark.nomad.sparkDistribution=https://s3.amazonaws.com/rcgenova-nomad-spark/spark-2.1.0-bin-nomad-preview-6.tgz jars/spark-sql_2.11-2.1.0-SNAPSHOT.jar
```

From spark-shell:

```bash
CREATE TEMPORARY VIEW usersTable
USING org.apache.spark.sql.parquet
OPTIONS (
  path "examples/src/main/resources/users.parquet"
);

SELECT * FROM usersTable;
```

### Data frames

```bash
$ bin/spark-shell --master nomad --conf spark.executor.instances=8 --conf spark.nomad.sparkDistribution=https://s3.amazonaws.com/rcgenova-nomad-spark/spark-2.1.0-bin-nomad-preview-6.tgz
```

From spark-shell:

```bash
$ val usersDF = spark.read.load("examples/src/main/resources/users.parquet")
$ usersDF.select("name", "favorite_color").write.save("/tmp/namesAndFavColors.parquet")
```
