---
layout: "docs"
page_title: "Nomad Enterprise Sentinel Policy Enforcement"
sidebar_current: "docs-enterprise-sentinel"
description: |-
  Nomad Enterprise provides support for policy enforcement using Sentinel.
---

# Nomad Enterprise Sentinel Policy Enforcement

In [Nomad Enterprise](https://www.hashicorp.com/products/nomad/), operators can
create [Sentinel policies](/guides/sentinel-policy.html) for fine grain policy
enforcement.  Sentinel policies build on top of the ACL system and allow operators to define
fine grain policies such as disallowing jobs to be submitted to production on
Fridays. These extremely rich policies are defined as code. For example, to
restrict jobs to only using the Docker driver, the operator would define and apply
the following policy:

```
# Only allows Docker based tasks
main = rule { all_drivers_docker }

# all_drivers_docker checks that all the drivers in use are Docker
all_drivers_docker = rule {
    all job.task_groups as tg {
        all tg.tasks as task {
            task.driver is "docker"
        }
    }
}
```
