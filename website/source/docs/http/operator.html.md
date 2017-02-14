---
layout: "http"
page_title: "HTTP API: /v1/operator/"
sidebar_current: "docs-http-operator"
description: >
  The '/v1/operator/' endpoints provides cluster-level tools for Nomad
  operators.
---

# /v1/operator

The Operator endpoint provides cluster-level tools for Nomad operators, such
as interacting with the Raft subsystem. This was added in Nomad 0.5.5

~> Use this interface with extreme caution, as improper use could lead to a
  Nomad outage and even loss of data.

See the [Outage Recovery](/guides/outage.html) guide for some examples of how
these capabilities are used. For a CLI to perform these operations manually, please
see the documentation for the [`nomad operator`](/docs/commands/operator-index.html)
command.

By default, the agent's local region is used; another region can be specified
using the `?region=` query parameter.

## GET

<dl>
  <dt>Description</dt>
  <dd>
    Query the status of a client node registered with Nomad.
  </dd>

  <dt>Method</dt>
  <dd>GET</dd>

  <dt>URL</dt>
  <dd>`/v1/operator/raft/configuration`</dd>

  <dt>Parameters</dt>
  <dd>
    <ul>
      <li>
        <span class="param">stale</span>
        <span class="param-flags">optional</span>
        If the cluster doesn't currently have a leader an error will be
        returned. You can use the `?stale` query parameter to read the Raft
        configuration from any of the Nomad servers.
      </li>
    </ul>
  </dd>

  <dt>Returns</dt>
  <dd>

    ```javascript
{
  "Servers": [
    {
      "ID": "127.0.0.1:4647",
      "Node": "alice",
      "Address": "127.0.0.1:4647",
      "Leader": true,
      "Voter": true
    },
    {
      "ID": "127.0.0.2:4647",
      "Node": "bob",
      "Address": "127.0.0.2:4647",
      "Leader": false,
      "Voter": true
    },
    {
      "ID": "127.0.0.3:4647",
      "Node": "carol",
      "Address": "127.0.0.3:4647",
      "Leader": false,
      "Voter": true
    }
  ],
  "Index": 22
}
    ```

  </dd>

  <dt>Field Reference</dt>
  <dd>

    <ul>
      <li>
        <span class="param">Servers</span>
        The returned `Servers` array has information about the servers in the Raft
        peer configuration. See the `Server` block for a description of its fields:
      </li>
      <li>
        <span class="param">Index</span>
        The `Index` value is the Raft corresponding to this configuration. The
        latest configuration may not yet be committed if changes are in flight.
      </li>
    </ul>

    `Server` Fields: 
    <ul>
      <li>
        <span class="param">ID</span>
        `ID` is the ID of the server. This is the same as the `Address` but may
        be upgraded to a GUID in a future version of Nomad.
      </li>
      <li>
        <span class="param">Node</span>
        `Node` is the node name of the server, as known to Nomad, or "(unknown)" if
        the node is stale and not known.
      </li>
      <li>
        <span class="param">Address</span>
        `Address` is the IP:port for the server.
      </li>
      <li>
        <span class="param">Leader</span>
        `Leader` is either "true" or "false" depending on the server's role in the
        Raft configuration.
      </li>
      <li>
        <span class="param">Voter</span>
        `Voter` is "true" or "false", indicating if the server has a vote in the Raft
        configuration. Future versions of Nomad may add support for non-voting servers.
      </li>
    </ul>

  </dd>
</dl>


## DELETE

<dl>
  <dt>Description</dt>
  <dd>
    Remove the Nomad server with given address from the Raft configuration. The
    return code signifies success or failure.
  </dd>

  <dt>Method</dt>
  <dd>DELETE</dd>

  <dt>URL</dt>
  <dd>`/v1/operator/raft/peer`</dd>

  <dt>Parameters</dt>
  <dd>
    <ul>
      <li>
        <span class="param">address</span>
        <span class="param-flags">required</span>
        The address specifies the server to remove and is given as an `IP:port`.
        The port number is usually 4647, unless configured otherwise. Nothing is
        required in the body of the request.
      </li>
    </ul>
  </dd>

  <dt>Returns</dt>
  <dd>None</dd>

</dl>
