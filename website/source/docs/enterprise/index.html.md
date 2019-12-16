---
layout: "docs"
page_title: "Nomad Enterprise"
sidebar_current: "docs-enterprise"
description: |-
  Nomad Enterprise adds operations, collaboration, and governance capabilities to Nomad.
  Features include Namespaces, Resource Quotas, Sentinel Policies, and Advanced Autopilot.
---

# Nomad Enterprise

Nomad Enterprise adds collaboration, operational, and governance capabilities to
Nomad.  Nomad Enterprise is available as a base Platform package with an
optional Governance & Policy add-on module.

Please navigate the sub-sections for more information about each package and its
features in detail.

## Nomad Enterprise Platform

### Advanced Autopilot

Nomad Enterprise Platform enables operators to easily upgrade Nomad as well as
enhances performance and availability through Advanced Autopilot features such
as Automated Upgrades, Enhanced Read Scalability, and Redundancy Zones.

See the [Advanced Autopilot Documentation](/docs/enterprise/autopilot.html) for
a thorough overview.

## Governance & Policy

Governance & Policy features are part of an add-on module that enables an
organization to securely operate Nomad at scale across multiple teams through
features such as Namespaces, Resource Quotas, Sentinel Policies, and Preemption.

### Namespaces

Namespaces enable multiple teams to safely use a shared multi-region Nomad
environment and reduce cluster fleet size. In Nomad Enterprise, a shared cluster
can be partitioned into multiple namespaces which allow jobs and their
associated objects to be isolated from each other and other users of the
cluster.

Namespaces enhance the usability of a shared cluster by isolating teams from the
jobs of others, by providing fine grain access control to jobs when coupled with
ACLs, and by preventing bad actors from negatively impacting the whole cluster.

See the [Namespaces Guide](/guides/governance-and-policy/namespaces.html) for a
thorough overview.

### Resource Quotas

Resource Quotas enable an operator to limit resource consumption across teams or
projects to reduce waste and align budgets. In Nomad Enterprise, operators can
define quota specifications and apply them to namespaces. When a quota is
attached to a namespace, the jobs within the namespace may not consume more
resources than the quota specification allows.

This allows operators to partition a shared cluster and ensure that no single
actor can consume the whole resources of the cluster.

See the [Resource Quotas Guide](/guides/governance-and-policy/quotas.html) for a
thorough overview.

### Sentinel Policies

In Nomad Enterprise, operators can create Sentinel policies for fine-grained
policy enforcement. Sentinel policies build on top of the ACL system and allow
operators to define policies such as disallowing jobs to be submitted to
production on Fridays or only allowing users to run jobs that use pre-authorized
Docker images. Sentinel policies are defined as code, giving operators
considerable flexibility to meet compliance requirements.

See the [Sentinel Documentation](/docs/enterprise/sentinel.html) for a thorough
overview.

### Preemption

When a Nomad cluster is at capacity for a given set of placement constraints,
any allocations that result from a newly scheduled service or batch job will
remain in the pending state until sufficient resources become available -
regardless of the defined priority.

Preemption enables Nomad's scheduler to automatically evict lower priority
allocations of service and batch jobs so that allocations from higher priority
jobs can be placed. This behavior ensures that critical workloads can run when
resources are limited or when partial outages require workloads to be
rescheduled across a smaller set of client nodes.

## Try Nomad Enterprise

Click [here](https://www.hashicorp.com/go/nomad-enterprise) to set up a demo or
request a trial of Nomad Enterprise.
