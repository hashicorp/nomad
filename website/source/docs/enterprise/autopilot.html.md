---
layout: "docs"
page_title: "Advanced Autopilot"
sidebar_current: "docs-enterprise-autopilot"
description: |-
  Nomad Enterprise adds operations, collaboration, and governance capabilities
  to Nomad. Features include Namespaces, Resource Quotas, Sentinel Policies, and
  Advanced
---

# Advanced Autopilot

Nomad Enterprise Platform enables operators to easily upgrade Nomad as well as
enhances performance and availability through Advanced Autopilot features such
as Automated Upgrades, Enhanced Read Scalability, and Redundancy Zones.

### Automated Upgrades

Automated Upgrades allows operators to deploy a complete cluster of new servers
and then simply wait for the upgrade to complete. As the new servers join the
cluster, server logic checks the version of each Nomad server node. If the
version is higher than the version on the current set of voters, it will avoid
promoting the new servers to voters until the number of new servers matches the
number of existing servers at the previous version. Once the numbers match,
Nomad will begin to promote new servers and demote old ones.

See the [Autopilot - Upgrade Migrations](/guides/operations/autopilot.html#upgrade-migrations) documentation
for a thorough overview.

### Enhanced Read Scalability

This feature enables an operator to introduce non-voting server nodes to a Nomad
cluster. Non-voting servers will receive the replication stream but will not
take part in quorum (required by the leader before log entries can be
committed). Adding explicit non-voters will scale reads and scheduling without
impacting write latency.

See the [Autopilot - Read Scalability](/guides/operations/autopilot.html#server-read-and-scheduling-scaling) documentation for a thorough overview.

### Redundancy Zones

Redundancy Zones enables an operator to deploy a non-voting server as a hot
standby server on a per availability zone basis. For example, in an environment
with three availability zones an operator can run one voter and one non-voter in
each availability zone, for a total of six servers. If an availability zone is
completely lost, only one voter will be lost, so the cluster remains available.
If a voter is lost in an availability zone, Nomad will promote the non-voter to
a voter automatically, putting the hot standby server into service quickly.

See the [Autopilot - Redundancy Zones](/guides/operations/autopilot.html#redundancy-zones) documentation for a thorough overview.
