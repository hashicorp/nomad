---
layout: "guides"
page_title: "Resource Quotas"
sidebar_current: "guides-security-quotas"
description: |-
  Nomad Enterprise provides support for resource quotas, which allow operators
  to restrict the aggregate resource usage of namespaces.
---

# Resource Quotas

[Nomad Enterprise](https://www.hashicorp.com/products/nomad/) provides support 
for resource quotas, which allow operators to restrict the aggregate resource 
usage of namespaces.

~> **Enterprise Only!** This functionality only exists in Nomad Enterprise.
This is not present in the open source version of Nomad.

## Use Case

When many teams or users are sharing Nomad clusters, there is the concern that a
single user could use more than their fair share of resources. Resource quotas
provide a mechanism for cluster administrators to restrict the resources that a
[namespace](/guides/security/namespaces.html) has access to.

## Quotas Objects

Quota specifications are first class objects in Nomad. A quota specification
has a unique name, an optional human readable description and a set of quota
limits. The quota limits defines the allowed resource usage within a region.

Quota objects are shareable among namespaces. This allows an operator to define
higher level quota specifications, such as a `prod-api` quota, and multiple
namespaces can apply the `prod-api` quota specification.

When a quota specification is attached to a namespace, all resource usage by
jobs in the namespaces are accounted toward the quota limits. If the resource is
exhausted, allocations with the namespaces will be queued until resources become
available by either other jobs finishing or the quota being expanded.

## Working with Quotas

For specific details about working with quotas, see the [quotas
commands](/docs/commands/quota.html) and [HTTP API](/api/quotas.html)
documentation.

### Creating quotas:

Resource quotas can be interacted with using the `nomad quota` subcommand. To
get started with creating a quota specification, run `nomad quota init` which
produces an example quota specification:

```
$ nomad quota init
Example quota specification written to spec.hcl

$ cat spec.hcl
name = "default-quota"
description = "Limit the shared default namespace"

# Create a limit for the global region. Additional limits may
# be specified in-order to limit other regions.
limit {
    region = "global"
    region_limit {
        cpu = 2500
        memory = 1000
    }
}
```

A quota specification is composed of one or more resource limits. Each limit
applies to a particular Nomad region. Within the limit object, operators can
specify the allowed CPU and memory usage.

To create the particular quota, it is as simple as running:

```
$ nomad quota apply spec.hcl
Successfully applied quota specification "default-quota"!

$ nomad quota list
Name           Description
default-quota  Limit the shared default namespace
api-prod       Production instances of backend API servers
api-qa         QA instances of backend API servers
web-prod       Production instances of webservers
web-qa         QA instances of webservers
```

### Attaching Quotas to Namespaces

In order for a quota to be enforced, we have to attach the quota specification
to a namespace. This can be done using the `nomad namespace apply` command.
We could add the quota specification we just created to the `default` namespace
as follows:

```
$ nomad namespace apply -quota default-quota default
Successfully applied namespace "default"!
```

### Viewing Quotas

Lets now run a job in the default namespace now that we have attached a quota:

```
$ nomad job init
Example job file written to example.nomad

$ nomad job run -detach example.nomad
Job registration successful
Evaluation ID: 985a1df8-0221-b891-5dc1-4d31ad4e2dc3

$ nomad quota status default-quota
Name        = default-quota
Description = Limit the shared default namespace
Limits      = 1

Quota Limits
Region  CPU Usage   Memory Usage
global  500 / 2500  256 / 1000
```

We can see the newly created job is accounted against the quota specification
since it is being run in a namespace that has attached the quota. Now let us
scale up the job from `count = 1` to `count = 4`:

```
# Change count
$ nomad job run -detach example.nomad
Job registration successful
Evaluation ID: ce8e1941-0189-b866-3dc4-7cd92dc38a69

$ nomad status example
ID            = example
Name          = example
Submit Date   = 10/16/17 10:51:32 PDT
Type          = service
Priority      = 50
Datacenters   = dc1
Status        = running
Periodic      = false
Parameterized = false

Summary
Task Group  Queued  Starting  Running  Failed  Complete  Lost
cache       1       0         3        0       0         0

Placement Failure
Task Group "cache":
  * Quota limit hit "memory exhausted (1024 needed > 1000 limit)"

Latest Deployment
ID          = 7cd98a69
Status      = running
Description = Deployment is running

Deployed
Task Group  Desired  Placed  Healthy  Unhealthy
cache       4        3       0        0

Allocations
ID        Node ID   Task Group  Version  Desired  Status   Created At
6d735236  81f72d90  cache       1        run      running  10/16/17 10:51:32 PDT
ce8e1941  81f72d90  cache       1        run      running  10/16/17 10:51:32 PDT
9b8e185e  81f72d90  cache       1        run      running  10/16/17 10:51:24 PDT
```

Here we can see Nomad created two more allocations but did not place the fourth
allocation since that would cause the quota to be oversubscribed on memory.

### ACLs

Access to quotas can be restricted using [ACLs](/guides/security/acl.html). As an
example we could create an ACL policy that allows read-only access to quotas.

```
# Allow read only access to quotas.
quota {
    policy = "read"
}
```

Creating or modifying quotas should typically be guarded by ACLs such that users
can not bypass enforcement by simply increasing or removing the quota
specification.

## Resource Limits

When specifying resource limits the following enforcement behaviors are defined:

* `limit < 0`: A limit less than zero disallows any access to the resource.

* `limit == 0`: A limit of zero allows unlimited access to the resource.

* `limit > 0`: A limit greater than zero enforces that the consumption is less
  than or equal to the given limit.

## Federation

Nomad makes working with quotas in a federated cluster simple by replicating
quota specifications from the [authoritative Nomad
region](/docs/configuration/server.html#authoritative_region). This allows
operators to interact with a single cluster but create quota specifications that
apply to all Nomad clusters.

As an example, we can create a quota specification that applies to two regions:

```
name = "federated-example"
description = "A single quota spec effecting multiple regions"

# Create a limits for two regions
limit {
    region = "europe"
    region_limit {
        cpu = 20000
        memory = 10000
    }
}

limit {
    region = "asia"
    region_limit {
        cpu = 10000
        memory = 5000
    }
}
```

If we apply this, and attach it to a namespace with jobs in each region, we can
see how the enforcement applies across federated clusters.

```
$ nomad quota apply spec.hcl
Successfully applied quota specification "federated-example"!

$ nomad quota status federated-example
Name        = federated-example
Description = A single quota spec effecting multiple regions
Limits      = 2

Quota Limits
Region  CPU Usage     Memory Usage
asia    2500 / 10000  1000 / 5000
europe  8800 / 20000  6000 / 10000
```
