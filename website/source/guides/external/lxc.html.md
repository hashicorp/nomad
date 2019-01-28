---
layout: "guides"
page_title: "LXC"
sidebar_current: "guides-external"
description: |-
  Guide for using LXC external task driver plugin.
---

## LXC

The `lxc` driver provides an interface for using LXC for running application containers. You can download the external LXC driver [here][lxc-driver]. This guide is compatible with Nomad 0.9 and above. If you are using an older version of Nomad, see the [LXC][lxc-docs] driver documentation.

## Reference Material

- Official [LXC][linux-containers] documentation
- Nomad [LXC][lxc-docs] external driver documentation

## Estimated Time to Complete

20 minutes

## Challenge

You need to deploy a workload using [Linux Containers][linux-containers-home]. Configure the client nodes that need to run this workload appropriately.

## Solution

Install and configure the LXC external driver plugin. Verify the configuration on the client node and deploy the application.

## Prerequisites

To perform the tasks described in this guide, you need to have a Nomad
environment with Consul installed. You can use this
[repo](https://github.com/hashicorp/nomad/tree/master/terraform#provision-a-nomad-cluster-in-the-cloud) to easily provision a sandbox environment. This guide will assume a cluster with one server node and one client node.

-> **Please Note:** This guide is for demo purposes and is only using a single server node. In a production cluster, 3 or 5 server nodes are recommended.

## Steps

### Step 1: Verify Client Node Configuration

External drivers must be placed in the [plugin_dir][plugin_dir] directory which defaults to [`data_dir`][data_dir]`/plugins`. Verify the `data_dir` directory on the client node configuration. If you are using the environment provided by this guide, the client configuration is located at `/etc/nomad.d/nomad.hcl`. The configuration file will show you that the `data_dir` directory is `/opt/nomad/data`. The relevant snippet of the configuration file is shown below:

```shell
$ cat /etc/nomad.d/nomad.hcl 
data_dir = "/opt/nomad/data"
bind_addr = "0.0.0.0"
...
```

### Step 2: Download and Install the LXC Driver 

Make a directory called `plugins` in [plugin_dir][plugin_dir] (which is `/opt/nomad/data` in our case) and download/place the [LXC driver][lxc-driver] in it. The following sequences of commands illustrate this process:

```shell
$ sudo mkdir /opt/nomad/data/plugins
$ curl -O /link/to/driver.zip
$ unzip nomad-driver-lxc.zip
Archive:  nomad-driver-lxc.zip
  inflating: nomad-driver-lxc   
$ sudo mv nomad-driver-lxc /opt/nomad/data/plugins
```
You can now delete the original zip file:

```shell
$ rm ./nomad-driver-lxc.zip
```

### Step 3: Verify the LXC Driver Status

After completing the previous steps, you do not need to explicitly enable the LXC driver in the client configuration, as it is enabled by default.

Restart the Nomad client service:

```shell
$ sudo systemctl restart nomad
```

After a few seconds, run the `nomad node status` command to verify the client node is ready:

```shell
$ nomad node status
ID        DC   Name             Class   Drain  Eligibility  Status
81c22a0c  dc1  ip-172-31-5-174  <none>  false  eligible     ready
```

You can now run the `nomad node status` command against the specific node ID to see which drivers are initialized on the client. In our case, the client node ID is `81c22a0c` (your client node ID will be different). You should see `lxc` appear in the `Driver Status` section as show below:

```shell
$ nomad node status 81c22a0c
ID            = 81c22a0c
Name          = ip-172-31-5-174
Class         = <none>
DC            = dc1
Drain         = false
Eligibility   = eligible
Status        = ready
Uptime        = 2h13m30s
Driver Status = docker,exec,java,lxc,mock_driver,raw_exec,rkt
...
```




## Next Steps


[data_dir]: /docs/configuration/index.html#data_dir
[linux-containers]: https://linuxcontainers.org/lxc/introduction/
[linux-containers-home]: https://linuxcontainers.org
[lxc-driver]: /coming/soon
[lxc-docs]: /docs/drivers/external/lxc.html
[plugin_dir]: /docs/configuration/index.html#plugin_dir