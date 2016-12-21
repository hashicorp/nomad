---
layout: "docs"
page_title: "Blue/Green & Canary Deployments - Operating a Job"
sidebar_current: "docs-operating-a-job-updating-blue-green-deployments"
description: |-
  Nomad supports blue/green and canary deployments through the declarative job
  file syntax. By specifying multiple task groups, Nomad allows for easy
  configuration and rollout of blue/green and canary deployments.
---

# Blue/Green &amp; Canary Deployments

Sometimes [rolling
upgrades](/docs/operating-a-job/update-strategies/rolling-upgrades.html) do not
offer the required flexibility for updating an application in production. Often
organizations prefer to put a "canary" build into production or utilize a
technique known as a "blue/green" deployment to ensure a safe application
rollout to production while minimizing downtime.

Blue/Green deployments have several other names including Red/Black or A/B, but
the concept is generally the same. In a blue/green deployment, there are two
application versions. Only one application version is active at a time, except
during the transition phase from one version to the next. The term "active"
tends to mean "receiving traffic" or "in service".

Imagine a hypothetical API server which has ten instances deployed to production
at version 1.3, and we want to safely upgrade to version 1.4. After the new
version has been approved to production, we may want to do a small rollout. In
the event of failure, we can quickly rollback to 1.3.

To start, version 1.3 is considered the active set and version 1.4 is the
desired set. Here is a sample job file which models the transition from version
1.3 to version 1.4 using a blue/green deployment.

```hcl
job "docs" {
  datacenters = ["dc1"]

  group "api-green" {
    count = 10

    task "api-server" {
      driver = "docker"

      config {
        image = "api-server:1.3"
      }
    }
  }

  group "api-blue" {
    count = 0

    task "api-server" {
      driver = "docker"

      config {
        image = "api-server:1.4"
      }
    }
  }
}
```

It is clear that the active group is "api-green" since it has a non-zero count.
To transition to v1.4 (api-blue), we increase the count of api-blue to match
that of api-green.

```diff
@@ -2,6 +2,8 @@ job "docs" {
 group "api-blue" {
-  count = 0
+  count = 10

   task "api-server" {
     driver = "docker"
```

Next we plan and run these changes:

```shell
$ nomad plan docs.nomad
```

Assuming the plan output looks okay, we are ready to run these changes.

```shell
$ nomad run docs.nomad
```

Our deployment is not yet finished. We are currently running at double capacity,
so approximately half of our traffic is going to the blue and half is going to
green. Usually we inspect our monitoring and reporting system. If we are
experiencing errors, we reduce the count of "api-blue" back to 0. If we are
running successfully, we change the count of "api-green" to 0.

```diff
@@ -2,6 +2,8 @@ job "docs" {
 group "api-green" {
-  count = 10
+  count = 0

   task "api-server" {
     driver = "docker"
```

The next time we want to do a deployment, the "green" group becomes our
transition group, since the "blue" group is currently active.

## Canary Deployments

A canary deployment is a special type of blue/green deployment in which a subset
of nodes continues to run in production for an extended period of time.
Sometimes this is done for logging/analytics or as an extended blue/green
deployment. Whatever the reason, Nomad supports canary deployments. Using the
same strategy as defined above, simply keep the "blue" at a lower number, for
example:

```hcl
job "docs" {
  datacenters = ["dc1"]

  group "api" {
    count = 10

    task "api-server" {
      driver = "docker"

      config {
        image = "api-server:1.3"
      }
    }
  }

  group "api-canary" {
    count = 1

    task "api-server" {
      driver = "docker"

      config {
        image = "api-server:1.4"
      }
    }
  }
}
```

Here you can see there is exactly one canary version of our application (v1.4)
and ten regular versions. Typically canary versions are also tagged
appropriately in the [service discovery](/docs/service-discovery/index.html)
layer to prevent unnecessary routing.
