---
layout: "http"
page_title: "HTTP API: /v1/agent/members"
sidebar_current: "docs-http-agent-members"
description: |-
  The '/1/agent/members' endpoint is used to query the gossip peers.
---

# /v1/agent/members

The `members` endpoint is used to query the agent for the known peers in
the gossip pool. This is only applicable to servers.

## GET

<dl>
  <dt>Description</dt>
  <dd>
    Lists the known members of the gossip pool.
  </dd>

  <dt>Method</dt>
  <dd>GET</dd>

  <dt>URL</dt>
  <dd>`/v1/agent/members`</dd>

  <dt>Parameters</dt>
  <dd>
    None
  </dd>

  <dt>Returns</dt>
  <dd>

    ```javascript
    {
      "ServerName": "DIPTANUs-MBP",
      "ServerRegion": "global",
      "ServerDC": "dc1",
      "Members": [
        {
          "Name": "DIPTANUs-MBP.global",
          "Addr": "127.0.0.1",
          "Port": 4648,
          "Tags": {
            "mvn": "1",
            "build": "0.5.0rc2",
            "port": "4647",
            "bootstrap": "1",
            "role": "nomad",
            "region": "global",
            "dc": "dc1",
            "vsn": "1"
          },
          "Status": "alive",
          "ProtocolMin": 1,
          "ProtocolMax": 4,
          "ProtocolCur": 2,
          "DelegateMin": 2,
          "DelegateMax": 4,
          "DelegateCur": 4
        }
      ]
    }
    ```

  </dd>
</dl>

