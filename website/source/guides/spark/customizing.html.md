---
layout: "guides"
page_title: "Apache Spark Integration - Customizing Applications"
sidebar_current: "guides-spark-customizing"
description: |-
  Learn how to customize the Nomad job that is created to run a Spark 
  application.
---

# Customizing Applications

There are two ways to customize the Nomad job that Spark creates to run an 
application:

 - Use the default job template and set configuration properties
 - Use a custom job template

## Using the Default Job Template

The Spark integration will use a generic job template by default. The template 
includes groups and tasks for the driver, executors and (optionally) the 
[shuffle service](/guides/spark/dynamic.html). The job itself and the tasks that
 are created have the `spark.nomad.role` meta value defined accordingly:

```hcl
job "structure" {
  meta {
    "spark.nomad.role" = "application"
  }

  # A driver group is only added in cluster mode
  group "driver" {
    task "driver" {
      meta {
        "spark.nomad.role" = "driver"
      }
    }
  }

  group "executors" {
    count = 2
    task "executor" {
      meta {
        "spark.nomad.role" = "executor"
      }
    }

    # Shuffle service tasks are only added when enabled (as it must be when 
    # using dynamic allocation)
    task "shuffle-service" {
      meta {
        "spark.nomad.role" = "shuffle"
      }
    }
  }
}
```

The default template can be customized indirectly by explicitly [setting 
configuration properties](/guides/spark/configuration.html).

## Using a Custom Job Template

An alternative to using the default template is to set the 
`spark.nomad.job.template` configuration property to the path of a file 
containing a custom job template. There are two important considerations:

  * The template must use the JSON format. You can convert an HCL jobspec to 
  JSON by running `nomad run -output <job.nomad>`.

  * `spark.nomad.job.template` should be set to a path on the submitting 
  machine, not to a URL (even in cluster mode). The template does not need to 
  be accessible to the driver or executors.

Using a job template you can override Spark’s default resource utilization, add 
additional metadata or constraints, set environment variables, add sidecar 
tasks and utilize the Consul and Vault integration. The template does 
not need to be a complete Nomad job specification, since Spark will add 
everything necessary to run your the application. For example, your template 
might set `job` metadata, but not contain any task groups, making it an 
incomplete Nomad job specification but still a valid template to use with Spark.

To customize the driver task group, include a task group in your template that 
has a task that contains a `spark.nomad.role` meta value set to `driver`.

To customize the executor task group, include a task group in your template that 
has a task that contains a `spark.nomad.role` meta value set to `executor` or 
`shuffle`.

The following template adds a `meta` value at the job level and an environment 
variable to the executor task group:

```hcl
job "template" {

  meta {
    "foo" = "bar"
  }

  group "executor-group-name" {

    task "executor-task-name" {
      meta {
        "spark.nomad.role" = "executor"
      }

      env {
        BAZ = "something"
      }
    }
  }
}
```

## Order of Precedence

The order of precedence for customized settings is as follows:

1. Explicitly set configuration properties.
2. Settings in the job template (if provided).
3. Default values of the configuration properties.

## Next Steps

Learn how to [allocate resources](/guides/spark/resource.html) for your Spark 
applications.
