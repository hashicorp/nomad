---
layout: "guides"
page_title: "LXC"
sidebar_current: "guides-external"
description: |-
  Guide for using LXC external task driver plugin.
---

## LXC

The `lxc` driver provides an interface for using LXC for running application containers. You can download the external LXC driver [here][lxc-driver].

## Reference Material

- The [LXC][lxc-docs] driver documentation

## Estimated Time to Complete

20 minutes

## Challenge


## Solution


## Prerequisites

To perform the tasks described in this guide, you need to have a Nomad
environment with Consul installed. You can use this
[repo](https://github.com/hashicorp/nomad/tree/master/terraform#provision-a-nomad-cluster-in-the-cloud) to easily provision a sandbox environment. This guide will assume a cluster with one server node and one client node.

-> **Please Note:** This guide is for demo purposes and is only using a single server
node. In a production cluster, 3 or 5 server nodes are recommended.

## Steps

### Step 1: Download and Install the LXC Driver 


### Step 2: Verify Client Configuration




## Next Steps


[lxc-driver]: /coming/soon
[lxc-docs]: /docs/drivers/external/lxc.html