---
layout: "docs"
page_title: "tls Stanza - Agent Configuration"
sidebar_current: "docs-agent-configuration-tls"
description: |-
  The "tls" stanza configures Nomad's TLS communication via HTTP and RPC to
  enforce secure cluster communication between servers, clients, and between.
---

# `tls` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>**tls**</code>
    </td>
  </tr>
</table>

The `tls` stanza configures Nomad's TLS communication via HTTP and RPC to
enforce secure cluster communication between servers, clients, and between.

```hcl
tls {
  http = true
  rpc  = true
}
```

~> Incorrect configuration of the TLS configuration can result in failure to
start the Nomad agent.

This section of the documentation only covers the configuration options for
`tls` stanza. To understand how to setup the certificates themselves, please see
the [Agent's Gossip and RPC Encryption](/docs/agent/encryption.html).

## `tls` Parameters

- `ca_file` `(string: "")` - Specifies the path to the CA certificate to use for
  Nomad's TLS communication.

- `cert_file` `(string: "")` - Specifies the path to the certificate file used
  for Nomad's TLS communication.

- `key_file` `(string: "")` - Specifies the path to the key file to use for
  Nomad's TLS communication.

- `http` `(bool: false)` - Specifies if TLS should be enabled on the HTTP
  endpoints on the Nomad agent, including the API.

- `rpc` `(bool: false)` - Specifies if TLS should be enabled on the RPC
  endpoints and [Raft][raft] traffic between the Nomad servers. Enabling this on
  a Nomad client makes the client use TLS for making RPC requests to the Nomad
  servers.

- `verify_https_client` `(bool: false)` - Specifies agents should require
  client certificates for all incoming HTTPS requests. The client certificates
  must be signed by the same CA as Nomad.

- `verify_server_hostname` `(bool: false)` - Specifies if outgoing TLS
  connections should verify the server's hostname.

## `tls` Examples

The following examples only show the `tls` stanzas. Remember that the
`tls` stanza is only valid in the placements listed above.

### Enabling TLS

This example shows enabling TLS configuration. This enables TLS communication
between all servers and clients using the default system CA bundle and
certificates.

```hcl
tls {
  http = true
  rpc  = true

  ca_file   = "/etc/certs/ca.crt"
  cert_file = "/etc/certs/nomad.crt"
  key_file  = "/etc/certs/nomad.key"
}
```

[raft]: https://github.com/hashicorp/serf "Serf by HashiCorp"
