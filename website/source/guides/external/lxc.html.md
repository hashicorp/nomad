---
layout: "guides"
page_title: "LXC"
sidebar_current: "guides-external-lxc"
description: |-
  Guide for using LXC external task driver plugin.
---

## LXC

The `lxc` driver provides an interface for using LXC for running application
containers. This guide walks through the steps involved in configuring a Nomad client agent to be able to run lxc jobs. You can download the external LXC driver [here][lxc_driver_download].

~> Note: This guide is compatible with Nomad 0.9 and above. If you are using an older version of Nomad, see the [LXC][lxc-docs] driver documentation.

## Reference Material

- Official [LXC][linux-containers] documentation
- Nomad [LXC][lxc-docs] external driver documentation
- Nomad LXC external driver [repo][lxc-driver-repo]

## Installation Instructions 

### Step 1: Install the `lxc` and `lxc-templates` Packages

Before deploying an LXC workload, you will need to install the `lxc` and `lxc-templates` packages which will provide the runtime and templates to start your container. Run the following command:

```shell
sudo apt install -y lxc lxc-templates
```

### Step 2: Download and Install the LXC Driver 

External drivers must be placed in the [plugin_dir][plugin_dir] directory which
defaults to [`data_dir`][data_dir]`/plugins`. Make a directory called `plugins` in [data_dir][data_dir] (which is `/opt/nomad/data` in the example below) and download/place the [LXC driver][lxc_driver_download] in it. The following sequence of commands illustrate this process:

```shell
$ sudo mkdir -p /opt/nomad/data/plugins
$ curl -O https://releases.hashicorp.com/nomad-driver-lxc/0.1.0-rc2/nomad-driver-lxc_0.1.0-rc2_linux_amd64.zip
$ unzip nomad-driver-lxc_0.1.0-rc2_linux_amd64.zip 
Archive:  nomad-driver-lxc_0.1.0-rc2_linux_amd64.zip
  inflating: nomad-driver-lxc
$ sudo mv nomad-driver-lxc /opt/nomad/data/plugins
```
You can now delete the original zip file:

```shell
$ rm ./nomad-driver-lxc*.zip
```

### Step 3: Verify the LXC Driver Status

After completing the previous steps, you do not need to explicitly enable the
LXC driver in the client configuration, as it is enabled by default.

Restart the Nomad client service:

```shell
$ sudo systemctl restart nomad
```

After a few seconds, run the `nomad node status` command to verify the client
node is ready:

```shell
$ nomad node status
ID        DC   Name             Class   Drain  Eligibility  Status
81c22a0c  dc1  ip-172-31-5-174  <none>  false  eligible     ready
```

You can now run the `nomad node status` command against the specific node ID to
see which drivers are initialized on the client. In our case, the client node ID
is `81c22a0c` (your client node ID will be different). You should see `lxc`
appear in the `Driver Status` section as show below:

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

### Step 4: Register the Nomad Job

You can run this [LXC example job][lxc-job] to register a Nomad job that deploys an LXC workload.

```shell
$ nomad run lxc.nomad
==> Monitoring evaluation "d8be10f4"
    Evaluation triggered by job "example-lxc"
    Allocation "4248c82e" created: node "81c22a0c", group "example"
    Allocation "4248c82e" status changed: "pending" -> "running" (Tasks are running)
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "d8be10f4" finished with status "complete"
```

### Step 5: Check the Status of the Job

Run the following command to check the status of the jobs in your
cluster:

```shell
$ nomad status
ID           Type     Priority  Status   Submit Date
example-lxc  service  50        running  2019-01-28T22:05:36Z
```
As shown above, our job is successfully running. You can see detailed
information about our specific job with the following command:

```shell
$ nomad status example-lxc
ID            = example-lxc
Name          = example-lxc
Submit Date   = 2019-01-28T22:05:36Z
Type          = service
Priority      = 50
Datacenters   = dc1
Status        = running
Periodic      = false
Parameterized = false

Summary
Task Group  Queued  Starting  Running  Failed  Complete  Lost
example     0       0         1        0       0         0

Allocations
ID        Node ID   Task Group  Version  Desired  Status   Created    Modified
4248c82e  81c22a0c  example     0        run      running  6m58s ago  6m47s ago
```

### More Configuration Options

The LXC driver is enabled by default in the client configuration. In order to
provide additional options to the LXC plugin, add [plugin
options][lxc_plugin_options] `volumes_enabled` and `lxc_path` for the `lxc`
driver in the client's configuration file like in the following example: 

```hcl
plugin "nomad-driver-lxc" {
  config {
    enabled = true
    volumes_enabled = true
    lxc_path = "/var/lib/lxc"
  }
}
```

[data_dir]: /docs/configuration/index.html#data_dir
[linux-containers]: https://linuxcontainers.org/lxc/introduction/
[linux-containers-home]: https://linuxcontainers.org
[lxc_driver_download]: https://releases.hashicorp.com/nomad-driver-lxc 
[lxc-driver-repo]: https://github.com/hashicorp/nomad-driver-lxc
[lxc-docs]: /docs/drivers/external/lxc.html
[lxc-job]: https://github.com/hashicorp/nomad-education-content/blob/master/lxc.nomad
[lxc_plugin_options]: /docs/drivers/external/lxc.html#plugin-options
[plugin_dir]: /docs/configuration/index.html#plugin_dir
[plugin_syntax]: /docs/configuration/plugin.html
