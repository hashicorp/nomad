---
layout: "docs"
page_title: "autopilot Stanza - Agent Configuration"
sidebar_current: "docs-configuration-autopilot"
description: |-
  The "autopilot" stanza configures the Nomad agent to configure Autopilot behavior.
---

# `autopilot` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>**autopilot**</code>
    </td>
  </tr>
</table>

The `autopilot` stanza configures the Nomad agent to configure Autopilot behavior.
For more information about Autopilot, see the [Autopilot Guide](/guides/operations/autopilot.html).

```hcl
autopilot {
    cleanup_dead_servers = true
    last_contact_threshold = "200ms"
    max_trailing_logs = 250
    server_stabilization_time = "10s"
    enable_redundancy_zones = false
    disable_upgrade_migration = false
    enable_custom_upgrades = false
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

- `enable_redundancy_zones` `(bool: false)` - (Enterprise-only) Controls whether
  Autopilot separates servers into zones for redundancy, in conjunction with the
  [redundancy_zone](/docs/configuration/server.html#redundancy_zone) parameter.
  Only one server in each zone can be a voting member at one time.

- `disable_upgrade_migration` `(bool: false)` - (Enterprise-only) Disables Autopilot's
  upgrade migration strategy in Nomad Enterprise of waiting until enough
  newer-versioned servers have been added to the cluster before promoting any of
  them to voters.

- `enable_custom_upgrades` `(bool: false)` - (Enterprise-only) Specifies whether to 
  enable using custom upgrade versions when performing migrations, in conjunction with
  the [upgrade_version](/docs/configuration/server.html#upgrade_version) parameter.

