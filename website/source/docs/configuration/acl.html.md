---
layout: "docs"
page_title: "acl Stanza - Agent Configuration"
sidebar_current: "docs-configuration-acl"
description: |-
  The "acl" stanza configures the Nomad agent to enable ACLs and tune various parameters.
---

# `acl` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>**acl**</code>
    </td>
  </tr>
</table>

The `acl` stanza configures the Nomad agent to enable ACLs and tunes various ACL parameters.

```hcl
acl {
  enabled = true
  token_ttl = "30s"
  policy_ttl = "60s"
}
```

## `acl` Parameters

- `enabled` `(bool: false)` - Specifies if ACL enforcement is enabled. All other
  client configuration options depend on this value.

- `token_ttl` `(string: "30s")` - Specifies the maximum time-to-live (TTL) for
  cached ACL tokens. This does not affect servers, since they do not cache tokens.
  Setting this value lower reduces how stale a token can be, but increases
  the request load against servers. If a client cannot reach a server, for example
  because of an outage, the TTL will be ignored and the cached value used.

- `policy_ttl` `(string: "30s")` - Specifies the maximum time-to-live (TTL) for
  cached ACL policies. This does not affect servers, since they do not cache policies.
  Setting this value lower reduces how stale a policy can be, but increases
  the request load against servers. If a client cannot reach a server, for example
  because of an outage, the TTL will be ignored and the cached value used.

- `replication_token` `(string: "")` - Specifies the Secret ID of the ACL token
  to use for replicating policies and tokens. This is used by servers in non-authoritative
  region to mirror the policies and tokens into the local region.

