---
layout: "guides"
page_title: "Running Apache Spark on Nomad"
sidebar_current: "guides-analytical-workloads-spark-intro"
description: |-
  Learn how to run Apache Spark on a Nomad cluster.
---

# Running Apache Spark on Nomad

Apache Spark is a popular data processing engine/framework that has been
architected to use third-party schedulers. The Nomad ecosystem includes a
[fork of Apache Spark](https://github.com/hashicorp/nomad-spark) that natively
integrates Nomad as a cluster manager and scheduler for Spark. When running on
Nomad, the Spark executors that run Spark tasks for your application, and
optionally the application driver itself, run as Nomad tasks in a Nomad job.

## Next Steps

The links in the sidebar contain detailed information about specific aspects of
the integration, beginning with [Getting Started](/guides/analytical-workloads/spark/pre.html).
