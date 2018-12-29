---
layout: "docs"
page_title: "Nomad Enterprise Namespaces"
sidebar_current: "docs-enterprise-namespaces"
description: |-
  Nomad Enterprise provides support for namespaces, which allows jobs and their
  associated objects to be segmented from each other and other users of the
  cluster.
---

# Nomad Enterprise Namespaces

In [Nomad Enterprise](https://www.hashicorp.com/products/nomad/), a shared
cluster can be partioned into [namespaces](/guides/namespaces.html) which allows
jobs and their associated objects to be isolated from each other and other users
of the cluster.

Namespaces enhance the usability of a shared cluster by isolating teams from the
jobs of others, provide fine grain access control to jobs when coupled with
[ACLs](/guides/acl.html), and can prevent bad actors from negatively impacting
the whole cluster when used in conjunction with
[quotas](/docs/enterprise/quotas/index.html).
