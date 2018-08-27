---
layout: "guides"
page_title: "Upgrade Guides"
sidebar_current: "guides-operations-upgrade-specific"
description: |-
  Specific versions of Nomad may have additional information about the upgrade
  process beyond the standard flow.
---

# Upgrade Guides

The [upgrading page](/guides/operations/upgrade/index.html) covers the details of doing
a standard upgrade. However, specific versions of Nomad may have more
details provided for their upgrades as a result of new features or changed
behavior. This page is used to document those details separately from the
standard upgrade flow.

## Nomad 0.8.0

### Raft Protocol Version Compatibility

When upgrading to Nomad 0.8.0 from a version lower than 0.7.0, users will need
to set the
[`raft_protocol`](/docs/configuration/server.html#raft_protocol) option
in their `server` stanza to 1 in order to maintain backwards compatibility with
the old servers during the upgrade.  After the servers have been migrated to
version 0.8.0, `raft_protocol` can be moved up to 2 and the servers restarted
to match the default.

The Raft protocol must be stepped up in this way; only adjacent version numbers are
compatible (for example, version 1 cannot talk to version 3). Here is a table of the
Raft Protocol versions supported by each Nomad version:

<table class="table table-bordered table-striped">
  <tr>
    <th>Version</th>
    <th>Supported Raft Protocols</th>
  </tr>
  <tr>
    <td>0.6 and earlier</td>
    <td>0</td>
  </tr>
  <tr>
    <td>0.7</td>
    <td>1</td>
  </tr>
  <tr>
    <td>0.8</td>
    <td>1, 2, 3</td>
  </tr>
</table>

In order to enable all [Autopilot](/guides/operations/autopilot.html) features, all servers
in a Nomad cluster must be running with Raft protocol version 3 or later.

#### Upgrading to Raft Protocol 3

This section provides details on upgrading to Raft Protocol 3 in Nomad 0.8 and higher. Raft protocol version 3 requires Nomad running 0.8.0 or newer on all servers in order to work. See [Raft Protocol Version Compatibility](/guides/operations/upgrade/upgrade-specific.html#raft-protocol-version-compatibility) for more details. Also the format of `peers.json` used for outage recovery is different when running with the latest Raft protocol. See [Manual Recovery Using peers.json](/guides/operations/outage.html#manual-recovery-using-peers-json) for a description of the required format.

Please note that the Raft protocol is different from Nomad's internal protocol as shown in commands like `nomad server members`. To see the version of the Raft protocol in use on each server, use the `nomad operator raft list-peers` command.

The easiest way to upgrade servers is to have each server leave the cluster, upgrade its `raft_protocol` version in the `server` stanza, and then add it back. Make sure the new server joins successfully and that the cluster is stable before rolling the upgrade forward to the next server. It's also possible to stand up a new set of servers, and then slowly stand down each of the older servers in a similar fashion.

When using Raft protocol version 3, servers are identified by their `node-id` instead of their IP address when Nomad makes changes to its internal Raft quorum configuration. This means that once a cluster has been upgraded with servers all running Raft protocol version 3, it will no longer allow servers running any older Raft protocol versions to be added. If running a single Nomad server, restarting it in-place will result in that server not being able to elect itself as a leader. To avoid this, either set the Raft protocol back to 2, or use [Manual Recovery Using peers.json](/guides/operations/outage.html#manual-recovery-using-peers-json) to map the server to its node ID in the Raft quorum configuration.


### Node Draining Improvements

Node draining via the [`node drain`][drain-cli] command or the [drain
API][drain-api] has been substantially changed in Nomad 0.8. In Nomad 0.7.1 and
earlier draining a node would immediately stop all allocations on the node
being drained. Nomad 0.8 now supports a [`migrate`][migrate] stanza in job
specifications to control how many allocations may be migrated at once and the
default will be used for existing jobs.

The `drain` command now blocks until the drain completes. To get the Nomad
0.7.1 and earlier drain behavior use the command: `nomad node drain -enable
-force -detach <node-id>`

See the [`migrate` stanza documentation][migrate] and [Decommissioning Nodes
guide](/guides/operations/node-draining.html) for details.

### Periods in Environment Variable Names No Longer Escaped

*Applications which expect periods in environment variable names to be replaced
with underscores must be updated.*

In Nomad 0.7 periods (`.`) in environment variables names were replaced with an
underscore in both the [`env`](/docs/job-specification/env.html) and
[`template`](/docs/job-specification/template.html) stanzas.

In Nomad 0.8 periods are *not* replaced and will be included in environment
variables verbatim.

For example the following stanza:

```text
env {
  registry.consul.addr = "${NOMAD_IP_http}:8500"
}
```

In Nomad 0.7 would be exposed to the task as
`registry_consul_addr=127.0.0.1:8500`. In Nomad 0.8 it will now appear exactly
as specified: `registry.consul.addr=127.0.0.1:8500`.

### Client APIs Unavailable on Older Nodes

Because Nomad 0.8 uses a new RPC mechanism to route node-specific APIs like
[`nomad alloc fs`](/docs/commands/alloc/fs.html) through servers to the node,
0.8 CLIs are incompatible using these commands on clients older than 0.8.

To access these commands on older clients either continue to use a pre-0.8
version of the CLI, or upgrade all clients to 0.8.

### CLI Command Changes

Nomad 0.8 has changed the organization of CLI commands to be based on
subcommands. An example of this change is the change from `nomad alloc-status`
to `nomad alloc status`. All commands have been made to be backwards compatible,
but operators should update any usage of the old style commands to the new style
as the old style will be deprecated in future versions of Nomad.

### RPC Advertise Address

The behavior of the [advertised RPC
address](/docs/configuration/index.html#rpc-1) has changed to be only used
to advertise the RPC address of servers to client nodes. Server to server
communication is done using the advertised Serf address. Existing cluster's
should not be effected but the advertised RPC address may need to be updated to
allow connecting client's over a NAT.


## Nomad 0.6.0

### Default `advertise` address changes

When no `advertise` address was specified and Nomad's `bind_addr` was loopback
or `0.0.0.0`, Nomad attempted to resolve the local hostname to use as an
advertise address.

Many hosts cannot properly resolve their hostname, so Nomad 0.6 defaults
`advertise` to the first private IP on the host (e.g. `10.1.2.3`).

If you manually configure `advertise` addresses no changes are necessary.

## Nomad Clients

The change to the default, advertised IP also effect clients that do not specify
which network_interface to use. If you have several routable IPs, it is advised
to configure the client's [network
interface](/docs/configuration/client.html#network_interface)
such that tasks bind to the correct address.

## Nomad 0.5.5

### Docker `load` changes

Nomad 0.5.5 has a backward incompatible change in the `docker` driver's
configuration. Prior to 0.5.5 the `load` configuration option accepted a list
images to load, in 0.5.5 it has been changed to a single string. No
functionality was changed. Even if more than one item was specified prior to
0.5.5 only the first item was used.

To do a zero-downtime deploy with jobs that use the `load` option:

* Upgrade servers to version 0.5.5 or later.

* Deploy new client nodes on the same version as the servers.

* Resubmit jobs with the `load` option fixed and a constraint to only run on
  version 0.5.5 or later:

```hcl
    constraint {
      attribute = "${attr.nomad.version}"
      operator  = "version"
      value     = ">= 0.5.5"
    }
```

* Drain and shutdown old client nodes.

### Validation changes

Due to internal job serialization and validation changes you may run into
issues using 0.5.5 command line tools such as `nomad run` and `nomad validate`
with 0.5.4 or earlier agents.

It is recommended you upgrade agents before or alongside your command line
tools.

## Nomad 0.4.0

Nomad 0.4.0 has backward incompatible changes in the logic for Consul
deregistration.  When a Task which was started by Nomad v0.3.x is uncleanly shut
down, the Nomad 0.4 Client will no longer clean up any stale services.  If an
in-place upgrade of the Nomad client to 0.4 prevents the Task from gracefully
shutting down and deregistering its Consul-registered services, the Nomad Client
will not clean up the remaining Consul services registered with the 0.3
Executor.

We recommend draining a node before upgrading to 0.4.0 and then re-enabling the
node once the upgrade is complete.


## Nomad 0.3.1

Nomad 0.3.1 removes artifact downloading from driver configurations and places them as
a first class element of the task. As such, jobs will have to be rewritten in
the proper format and resubmitted to Nomad. Nomad clients will properly
re-attach to existing tasks but job definitions must be updated before they can
be dispatched to clients running 0.3.1.

## Nomad 0.3.0

Nomad 0.3.0 has made several substantial changes to job files included a new
`log` block and variable interpretation syntax (`${var}`), a modified `restart`
policy syntax, and minimum resources for tasks as well as validation. These
changes require a slight change to the default upgrade flow.

After upgrading the version of the servers, all previously submitted jobs must
be resubmitted with the updated job syntax using a Nomad 0.3.0 binary.

* All instances of `$var` must be converted to the new syntax of `${var}`

* All tasks must provide their required resources for CPU, memory and disk as
  well as required network usage if ports are required by the task.

* Restart policies must be updated to indicate whether it is desired for the
  task to restart on failure or to fail using `mode = "delay"` or `mode =
  "fail"` respectively.

* Service names that include periods will fail validation. To fix, remove any
  periods from the service name before running the job.

After updating the Servers and job files, Nomad Clients can be upgraded by first
draining the node so no tasks are running on it. This can be verified by running
`nomad node status <node-id>` and verify there are no tasks in the `running`
state. Once that is done the client can be killed, the `data_dir` should be
deleted and then Nomad 0.3.0 can be launched.

[drain-api]: /api/nodes.html#drain-node
[drain-cli]: /docs/commands/node/drain.html
[migrate]: /docs/job-specification/migrate.html
