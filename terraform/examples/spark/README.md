## Spark integration

`cd` to `examples/spark/spark` on one of the servers. The `spark/spark` subdirectory will be created when the cluster is provisioned.

You can use the spark-submit commands below to run several of the official Spark examples against Nomad. You can monitor Nomad status simulaneously with:

```bash
$ nomad status
$ nomad status [JOB_ID]
$ nomad alloc-status [ALLOC_ID]
```

### SparkPi

Java

```bash
$ ./bin/spark-submit --class org.apache.spark.examples.JavaSparkPi --master nomad --conf spark.executor.instances=8 --conf spark.nomad.sparkDistribution=https://s3.amazonaws.com/rcgenova-nomad-spark/spark-2.1.0-SNAPSHOT-bin-nomad-spark.tgz examples/jars/spark-examples*.jar 100
```

Python

```bash
$ ./bin/spark-submit --master nomad --conf spark.executor.instances=8 --conf spark.nomad.sparkDistribution=https://s3.amazonaws.com/rcgenova-nomad-spark/spark-2.1.0-SNAPSHOT-bin-nomad-spark.tgz examples/src/main/python/pi.py 100
```

Scala

```bash
$ ./bin/spark-submit --class org.apache.spark.examples.SparkPi --master nomad --conf spark.executor.instances=8 --conf spark.nomad.sparkDistribution=https://s3.amazonaws.com/rcgenova-nomad-spark/spark-2.1.0-SNAPSHOT-bin-nomad-spark.tgz examples/jars/spark-examples*.jar 100
```

## Machine Learning

Python

```bash
$ ./bin/spark-submit --master nomad --conf spark.executor.instances=8 --conf spark.nomad.sparkDistribution=https://s3.amazonaws.com/rcgenova-nomad-spark/spark-2.1.0-SNAPSHOT-bin-nomad-spark.tgz examples/src/main/python/ml/logistic_regression_with_elastic_net.py
```

Scala

```bash
$ ./bin/spark-submit --class org.apache.spark.examples.SparkLR --master nomad --conf spark.executor.instances=8 --conf spark.nomad.sparkDistribution=https://s3.amazonaws.com/rcgenova-nomad-spark/spark-2.1.0-SNAPSHOT-bin-nomad-spark.tgz examples/jars/spark-examples*.jar
```

## pyspark

```bash
$ ./bin/pyspark --master nomad --conf spark.executor.instances=8 --conf spark.nomad.sparkDistribution=https://s3.amazonaws.com/rcgenova-nomad-spark/spark-2.1.0-SNAPSHOT-bin-nomad-spark.tgz

df = spark.read.json("examples/src/main/resources/people.json")
df.show()
df.printSchema()
df.createOrReplaceTempView("people")
sqlDF = spark.sql("SELECT * FROM people")
sqlDF.show()
```

## spark-shell

```bash
$ ./bin/spark-shell --master nomad --conf spark.executor.instances=8 --conf spark.nomad.sparkDistribution=https://s3.amazonaws.com/rcgenova-nomad-spark/spark-2.1.0-SNAPSHOT-bin-nomad-spark.tgz

:type spark
spark.version

val data = 1 to 10000
val distData = sc.parallelize(data)
distData.filter(_ < 10).collect()
```

## spark-sql

```bash
$ ./bin/spark-sql --master nomad --conf spark.executor.instances=8 --conf spark.nomad.sparkDistribution=https://s3.amazonaws.com/rcgenova-nomad-spark/spark-2.1.0-SNAPSHOT-bin-nomad-spark.tgz jars/spark-sql_2.11-2.1.0-SNAPSHOT.jar
```


