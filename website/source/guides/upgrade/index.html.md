---
layout: "guides"
page_title: "Upgrading"
sidebar_current: "guides-upgrade"
description: |-
  Learn how to upgrade Nomad.
---

# Upgrading

This page documents how to upgrade Nomad when a new version is released.

~> **Upgrade Warning!** Both Nomad Clients and Servers are meant to be
long-running processes that maintain communication with each other. Nomad
Servers maintain quorum with other Servers and Clients are in constant
communication with Servers. As such, care should be taken to properly
upgrade Nomad to ensure minimal service disruption. Unsafe upgrades can
cause a service outage.

~> **Downgrade Warning!** We currently do not support safely downgrading
Nomad servers. This is due to the fact that the data directory
contains transaction logs which may not safely apply after a downgrade. To downgrade
a Nomad server, its data directory must be wiped out. In general, Client downgrades are
safe to do, however we recommend checking the [upgrade specific](/guides/upgrade/upgrade-specific.html)
page which will highlight versions where a client downgrade is not supported.

## Upgrade Process

For upgrades we strive to ensure backwards compatibility. For most upgrades, the
process is as simple as upgrading the binary and restarting the service.

Prior to starting the upgrade please check the
[specific version details](/guides/upgrade/upgrade-specific.html) page as some
version differences may require specific steps.

At a high level we complete the following steps to upgrade Nomad:

* **Add the new version**
* **Check cluster health**
* **Remove the old version**
* **Check cluster health**
* **Upgrade clients**

### 1. Add the new version to the existing cluster

Whether you are replacing the software in place on existing systems or bringing
up new hosts you should make changes incrementally, verifying cluster health at
each step of the upgrade  

On a single server, install the new version of Nomad.  You can do this by
joining a new server to the cluster or by replacing or upgrading the binary
locally and restarting the service.

### 2. Check cluster health

Monitor the Nomad logs on the remaining nodes to check the new node has entered
the cluster correctly.

Run `nomad agent-info` on the new server and check that the `last_log_index` is
of a similar value to the other nodes.  This step ensures that changes have been
replicated to the new node.

```
ubuntu@nomad-server-10-1-1-4:~$ nomad agent-info
nomad
  bootstrap = false
  known_regions = 1
  leader = false
  server = true
raft
  applied_index = 53460
  commit_index = 53460
  fsm_pending = 0
  last_contact = 54.512216ms
  last_log_index = 53460
  last_log_term = 1
  last_snapshot_index = 49511
  last_snapshot_term = 1
  num_peers = 2
...
```

Continue with the upgrades across the Server fleet making sure to do a single Nomad
server at a time.  You can check state of the servers and clients with the
`nomad server members` and `nomad node status` commands which indicate state of the
nodes.

### 3. Remove the old versions from servers

If you are doing an in place upgrade on existing servers this step is not
necessary as the version was changed in place.

If you are doing an upgrade by adding new servers and removing old servers
from the fleet you need to ensure that the server has left the fleet safely.

1. Stop the service on the existing host
2. On another server issue a `nomad server members` and check the status, if
the server is now in a left state you are safe to continue.
3. If the server is not in a left state, issue a `nomad server force-leave <server id>`
to remove the server from the cluster.

Monitor the logs of the other hosts in the Nomad cluster over this period.

### 4. Check cluster health

Use the same actions in step #2 above to confirm cluster health.

### 5. Upgrade clients

Following the successful upgrade of the servers you can now update your
clients using a similar process as the servers.  You may either upgrade clients
in-place or start new nodes on the new version. See the [Workload Migration
Guide](/guides/operations/node-draining.html) for instructions on how to migrate running
allocations from the old nodes to the new nodes with the [`nomad node
drain`](/docs/commands/node/drain.html) command.

## Done!

You are now running the latest Nomad version. You can verify all
Clients joined by running `nomad node status` and checking all the clients
are in a `ready` state.

## Upgrading to Nomad Enterprise

The process of upgrading to a Nomad Enterprise version is identical to upgrading
between versions of open source Nomad. The same guidance above should be
followed and as always, prior to starting the upgrade please check the [specific
version details](/guides/upgrade/upgrade-specific.html) page as some version
differences may require specific steps.
