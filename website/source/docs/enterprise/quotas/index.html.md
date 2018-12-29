---
layout: "docs"
page_title: "Nomad Enterprise Resource Quotas"
sidebar_current: "docs-enterprise-quotas"
description: |-
  Nomad Enterprise provides support for applying resource quotas to namespaces
  which restricts the overall resources that jobs within the namespace are
  allowed to consume.
---

# Nomad Enterprise Resource Quotas

In [Nomad Enterprise](https://www.hashicorp.com/products/nomad/), operators can
define [quota specifications](/guides/quotas.html) and apply them to namespaces.
When a quota is attached to a namespace, the jobs within the namespace may not
consume more resources than the quota specification allows.

This allows operators to partition a shared cluster and ensure that no single
actor can consume the whole resources of the cluster.

See the [Resource Quotas Guide](/guides/quotas.html) for more details.
