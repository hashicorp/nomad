---
layout: "guides"
page_title: "Outage Recovery"
sidebar_current: "guides-operations-outage-recovery"
description: |-
  Don't panic! This is a critical first step. Depending on your deployment
  configuration, it may take only a single server failure for cluster
  unavailability. Recovery requires an operator to intervene, but recovery is
  straightforward.
---

# Outage Recovery

Don't panic! This is a critical first step.

Depending on your
[deployment configuration](/docs/internals/consensus.html#deployment_table), it
may take only a single server failure for cluster unavailability. Recovery
requires an operator to intervene, but the process is straightforward.

~> This guide is for recovery from a Nomad outage due to a majority of server
nodes in a datacenter being lost. If you are looking to add or remove servers,
see the [bootstrapping guide](/guides/operations/cluster/bootstrapping.html).

## Failure of a Single Server Cluster

If you had only a single server and it has failed, simply restart it. A
single server configuration requires the
[`-bootstrap-expect=1`](/docs/configuration/server.html#bootstrap_expect)
flag. If the server cannot be recovered, you need to bring up a new
server. See the [bootstrapping guide](/guides/operations/cluster/bootstrapping.html)
for more detail.

In the case of an unrecoverable server failure in a single server cluster, data
loss is inevitable since data was not replicated to any other servers. This is
why a single server deploy is **never** recommended.

## Failure of a Server in a Multi-Server Cluster

If you think the failed server is recoverable, the easiest option is to bring
it back online and have it rejoin the cluster with the same IP address, returning
the cluster to a fully healthy state. Similarly, even if you need to rebuild a
new Nomad server to replace the failed node, you may wish to do that immediately.
Keep in mind that the rebuilt server needs to have the same IP address as the failed
server. Again, once this server is online and has rejoined, the cluster will return
to a fully healthy state.

Both of these strategies involve a potentially lengthy time to reboot or rebuild
a failed server. If this is impractical or if building a new server with the same
IP isn't an option, you need to remove the failed server. Usually, you can issue
a [`nomad server force-leave`](/docs/commands/server/force-leave.html) command
to remove the failed server if it's still a member of the cluster.

If [`nomad server force-leave`](/docs/commands/server/force-leave.html) isn't
able to remove the server, you have two methods available to remove it,
depending on your version of Nomad:

* In Nomad 0.5.5 and later, you can use the [`nomad operator raft
  remove-peer`](/docs/commands/operator/raft-remove-peer.html) command to remove
  the stale peer server on the fly with no downtime.

* In versions of Nomad prior to 0.5.5, you can manually remove the stale peer
  server using the `raft/peers.json` recovery file on all remaining servers. See
  the [section below](#manual-recovery-using-peers-json) for details on this
  procedure. This process requires Nomad downtime to complete.

In Nomad 0.5.5 and later, you can use the [`nomad operator raft
list-peers`](/docs/commands/operator/raft-list-peers.html) command to inspect
the Raft configuration:

```
$ nomad operator raft list-peers
Node                   ID               Address          State     Voter
nomad-server01.global  10.10.11.5:4647  10.10.11.5:4647  follower  true
nomad-server02.global  10.10.11.6:4647  10.10.11.6:4647  leader    true
nomad-server03.global  10.10.11.7:4647  10.10.11.7:4647  follower  true
```

## Failure of Multiple Servers in a Multi-Server Cluster

In the event that multiple servers are lost, causing a loss of quorum and a
complete outage, partial recovery is possible using data on the remaining
servers in the cluster. There may be data loss in this situation because multiple
servers were lost, so information about what's committed could be incomplete.
The recovery process implicitly commits all outstanding Raft log entries, so
it's also possible to commit data that was uncommitted before the failure.

See the [section below](#manual-recovery-using-peers-json) for details of the
recovery procedure. You simply include just the remaining servers in the
`raft/peers.json` recovery file.  The cluster should be able to elect a leader
once the remaining servers are all restarted with an identical `raft/peers.json`
configuration.

Any new servers you introduce later can be fresh with totally clean data directories
and joined using Nomad's `server join` command.

In extreme cases, it should be possible to recover with just a single remaining
server by starting that single server with itself as the only peer in the
`raft/peers.json` recovery file.

Prior to Nomad 0.5.5 it wasn't always possible to recover from certain
types of outages with `raft/peers.json` because this was ingested before any Raft
log entries were played back. In Nomad 0.5.5 and later, the `raft/peers.json`
recovery file is final, and a snapshot is taken after it is ingested, so you are
guaranteed to start with your recovered configuration. This does implicitly commit
all Raft log entries, so should only be used to recover from an outage, but it
should allow recovery from any situation where there's some cluster data available.

## Manual Recovery Using peers.json

To begin, stop all remaining servers. You can attempt a graceful leave,
but it will not work in most cases. Do not worry if the leave exits with an
error. The cluster is in an unhealthy state, so this is expected.

In Nomad 0.5.5 and later, the `peers.json` file is no longer present
by default and is only used when performing recovery. This file will be deleted
after Nomad starts and ingests this file. Nomad 0.5.5 also uses a new, automatically-
created `raft/peers.info` file to avoid ingesting the `raft/peers.json` file on the
first start after upgrading. Be sure to leave `raft/peers.info` in place for proper
operation.

Using `raft/peers.json` for recovery can cause uncommitted Raft log entries to be
implicitly committed, so this should only be used after an outage where no
other option is available to recover a lost server. Make sure you don't have
any automated processes that will put the peers file in place on a
periodic basis.

The next step is to go to the
[`-data-dir`](/docs/configuration/index.html#data_dir) of each Nomad
server. Inside that directory, there will be a `raft/` sub-directory. We need to
create a `raft/peers.json` file. It should look something like:

```javascript
[
  "10.0.1.8:4647",
  "10.0.1.6:4647",
  "10.0.1.7:4647"
]
```

Simply create entries for all remaining servers. You must confirm
that servers you do not include here have indeed failed and will not later
rejoin the cluster. Ensure that this file is the same across all remaining
server nodes.

At this point, you can restart all the remaining servers. In Nomad 0.5.5 and
later you will see them ingest recovery file:

```text
...
2016/08/16 14:39:20 [INFO] nomad: found peers.json file, recovering Raft configuration...
2016/08/16 14:39:20 [INFO] nomad.fsm: snapshot created in 12.484µs
2016/08/16 14:39:20 [INFO] snapshot: Creating new snapshot at /tmp/peers/raft/snapshots/2-5-1471383560779.tmp
2016/08/16 14:39:20 [INFO] nomad: deleted peers.json file after successful recovery
2016/08/16 14:39:20 [INFO] raft: Restored from snapshot 2-5-1471383560779
2016/08/16 14:39:20 [INFO] raft: Initial configuration (index=1): [{Suffrage:Voter ID:10.212.15.121:4647 Address:10.212.15.121:4647}]
...
```

If any servers managed to perform a graceful leave, you may need to have them
rejoin the cluster using the [`server join`](/docs/commands/server/join.html) command:

```text
$ nomad server join <Node Address>
Successfully joined cluster by contacting 1 nodes.
```

It should be noted that any existing member can be used to rejoin the cluster
as the gossip protocol will take care of discovering the server nodes.

At this point, the cluster should be in an operable state again. One of the
nodes should claim leadership and emit a log like:

```text
[INFO] nomad: cluster leadership acquired
```

In Nomad 0.5.5 and later, you can use the [`nomad operator raft
list-peers`](/docs/commands/operator/raft-list-peers.html) command to inspect
the Raft configuration:

```
$ nomad operator raft list-peers
Node                   ID               Address          State     Voter
nomad-server01.global  10.10.11.5:4647  10.10.11.5:4647  follower  true
nomad-server02.global  10.10.11.6:4647  10.10.11.6:4647  leader    true
nomad-server03.global  10.10.11.7:4647  10.10.11.7:4647  follower  true
```

## Peers.json Format Changes in Raft Protocol 3
For Raft protocol version 3 and later, peers.json should be formatted as a JSON
array containing the node ID, address:port, and suffrage information of each
Nomad server in the cluster, like this:

```
[
  {
    "id": "adf4238a-882b-9ddc-4a9d-5b6758e4159e",
    "address": "10.1.0.1:8300",
    "non_voter": false
  },
  {
    "id": "8b6dda82-3103-11e7-93ae-92361f002671",
    "address": "10.1.0.2:8300",
    "non_voter": false
  },
  {
    "id": "97e17742-3103-11e7-93ae-92361f002671",
    "address": "10.1.0.3:8300",
    "non_voter": false
  }
]
```

- `id` `(string: <required>)` - Specifies the `node ID`
  of the server. This can be found in the logs when the server starts up,
  and it can also be found inside the `node-id` file in the server's data directory.

- `address` `(string: <required>)` - Specifies the IP and port of the server. The port is the
  server's RPC port used for cluster communications.

- `non_voter` `(bool: <false>)` - This controls whether the server is a non-voter, which is used
  in some advanced [Autopilot](/guides/operations/autopilot.html) configurations. If omitted, it will
  default to false, which is typical for most clusters.
