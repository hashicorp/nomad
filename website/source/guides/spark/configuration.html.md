---
layout: "guides"
page_title: "Apache Spark Integration - Configuration Properties"
sidebar_current: "guides-spark-configuration"
description: |-
  Comprehensive list of Spark configuration properties.
---

# Spark Configuration Properties

Spark [configuration properties](https://spark.apache.org/docs/latest/configuration.html#available-properties) 
are generally applicable to the Nomad integration. The properties listed below 
 are specific to running Spark on Nomad. Configuration properties can be set by 
 adding `--conf [property]=[value]` to the `spark-submit` command.

- `spark.nomad.cluster.expectImmediateScheduling` `(bool: false)` - Specifies 
that `spark-submit` should fail if Nomad is not able to schedule the job 
immediately.

- `spark.nomad.cluster.monitorUntil` `(string: "submitted"`) - Specifies the 
length of time that `spark-submit` should monitor a Spark application in cluster
 mode. When set to `submitted`, `spark-submit` will return as soon as the 
 application has been submitted to the Nomad cluster. When set to `scheduled`, 
 `spark-submit` will return as soon as the Nomad job has been scheduled. When 
 set to `complete`, `spark-submit` will tail the output from the driver process 
 and return when the job has completed.

- `spark.nomad.datacenters` `(string: dynamic)` - Specifies a comma-separated 
list of Nomad datacenters to use. This property defaults to the datacenter of 
the first Nomad server contacted.

- `spark.nomad.docker.email` `(string: nil)` - Specifies the email address to 
use when downloading the Docker image specified by 
[spark.nomad.dockerImage](#spark.nomad.dockerImage). See the 
[Docker driver authentication](https://www.nomadproject.io/docs/drivers/docker.html#authentication) 
docs for more information.

-  `spark.nomad.docker.password` `(string: nil)` - Specifies the password to use
  when downloading the Docker image specified by 
  [spark.nomad.dockerImage](#spark.nomad.dockerImage). See the 
[Docker driver authentication](https://www.nomadproject.io/docs/drivers/docker.html#authentication) 
docs for more information.

- `spark.nomad.docker.serverAddress` `(string: nil)` - Specifies the server 
address (domain/IP without the protocol) to use when downloading the Docker 
image specified by [spark.nomad.dockerImage](#spark.nomad.dockerImage). Docker 
Hub is used by default. See the 
[Docker driver authentication](https://www.nomadproject.io/docs/drivers/docker.html#authentication) 
docs for more information.

- `spark.nomad.docker.username` `(string: nil)` - Specifies the username to use
 when downloading the Docker image specified by 
 [spark.nomad.dockerImage](#spark-nomad-dockerImage). See the 
[Docker driver authentication](https://www.nomadproject.io/docs/drivers/docker.html#authentication) 
docs for more information.

- `spark.nomad.dockerImage` `(string: nil)` - Specifies the `URL` for the 
[Docker image](https://www.nomadproject.io/docs/drivers/docker.html#image) to 
use to run Spark with Nomad's `docker` driver. When not specified, Nomad's 
`exec` driver will be used instead.

- `spark.nomad.driver.cpu` `(string: "1000")` - Specifies the CPU in MHz that 
should be reserved for driver tasks.

- `spark.nomad.driver.logMaxFileSize` `(string: "1m")` - Specifies the maximum 
size by time that Nomad should use for driver task log files.

- `spark.nomad.driver.logMaxFiles` `(string: "5")` - Specifies the number of log
 files that Nomad should keep for driver tasks.

- `spark.nomad.driver.networkMBits` `(string: "1")` - Specifies the network 
bandwidth that Nomad should allocate to driver tasks.

- `spark.nomad.driver.retryAttempts` `(string: "5")` - Specifies the number of 
times that Nomad should retry driver task groups upon failure.

- `spark.nomad.driver.retryDelay` `(string: "15s")` - Specifies the length of 
time that Nomad should wait before retrying driver task groups upon failure.

- `spark.nomad.driver.retryInterval` `(string: "1d")` - Specifies Nomad's retry 
interval for driver task groups.

- `spark.nomad.executor.cpu` `(string: "1000")` - Specifies the CPU in MHz that 
should be reserved for executor tasks.

- `spark.nomad.executor.logMaxFileSize` `(string: "1m")` - Specifies the maximum
 size by time that Nomad should use for executor task log files.

- `spark.nomad.executor.logMaxFiles` `(string: "5")` - Specifies the number of 
log files that Nomad should keep for executor tasks.

- `spark.nomad.executor.networkMBits` `(string: "1")` - Specifies the network 
bandwidth that Nomad should allocate to executor tasks.

- `spark.nomad.executor.retryAttempts` `(string: "5")` - Specifies the number of
 times that Nomad should retry executor task groups upon failure.

- `spark.nomad.executor.retryDelay` `(string: "15s")` - Specifies the length of 
time that Nomad should wait before retrying executor task groups upon failure.

- `spark.nomad.executor.retryInterval` `(string: "1d")` - Specifies Nomad's retry 
interval for executor task groups.

- `spark.nomad.job` `(string: nil)` - Specifies the Nomad job name.

- `spark.nomad.job.template` `(string: nil)` - Specifies the path to a JSON file 
containing a Nomad job to use as a template. This can also be set with 
`spark-submit's --nomad-template` parameter.

- `spark.nomad.priority` `(string: nil)` - Specifies the priority for the 
Nomad job.

- `spark.nomad.region` `(string: dynamic)` - Specifies the Nomad region to use. 
This property defaults to the region of the first Nomad server contacted.

- `spark.nomad.shuffle.cpu` `(string: "1000")` - Specifies the CPU in MHz that 
should be reserved for shuffle service tasks.

- `spark.nomad.shuffle.logMaxFileSize` `(string: "1m")` - Specifies the maximum
 size by time that Nomad should use for shuffle service task log files..

- `spark.nomad.shuffle.logMaxFiles` `(string: "5")` - Specifies the number of 
log files that Nomad should keep for shuffle service tasks.

- `spark.nomad.shuffle.memory` `(string: "256m")` - Specifies the memory that 
Nomad should allocate for the shuffle service tasks.

- `spark.nomad.shuffle.networkMBits` `(string: "1")` - Specifies the network 
bandwidth that Nomad should allocate to shuffle service tasks.

- `spark.nomad.sparkDistribution` `(string: nil)` - Specifies the location of 
the Spark distribution archive file to use.

- `spark.nomad.tls.caCert` `(string: nil)` - Specifies the path to a `.pem` file
 containing the certificate authority that should be used to validate the Nomad 
 server's TLS certificate.

- `spark.nomad.tls.cert` `(string: nil)` - Specifies the path to a `.pem` file 
containing the TLS certificate to present to the Nomad server.

- `spark.nomad.tls.trustStorePassword` `(string: nil)` - Specifies the path to a
 `.pem` file containing the private key corresponding to the certificate in 
[spark.nomad.tls.cert](#spark-nomad-tls-cert).

