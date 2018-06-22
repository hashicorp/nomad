---
layout: "docs"
page_title: "Nomad Enterprise Advanced Autopilot"
sidebar_current: "docs-enterprise-autopilot"
description: |-
  Nomad Enterprise supports Advanced Autopilot capabilities which enable fully 
  automated server upgrades, higher throughput for reads and scheduling, and hot 
  server failover on a per availability zone basis.
---

# Nomad Enterprise Advanced Autopilot

[Nomad Enterprise](https://www.hashicorp.com/go/nomad-enterprise) supports Advanced Autopilot capabilities which enable fully
automated server upgrades, higher throughput for reads and scheduling, and hot
server failover on a per availability zone basis. See the sections below for 
additional details on each of these capabilities. 

* **Automated Upgrades:** Advanced Autopilot enables an upgrade pattern that 
allows operators to deploy a complete cluster of new servers and then simply wait 
for the upgrade to complete. As the new servers join the cluster, server 
introduction logic checks the version of each Nomad server. If the version is 
higher than the version on the current set of voters, it will avoid promoting 
the new servers to voters until the number of new servers matches the number of 
existing servers at the previous version. Once the numbers match, Autopilot will 
begin to promote new servers and demote old ones.

* **Enhanced Read Scalability:** With Advanced Autopilot, servers can be 
explicitly marked as non-voters. Non-voters will receive the replication stream 
but will not take part in quorum (required by the leader before log entries can 
be committed). Adding explicit non-voters will scale reads and scheduling without 
impacting write latency.

* **Redundancy Zones:** Advanced Autopilot redundancy zones make it possible to 
have more servers than availability zones. For example, in an environment with 
three availability zones it's now possible to run one voter and one non-voter in 
each availability zone, for a total of six servers. If an availability zone is 
completely lost, only one voter will be lost, so the cluster remains available. 
If a voter is lost in an availability zone, Autopilot will promote the non-voter 
to voter automatically, putting the hot standby server into service quickly.

See the [Nomad Autopilot Guide](/guides/operations/autopilot.html)
for a comprehensive overview of Nomad's open source and enterprise Autopilot features.

Click [here](https://www.hashicorp.com/go/nomad-enterprise) to set up a demo or 
request a trial of Nomad Enterprise.
