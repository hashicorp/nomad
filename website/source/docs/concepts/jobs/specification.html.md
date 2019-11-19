---
layout: "docs"
page_title: "Job Specifications"
sidebar_current: "docs-concepts-jobs-specification"
description: |-
  A survey of core concepts for the Nomad Job Specification.
---

# Job Specifications

The Nomad job specification (or "jobspec" for short) defines the schema for
Nomad jobs. Nomad jobs are specified in [HCL], which aims to strike a balance
between human readable and editable, and machine-friendly. 

Nomad can read JSON-equivalent job specifications; however, we generally
recommend using the HCL syntax where possible.

## HCL in a nutshell

Nomad HCL job specifications contains a few structural elements:

- **Root** - the entire content of the specification; interpreted as a Body.

- **Body** - a collection of associated Attributes and Stanzas.

- **Attributes** - an assignment of a value to a specified name.

- **Stanzas** - a child body annotated by a type and optional labels.

- **Label** - either quoted literal strings or naked identifiers depending on
  the stanza. Most stanza labels in Nomad are quoted literal strings.


## Job structure

The required hierarchy for stanzas within a job is:

```text
job
  \_ group
        \_ task
```

Each job file defines a single job; however, a job may have multiple groups, and
each group may have multiple tasks. Groups contain a set of tasks that are
co-located on a machine.

## The `job` stanza

The [`job` stanza] is the top-most configuration option in the job specification.
A job is a declarative specification of tasks that Nomad should run. Jobs have
one or more groups, which are themselves collections of one or more tasks.

The name of a job is the primary identifier for that job specification within
a region namespace and must be unique within that scope. 

~> Nomad open-source supports a single, default namespace, while Nomad
   Enterprise supports multiple namespaces.

The job stanza is labeled with the name of the job.  For example, a job named
"redis" would look like the following:

```hcl
job "redis" {}
```

The job stanza body must contain one or more group stanzas.

## The `group` stanza

The [`group` stanza] defines a series of tasks that should be co-located on the
same Nomad client. Any task within a group will be placed on the same client.
When an instance of this job is created, these tasks will run within the same
allocation.

Adding a group to the example HCL from above, would look like this:

```hcl
job "redis" {
  group "cache" {}
}
```

The job stanza body must contain one or more task stanzas.

## The `task` stanza

The [`task` stanza] creates an individual unit of work, such as a Docker
container, web application, or batch processing. A task must specify a `driver`.
These [task drivers] have individual requirements which should be consulted when
designing your own job specification.

Task stanzas are labeled with the task name similar to job and group. This name
is used when the task instances are created at run-time.

For this example, the task will use the [`docker` driver] to start a Redis Docker
container:

```hcl
job "redis" {
  group "cache" {
    task "container" {
      driver="docker"

    }
  }
}
```

### type

### driver

### count


## Learn more about the Nomad job specification

There is exhaustive documentation of the Nomad [job specification]. You should
consult this documentation when reading and designing Nomad job specifications
for your own workloads.

[`docker` driver]: /docs/drivers/docker.html
[`group` stanza]: /docs/job-specification/group.html
[`job` stanza]: /docs/job-specification/job.html
[`task` stanza]: /docs/job-specification/task.html
[hcl]: https://github.com/hashicorp/hcl "HashiCorp Configuration Language"
[job specification]: /docs/job-specification/index.html
[task drivers]: /docs/drivers/index.html
[`driver` argument]: /docs/job-specification/task.html#driver