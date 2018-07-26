---
layout: "guides"
page_title: "Autopilot"
sidebar_current: "guides-operations-autopilot"
description: |-
  This guide covers how to configure and use Autopilot features.
---

# Autopilot

Autopilot is a set of new features added in Nomad 0.8 to allow for automatic
operator-friendly management of Nomad servers. It includes cleanup of dead
servers, monitoring the state of the Raft cluster, and stable server introduction.

To enable Autopilot features (with the exception of dead server cleanup),
the `raft_protocol` setting in the [server stanza](/docs/configuration/server.html)
must be set to 3 on all servers. In Nomad 0.8 this setting defaults to 2; in Nomad 0.9 it will default to 3.
For more information, see the [Version Upgrade section](/guides/operations/upgrade/upgrade-specific.html#raft-protocol-version-compatibility)
on Raft Protocol versions.

## Configuration

The configuration of Autopilot is loaded by the leader from the agent's
[Autopilot settings](/docs/configuration/autopilot.html) when initially
bootstrapping the cluster:

```
autopilot {
    cleanup_dead_servers = true
    last_contact_threshold = 200ms
    max_trailing_logs = 250
    server_stabilization_time = "10s"
    enable_redundancy_zones = false
    disable_upgrade_migration = false
    enable_custom_upgrades = false
}
```

After bootstrapping, the configuration can be viewed or modified either via the
[`operator autopilot`](/docs/commands/operator.html) subcommand or the
[`/v1/operator/autopilot/configuration`](/api/operator.html#read-autopilot-configuration)
HTTP endpoint:

```
$ nomad operator autopilot get-config
CleanupDeadServers = true
LastContactThreshold = 200ms
MaxTrailingLogs = 250
ServerStabilizationTime = 10s
EnableRedundancyZones = false
DisableUpgradeMigration = false
EnableCustomUpgrades = false

$ nomad operator autopilot set-config -cleanup-dead-servers=false
Configuration updated!

$ nomad operator autopilot get-config
CleanupDeadServers = false
LastContactThreshold = 200ms
MaxTrailingLogs = 250
ServerStabilizationTime = 10s
EnableRedundancyZones = false
DisableUpgradeMigration = false
EnableCustomUpgrades = false
```

## Dead Server Cleanup

Dead servers will periodically be cleaned up and removed from the Raft peer
set, to prevent them from interfering with the quorum size and leader elections.
This cleanup will also happen whenever a new server is successfully added to the
cluster.

Prior to Autopilot, it would take 72 hours for dead servers to be automatically reaped,
or operators had to script a `nomad force-leave`. If another server failure occurred,
it could jeopardize the quorum, even if the failed Nomad server had been automatically
replaced. Autopilot helps prevent these kinds of outages by quickly removing failed
servers as soon as a replacement Nomad server comes online. When servers are removed
by the cleanup process they will enter the "left" state.

This option can be disabled by running `nomad operator autopilot set-config`
with the `-cleanup-dead-servers=false` option.

## Server Health Checking

An internal health check runs on the leader to track the stability of servers.
A server is considered healthy if all of the following conditions are true:

- Its status according to Serf is 'Alive'
- The time since its last contact with the current leader is below
`LastContactThreshold`
- Its latest Raft term matches the leader's term
- The number of Raft log entries it trails the leader by does not exceed
`MaxTrailingLogs`

The status of these health checks can be viewed through the 
[`/v1/operator/autopilot/health`](/api/operator.html#read-health) HTTP endpoint, with
a top level `Healthy` field indicating the overall status of the cluster:

```
$ curl localhost:8500/v1/operator/autopilot/health
{
    "Healthy": true,
    "FailureTolerance": 0,
    "Servers": [
        {
            "ID": "e349749b-3303-3ddf-959c-b5885a0e1f6e",
            "Name": "node1",
            "Address": "127.0.0.1:4647",
            "SerfStatus": "alive",
            "Version": "0.8.0",
            "Leader": true,
            "LastContact": "0s",
            "LastTerm": 2,
            "LastIndex": 10,
            "Healthy": true,
            "Voter": true,
            "StableSince": "2017-03-28T18:28:52Z"
        },
        {
            "ID": "e35bde83-4e9c-434f-a6ef-453f44ee21ea",
            "Name": "node2",
            "Address": "127.0.0.1:4747",
            "SerfStatus": "alive",
            "Version": "0.8.0",
            "Leader": false,
            "LastContact": "35.371007ms",
            "LastTerm": 2,
            "LastIndex": 10,
            "Healthy": true,
            "Voter": false,
            "StableSince": "2017-03-28T18:29:10Z"
        }
    ]
}
```

## Stable Server Introduction

When a new server is added to the cluster, there is a waiting period where it
must be healthy and stable for a certain amount of time before being promoted
to a full, voting member. This can be configured via the `ServerStabilizationTime`
setting.

---

~> The following Autopilot features are available only in
   [Nomad Enterprise](https://www.hashicorp.com/products/nomad/) version 0.8.0 and later.

## Server Read and Scheduling Scaling

With the [`non_voting_server`](/docs/configuration/server.html#non_voting_server) option, a
server can be explicitly marked as a non-voter and will never be promoted to a voting
member. This can be useful when more read scaling is needed; being a non-voter means
that the server will still have data replicated to it, but it will not be part of the
quorum that the leader must wait for before committing log entries. Non voting servers can also
act as scheduling workers to increase scheduling throughput in large clusters.

## Redundancy Zones

Prior to Autopilot, it was difficult to deploy servers in a way that took advantage of
isolated failure domains such as AWS Availability Zones; users would be forced to either
have an overly-large quorum (2-3 nodes per AZ) or give up redundancy within an AZ by
deploying just one server in each.

If the `EnableRedundancyZones` setting is set, Nomad will use its value to look for a
zone in each server's specified [`redundancy_zone`](/docs/configuration/server.html#redundancy_zone)
field.

Here's an example showing how to configure this:

```hcl
/* config.hcl */
server {
    redundancy_zone = "west-1"
}
```

```
$ nomad operator autopilot set-config -enable-redundancy-zones=true
Configuration updated!
```

Nomad will then use these values to partition the servers by redundancy zone, and will
aim to keep one voting server per zone. Extra servers in each zone will stay as non-voters
on standby to be promoted if the active voter leaves or dies.

## Upgrade Migrations

Autopilot in Nomad Enterprise supports upgrade migrations by default. To disable this
functionality, set `DisableUpgradeMigration` to true.

When a new server is added and Autopilot detects that its Nomad version is newer than
that of the existing servers, Autopilot will avoid promoting the new server until enough
newer-versioned servers have been added to the cluster. When the count of new servers
equals or exceeds that of the old servers, Autopilot will begin promoting the new servers
to voters and demoting the old servers. After this is finished, the old servers can be
safely removed from the cluster.

To check the Nomad version of the servers, either the [autopilot health](/api/operator.html#read-health)
endpoint or the `nomad members`command can be used:

```
$ nomad server members
Name   Address    Port  Status  Leader  Protocol  Build  Datacenter  Region
node1  127.0.0.1  4648  alive   true    3         0.7.1  dc1         global
node2  127.0.0.1  4748  alive   false   3         0.7.1  dc1         global
node3  127.0.0.1  4848  alive   false   3         0.7.1  dc1         global
node4  127.0.0.1  4948  alive   false   3         0.8.0  dc1         global
```

### Migrations Without a Nomad Version Change

The `EnableCustomUpgrades` field can be used to override the version information used during
a migration, so that the migration logic can be used for updating the cluster when
changing configuration.

If the `EnableCustomUpgrades` setting is set to `true`, Nomad will use its value to look for a
version in each server's specified [`upgrade_version`](/docs/configuration/server.html#upgrade_version)
tag. The upgrade logic will follow semantic versioning and the `upgrade_version`
must be in the form of either `X`, `X.Y`, or `X.Y.Z`.
