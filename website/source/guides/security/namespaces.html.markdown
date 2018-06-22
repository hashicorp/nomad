---
layout: "guides"
page_title: "Namespaces"
sidebar_current: "guides-security-namespaces"
description: |-
  Nomad Enterprise provides support for namespaces, which allow jobs and their
  associated objects to be segmented from each other and other users of the
  cluster.
---

# Namespaces

[Nomad Enterprise](https://www.hashicorp.com/products/nomad/) has support for 
namespaces, which allow jobs and their associated objects to be segmented from 
each other and other users of the cluster.

~> **Enterprise Only!** This functionality only exists in Nomad Enterprise.
This is not present in the open source version of Nomad.

## Use Case

Namespaces allow a single cluster to be shared by many teams and projects
without conflict. Nomad requires job IDs to be unique within namespaces but not
across namespaces. This allows each team to operate independently of others.

When combined with ACLs, the isolation of namespaces can be enforced, only
allowing designated users access to read or modify the jobs and associated
objects in a namespace.

When [resource quotas](/guides/security/quotas.html) are applied to a namespace they
provide a means to limit resource consumption by the jobs in the namespace. This
can prevent a single actor from consuming excessive cluster resources and
negatively impacting other teams and applications sharing the cluster.

## Namespaced Objects

Nomad places all jobs and their derived objects into namespaces. These include
jobs, allocations, deployments, and evaluations. 

Nomad does not namespace objects that are shared across multiple namespaces.
This includes nodes, [ACL policies](/guides/security/acl.html), [Sentinel
policies](/guides/security/sentinel-policy.html), and [quota
specifications](/guides/security/quotas.html).

## Working with Namespaces

For specific details about working with namespaces, see the [namespace
commands](/docs/commands/namespace.html) and [HTTP API](/api/namespaces.html)
documentation.

### Creating and viewing namespaces:

Namespaces can be interacted with using the `nomad namespace` subcommand. The
following creates and lists the namespaces of a cluster:

```
$ nomad namespace apply -description "QA instances of webservers" web-qa
Successfully applied namespace "web-qa"!

$ nomad namespace list
Name      Description
default   Default shared namespace
api-prod  Production instances of backend API servers
api-qa    QA instances of backend API servers
web-prod  Production instances of webservers
web-qa    QA instances of webservers
```

### Running jobs

To run a job in a specific namespace, we annotate the job with the `namespace`
parameter. If omitted, the job will be run in the `default` namespace. Below is
an example of running the job in the newly created `web-qa` namespace:

```
job "rails-www" {

    # Run in the QA environments
    namespace = "web-qa"

    # Only run in one datacenter when QAing
    datacenters = ["us-west1"]
    ...
}
```

### Specifying desired namespace

When using commands that operate on objects that are namespaced, the namespace
can be specified either with the flag `-namespace` or read from the
`NOMAD_NAMESPACE` environment variable:

```
$ nomad job status -namespace=web-qa
ID         Type     Priority  Status   Submit Date
rails-www  service  50        running  09/17/17 19:17:46 UTC

$ export NOMAD_NAMESPACE=web-qa

$ nomad job status
ID         Type     Priority  Status   Submit Date
rails-www  service  50        running  09/17/17 19:17:46 UTC
```

### ACLs

Access to namespaces can be restricted using [ACLs](/guides/security/acl.html). As an
example we could create an ACL policy that allows full access to the QA
environment for our web namespaces but restrict the production access by
creating the following policy:

```
# Allow read only access to the production namespace
namespace "web-prod" {
    policy = "read"
}

# Allow writing to the QA namespace
namespace "web-qa" {
    policy = "write"
}
```
