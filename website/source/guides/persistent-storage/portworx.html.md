---
layout: "guides"
page_title: "Persistent Storage with Portworx"
sidebar_current: "guides-persistent-storage"
description: |-
  There are multiple approaches to deploying stateful applications in Nomad.
  This guide uses Portworx deploy a MySQL database.
---

# Persistent Storage with Portworx

...

## Reference Material

- [Portworx on Nomad][portworx-nomad]

## Estimated Time to Complete

20 minutes

## Challenge

Deploy an HA MySQL database with a replication factor of 3, ensuring the data
will be replicated on 3 different client nodes.

## Solution

Configure Portworx on each client node in order to create a storage pool that
the MySQL task can use for storage and replication.

## Prerequisites

To perform the tasks described in this guide, you need to have a Nomad
environment with Consul installed. You can use this [repo][repo] to easily
provision a sandbox environment. This guide will assume a cluster with one
server node and three client nodes.

-> **Please Note:** This guide is for demo purposes and is only using a single
server node. In a production cluster, 3 or 5 server nodes are recommended.

## Steps

### Step 1: Ensure Block Device Requirements

* Portworx needs an unformatted and unmounted block device that it can fully
  manage. If you have provisioned a Nomad cluster in AWS using the environment
  provided in this guide, you already have an external block device ready to use
  (`/dev/xvdd`) with a capacity of 50 GB.

* Ensure your root volume's size is at least 20 GB. If you are using the
  environment provided in this guide, simply add the following line to your
  `terraform.tfvars` file:

  ```
  root_block_device_size = 20
  ```

### Step 2: Install the MySQL client

We will use the MySQL client to connect to our MySQL database and verify our data.
Ensure it is installed on each client node:

```
$ sudo apt install mysql-client
```

### Step 3: Set up the PX-OCI Bundle

Run the following command on each client node to set up the [PX-OCI][px-oci]
bundle:

```
sudo docker run --entrypoint /runc-entry-point.sh \
    --rm -i --privileged=true \
    -v /opt/pwx:/opt/pwx -v /etc/pwx:/etc/pwx \
    portworx/px-enterprise:2.0.2.3
```

If the command is successful, you will see output similar to the output show
below (the output has been abbreviated):

```
Unable to find image 'portworx/px-enterprise:2.0.2.3' locally
2.0.2.3: Pulling from portworx/px-enterprise
...
Status: Downloaded newer image for portworx/px-enterprise:2.0.2.3
Executing with arguments: 
INFO: Copying binaries...
INFO: Copying rootfs...
[###############################################################################[.....................................................................................................Total bytes written: 2303375360 (2.2GiB, 48MiB/s)
INFO: Done copying OCI content.
You can now run the Portworx OCI bundle by executing one of the following:

    # sudo /opt/pwx/bin/px-runc run [options]
    # sudo /opt/pwx/bin/px-runc install [options]
...
```

### Step 4: Configure Portworx OCI Bundle

Configure the Portworx OCI bundle on each client node by running the following
command (the values provided to the options will be different for your
environment):

```
$ sudo /opt/pwx/bin/px-runc install -k consul://172.31.49.111:8500 \
    -c my_test_cluster -s /dev/xvdd
```
* You can use client node you are on with the `-k` option since Consul is
  installed alongside Nomad

* Be sure to provide the `-s` option with your external block device path

If the configuration is successful, you will see the following output
(abbreviated):

```
INFO[0000] Rootfs found at /opt/pwx/oci/rootfs          
INFO[0000] PX binaries found at /opt/pwx/bin/px-runc    
INFO[0000] Initializing as version 2.0.2.3-c186a87 (OCI) 
...
INFO[0000] Successfully written /etc/systemd/system/portworx.socket 
INFO[0000] Successfully written /etc/systemd/system/portworx-output.service 
INFO[0000] Successfully written /etc/systemd/system/portworx.service 
```

Since we have created new unit files, please run the following command to reload
the systemd manager configuration:

```
sudo systemctl daemon-reload
```

### Step 5: Start Portworx and Check Status

Run the following command to start Portworx:

```
$ sudo systemctl start portworx
```
Verify the service:

```
$ sudo systemctl status portworx
● portworx.service - Portworx OCI Container
   Loaded: loaded (/etc/systemd/system/portworx.service; disabled; vendor preset
   Active: active (running) since Wed 2019-03-06 15:16:51 UTC; 1h 47min ago
     Docs: https://docs.portworx.com/runc
  Process: 28230 ExecStartPre=/bin/sh -c /opt/pwx/bin/runc delete -f portworx ||
 Main PID: 28238 (runc)
...
```
Wait a few moments (Portworx may still be initializing) and then check the
status of Portworx using the `pxctl` command. 

```
$ pxctl status
```

If everything is working properly, you should see the following output:

```
Status: PX is operational
License: Trial (expires in 31 days)
Node ID: 07113eef-0533-4de8-b1cf-4471c18a7cda
	IP: 172.31.53.231 
 	Local Storage Pool: 1 pool
	POOL	IO_PRIORITY	RAID_LEVEL	USABLE	USED	STATUS	ZONE	REGION
	0	LOW		raid0		50 GiB	4.4 GiB	Online	us-east-1c	us-east-1
	Local Storage Devices: 1 device
```
Once all nodes are configured, you should see a cluster summary with the total
capacity of the storage pool (if you're using the environment provided in this
guide, the total capacity will be 150 GB since the external block device
attached to each client nodes has a capacity of 50 GB):

```
Cluster Summary
	Cluster ID: my_test_cluster
	Cluster UUID: 705a1cbd-4d58-4a0e-a970-1e6b28375590
	Scheduler: none
	Nodes: 3 node(s) with storage (3 online)
...
Global Storage Pool
	Total Used    	:  13 GiB
	Total Capacity	:  150 GiB
```

### Step 6: Deploy a MySQL Database



## Next Steps

[portworx-nomad]: https://docs.portworx.com/install-with-other/nomad
[px-oci]: https://docs.portworx.com/install-with-other/docker/standalone/#why-oci
[repo]: https://github.com/hashicorp/nomad/tree/master/terraform#provision-a-nomad-cluster-in-the-cloud