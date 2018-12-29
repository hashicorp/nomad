---
layout: "docs"
page_title: "server_join Stanza - Agent Configuration"
sidebar_current: "docs-agent-configuration--server-join"
description: |-
  The "server_join" stanza specifies how the Nomad agent will discover and connect to Nomad servers.
---

# `server_join` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>server -> **server_join**</code>
      <br>
      <code>client -> **server_join**</code>
    </td>
  </tr>
</table>

The `server_join` stanza specifies how the Nomad agent will discover and connect
to Nomad servers.

```hcl
server_join {
  retry_join = [ "1.1.1.1", "2.2.2.2" ]
  retry_max = 3
  retry_interval = "15s"
}
```

## `server_join` Parameters

-   `retry_join` `(array<string>: [])` - Specifies a list of server addresses to
  join. This is similar to [`start_join`](#start_join), but will continue to
  be attempted even if the initial join attempt fails, up to
  [retry_max](#retry_max). Further, `retry_join` is available to
  both Nomad servers and clients, while `start_join` is only defined for Nomad
  servers.  This is useful for cases where we know the address will become
  available eventually.  Use `retry_join` with an array as a replacement for
  `start_join`, **do not use both options**.

    Address format includes both using IP addresses as well as an interface to the
  [go-discover](https://github.com/hashicorp/go-discover) library for doing
  automated cluster joining using cloud metadata. See [Cloud
  Auto-join][cloud_auto_join] for more information.

    ```
  server_join {
    retry_join = [ "1.1.1.1", "2.2.2.2" ]
  }
  ```

    Using the `go-discover` interface, this can be defined both in a client or
  server configuration as well as provided as a command-line argument.

    ```
  server_join {
    retry_join = [ "provider=aws tag_key=..." ]
  }
  ```

    See the [server address format](#server-address-format) for more information
  about expected server address formats.

- `retry_interval` `(string: "30s")` - Specifies the time to wait between retry
  join attempts.

- `retry_max` `(int: 0)` - Specifies the maximum number of join attempts to be
  made before exiting with a return code of 1. By default, this is set to 0
  which is interpreted as infinite retries.

- `start_join` `(array<string>: [])` - Specifies a list of server addresses to
  join on startup. If Nomad is unable to join with any of the specified
  addresses, agent startup will fail. See the
  [server address format](#server-address-format) section for more information
  on the format of the string. This field is defined only for Nomad servers and
  will result in a configuration parse error if included in a client
  configuration.

## Server Address Format

This section describes the acceptable syntax and format for describing the
location of a Nomad server. There are many ways to reference a Nomad server,
including directly by IP address and resolving through DNS.

### Directly via IP Address

It is possible to address another Nomad server using its IP address. This is
done in the `ip:port` format, such as:

```
1.2.3.4:5678
```

If the port option is omitted, it defaults to the Serf port, which is 4648
unless configured otherwise:

```
1.2.3.4 => 1.2.3.4:4648
```

### Via Domains or DNS

It is possible to address another Nomad server using its DNS address. This is
done in the `address:port` format, such as:

```
nomad-01.company.local:5678
```

If the port option is omitted, it defaults to the Serf port, which is 4648
unless configured otherwise:

```
nomad-01.company.local => nomad-01.company.local:4648
```

### Via the go-discover interface

As of Nomad 0.8.4, `retry_join` accepts a unified interface using the
[go-discover](https://github.com/hashicorp/go-discover) library for doing
automated cluster joining using cloud metadata. See [Cloud
Auto-join][cloud_auto_join] for more information.

```
"provider=aws tag_key=..." => 1.2.3.4:4648
```

[cloud_auto_join]: /docs/agent/cloud_auto_join.html "Nomad Cloud Auto-join"
