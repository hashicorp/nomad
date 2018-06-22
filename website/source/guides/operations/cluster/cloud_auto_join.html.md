---
layout: "guides"
page_title: "Cloud Auto-join"
sidebar_current: "guides-operations-cluster-cloud-auto-join"
description: |-
  Nomad supports automatic cluster joining using cloud metadata from various 
  cloud providers
---

# Cloud Auto-joining

As of Nomad 0.8.4,
[`retry_join`](/docs/configuration/server_join.html#retry_join) accepts a
unified interface using the
[go-discover](https://github.com/hashicorp/go-discover) library for doing
automatic cluster joining using cloud metadata. To use retry-join with a
supported cloud provider, specify the configuration on the command line or
configuration file as a `key=value key=value ...` string. Values are taken 
literally and must not be URL encoded. If the values contain spaces, backslashes 
or double quotes thenthey need to be double quoted and the usual escaping rules 
apply.

```json
{
  "retry_join": ["provider=my-cloud config=val config2=\"some other val\" ..."]
}
```

The cloud provider-specific configurations are documented [here](/docs/configuration/server_join.html#cloud-auto-join). 
This can be combined with static IP or DNS addresses or even multiple configurations
for different providers. In order to use discovery behind a proxy, you will need to set
`HTTP_PROXY`, `HTTPS_PROXY` and `NO_PROXY` environment variables per
[Golang `net/http` library](https://golang.org/pkg/net/http/#ProxyFromEnvironment).




