---
layout: "guides"
page_title: "Stateful Workloads with Portworx"
sidebar_current: "guides-stateful-workloads"
description: |-
  There are multiple approaches to deploying stateful applications in Nomad.
  This guide uses Portworx deploy a MySQL database.
---

# Stateful Workloads with Portworx

Portworx integrates with Nomad and can manage storage for stateful workloads
running inside your Nomad cluster. This guide walks you through deploying an HA
MySQL workload.

## Reference Material

- [Portworx on Nomad][portworx-nomad]

## Estimated Time to Complete

20 minutes

## Challenge

Deploy an HA MySQL database with a replication factor of 3, ensuring the data
will be replicated on 3 different client nodes.

## Solution

Configure Portworx on each Nomad client node in order to create a storage pool
that the MySQL task can use for storage and replication.

## Prerequisites

To perform the tasks described in this guide, you need to have a Nomad
environment with Consul installed. You can use this [repo][repo] to easily
provision a sandbox environment. This guide will assume a cluster with one
server node and three client nodes.

~> **Please Note:** This guide is for demo purposes and is only using a single
server node. In a production cluster, 3 or 5 server nodes are recommended.

## Steps

### Step 1: Ensure Block Device Requirements

* Portworx needs an unformatted and unmounted block device that it can fully
  manage. If you have provisioned a Nomad cluster in AWS using the environment
  provided in this guide, you already have an external block device ready to use
  (`/dev/xvdd`) with a capacity of 50 GB.

* Ensure your root volume's size is at least 20 GB. If you are using the
  environment provided in this guide, add the following line to your
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

### Step 6: Create a Portworx Volume

Run the following command to create a Portworx volume that our job will be able
to use:

```
$ pxctl volume create -s 10 -r 3 mysql
```
You should see output similar to what is shown below:

```
Volume successfully created: 693373920899724151
```

* Please note from the options provided that the name of the volume we created
  is `mysql` and the size is 10 GB.

* We have configured a replication factor of 3 which ensures our data is
  available on all 3 client nodes.

Run `pxctl volume inspect mysql` to verify the status of the volume:

```
$ pxctl volume inspect mysql
Volume	:  693373920899724151
	Name            	 :  mysql
	Size            	 :  10 GiB
	Format          	 :  ext4
	HA              	 :  3
...
	Replica sets on nodes:
		Set 0
		  Node 		 : 172.31.58.210 (Pool 0)
		  Node 		 : 172.31.51.110 (Pool 0)
		  Node 		 : 172.31.48.98 (Pool 0)
	Replication Status	 :  Up
```

### Step 7: Create the `mysql.nomad` Job File

We are now ready to deploy a MySQL database that can use Portworx for storage.
Create a file called `mysql.nomad` and provide it the following contents:

```
job "mysql-server" {
  datacenters = ["dc1"]
  type        = "service"

  group "mysql-server" {
    count = 1

    restart {
      attempts = 10
      interval = "5m"
      delay    = "25s"
      mode     = "delay"
    }

    task "mysql-server" {
      driver = "docker"

      env = {
        "MYSQL_ROOT_PASSWORD" = "password"
      }

      config {
        image = "hashicorp/mysql-portworx-demo:latest"

        port_map {
          db = 3306
        }

        volumes = [
          "mysql:/var/lib/mysql"
        ]

        volume_driver = "pxd"
      }

      resources {
        cpu    = 500
        memory = 1024

        network {
          port "db" {
            static = 3306
          }
        }
      }

      service {
        name = "mysql-server"
        port = "db"

        check {
          type     = "tcp"
          interval = "10s"
          timeout  = "2s"
        }
      }
    }
  }
}
```

* Please note from the job file that we are using the `pxd` volume driver that
  has been configured from the previous steps.

* The service name is `mysql-server` which we will use later to connect to the
  database.

### Step 8: Deploy the MySQL Database

Register the job file you created in the previous step with the following
command:

```
$ nomad run mysql.nomad 
==> Monitoring evaluation "aa478d82"
    Evaluation triggered by job "mysql-server"
    Allocation "6c3b3703" created: node "be8aad4e", group "mysql-server"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "aa478d82" finished with status "complete"
```

Check the status of the allocation and ensure the task is running:

```
$ nomad status mysql-server
ID            = mysql-server
...
Summary
Task Group    Queued  Starting  Running  Failed  Complete  Lost
mysql-server  0       0         1        0       0         0
```

### Step 9: Connect to MySQL 

Using the mysql client (installed in [Step
2](#step-2-install-the-mysql-client)), connect to the database and access the
information:

```
mysql -h mysql-server.service.consul -u web -p -D itemcollection
```
The password for this demo database is `password`.

~> **Please Note:** This guide is for demo purposes and does not follow best
practices for securing database passwords. See [Keeping Passwords
Secure][password-security] for more information.

Consul is installed alongside Nomad in this cluster so we were able to
connect using the `mysql-server` service name we registered with our task in
our job file.

### Step 10: Add Data to MySQL

Once you are connected to the database, verify the table `items` exists:

```
mysql> show tables;
+--------------------------+
| Tables_in_itemcollection |
+--------------------------+
| items                    |
+--------------------------+
1 row in set (0.00 sec)
```

Display the contents of this table with the following command:

```
mysql> select * from items;
+----+----------+
| id | name     |
+----+----------+
|  1 | bike     |
|  2 | baseball |
|  3 | chair    |
+----+----------+
3 rows in set (0.00 sec)
```

Now add some data to this table (after we terminate our database in Nomad and
bring it back up, this data should still be intact):

```
mysql> INSERT INTO items (name) VALUES ('glove');
```

Run the `INSERT INTO` command as many times as you like with different values.

```
mysql> INSERT INTO items (name) VALUES ('hat');
mysql> INSERT INTO items (name) VALUES ('keyboard');
```
Once you you are done, type `exit` and return back to the Nomad client command
line:

```
mysql> exit
Bye
```

### Step 11: Stop and Purge the Database Job

Run the following command to stop and purge the MySQL job from the cluster:

```
$ nomad stop -purge mysql-server
==> Monitoring evaluation "6b784149"
    Evaluation triggered by job "mysql-server"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "6b784149" finished with status "complete"
```

Verify no jobs are running in the cluster:

```
$ nomad status
No running jobs
```
You can optionally stop the nomad service on whichever node you are on and move
to another node to simulate a node failure.

### Step 12: Re-deploy the Database

Using the `mysql.nomad` job file from [Step
6](#step-6-create-the-mysql-nomad-job-file), re-deploy the database to the Nomad
cluster.

```
==> Monitoring evaluation "61b4f648"
    Evaluation triggered by job "mysql-server"
    Allocation "8e1324d2" created: node "be8aad4e", group "mysql-server"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "61b4f648" finished with status "complete"
```

### Step 13: Verify Data

Once you re-connect to MySQL, you should be able to see that the information you
added prior to destroying the database is still present:

```
mysql> select * from items;
+----+----------+
| id | name     |
+----+----------+
|  1 | bike     |
|  2 | baseball |
|  3 | chair    |
|  4 | glove    |
|  5 | hat      |
|  6 | keyboard |
+----+----------+
6 rows in set (0.00 sec)
```

[password-security]: https://dev.mysql.com/doc/refman/8.0/en/password-security.html
[portworx-nomad]: https://docs.portworx.com/install-with-other/nomad
[px-oci]: https://docs.portworx.com/install-with-other/docker/standalone/#why-oci
[repo]: https://github.com/hashicorp/nomad/tree/master/terraform#provision-a-nomad-cluster-in-the-cloud
