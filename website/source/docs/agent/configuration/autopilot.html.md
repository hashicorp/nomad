---
layout: "docs"
page_title: "autopilot Stanza - Agent Configuration"
sidebar_current: "docs-agent-configuration-autopilot"
description: |-
  The "autopilot" stanza configures the Nomad agent to configure Autopilot behavior.
---

# `autopilot` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>**acl**</code>
    </td>
  </tr>
</table>

The `autopilot` stanza configures the Nomad agent to configure Autopilot behavior.

```hcl
autopilot {
    cleanup_dead_servers = true
    last_contact_threshold = "200ms"
    max_trailing_logs = 250
    server_stabilization_time = "10s"
    redundancy_zone_tag = ""
    disable_upgrade_migration = true
    upgrade_version_tag = ""
}
```

## `autopilot` Parameters

- `cleanup_dead_servers` `(bool: true)` - Specifies automatic removal of dead
  server nodes periodically and whenever a new server is added to the cluster.

- `last_contact_threshold` `(string: "200ms")` - Specifies the maximum amount of
  time a server can go without contact from the leader before being considered
  unhealthy. Must be a duration value such as `10s`.

- `max_trailing_logs` `(int: 250)` specifies the maximum number of log entries
  that a server can trail the leader by before being considered unhealthy.

- `server_stabilization_time` `(string: "10s")` - Specifies the minimum amount of
  time a server must be stable in the 'healthy' state before being added to the
  cluster. Only takes effect if all servers are running Raft protocol version 3
  or higher. Must be a duration value such as `30s`.

- `redundancy_zone_tag` `(string: "")` - Controls the node-meta key to use when
  Autopilot is separating servers into zones for redundancy. Only one server in
  each zone can be a voting member at one time. If left blank, this feature will
  be disabled.

- `disable_upgrade_migration` `(bool: false)` - Disables Autopilot's upgrade
  migration strategy in Nomad Enterprise of waiting until enough
  newer-versioned servers have been added to the cluster before promoting any of
  them to voters.

- `upgrade_version_tag` `(string: "")` - Controls the node-meta key to use for
  version info when performing upgrade migrations. If left blank, the Nomad
  version will be used.

