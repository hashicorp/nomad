---
layout: "guides"
page_title: "Stateful Workloads with Nomad Host Volumes"
sidebar_current: "guides-stateful-workloads-host-volumes"
description: |-
  There are multiple approaches to deploying stateful applications in Nomad.
  This guide uses Nomad Host to Volumes deploy a MySQL database.
---

# Stateful Workloads with Nomad Host Volumes

~> **Note:** This guide requires Nomad 0.10.0 or later.

Nomad Host Volumes can manage storage for stateful workloads running inside your
Nomad cluster. This guide walks you through deploying a MySQL workload to a node
containing supporting storage.

Nomad host volumes provide a more workload-agnostic way to specify resources,
available for Nomad drivers like `exec`, `java`, and `docker`. See the
[`host_volume` specification][host_volume spec] for more information about
supported drivers.

Nomad is also aware of host volumes during the scheduling process, enabling it
to make scheduling decisions based on the availability of host volumes on a
specific client.

This can be contrasted with Nomad support for Docker volumes. Because Docker
volumes are managed outside of Nomad and the Nomad scheduled is not aware of
them, Docker volumes have to either be deployed to all clients or operators have
to use an additional, manually-maintained constraint to inform the scheduler
where they are present.

## Reference material

- [Nomad `host_volume` specification][host_volume spec]
- [Nomad `volume` specification][volume spec]
- [Nomad `volume_mount` specification][volume_mount spec]

## Estimated time to complete

20 minutes

## Challenge

Deploy a MySQL database that needs to be able to persist data without using
operator-configured Docker volumes.

## Solution

Configure Nomad Host Volumes on a Nomad client node in order to persist data
in the event that the container is restarted.

## Prerequisites

To perform the tasks described in this guide, you need to have a Nomad
environment with Consul installed. You can use this [project][repo] to easily
provision a sandbox environment. This guide will assume a cluster with one
server node and three client nodes.

~> **Please Note:** This guide is for demo purposes and is only using a single
server node. In a production cluster, 3 or 5 server nodes are recommended.

### Prerequisite 1: Install the MySQL client

We will use the MySQL client to connect to our MySQL database and verify our data.
Ensure it is installed on a node with access to port 3306 on your Nomad clients:

Ubuntu:

```bash
sudo apt install mysql-client
```

CentOS:

```bash
sudo yum install mysql
```

macOS via Homebrew:

```bash
brew install mysql-client
```

### Step 1: Create a directory to use as a mount target

On a Nomad client node in your cluster, create a directory that will be used for
persisting the MySQL data. For this example, let's create the directory
`/opt/mysql/data`.

```bash
sudo mkdir -p /opt/mysql/data
```

You might need to change the owner on this folder if the Nomad client does not
run as the `root` user.

```bash
sudo chown «Nomad user» /opt/mysql/data
```

### Step 2: Configure the `mysql` host volume on the client

Edit the Nomad configuration on this Nomad client to create the Host Volume.

Add the following to the `client` stanza of your Nomad configuration:

```hcl
  host_volume "mysql" {
    path      = "/opt/mysql/data"
    read_only = false
  }
```

Save this change, and then restart the Nomad service on this client to make the
Host Volume active. While still on the client, you can easily verify that the
host volume is configured by using the `nomad node status` command as shown
below:

```shell
$ nomad node status -short -self
ID           = 12937fa7
Name         = ip-172-31-15-65
Class        = <none>
DC           = dc1
Drain        = false
Eligibility  = eligible
Status       = ready
Host Volumes = mysql
Drivers      = docker,exec,java,mock_driver,raw_exec,rkt
...
```

### Step 3: Create the `mysql.nomad` job file

We are now ready to deploy a MySQL database that can use Nomad Host Volumes for
storage. Create a file called `mysql.nomad` and provide it the following
contents:

```hcl
job "mysql-server" {
  datacenters = ["dc1"]
  type        = "service"

  group "mysql-server" {
    count = 1

    volume "mysql" {
      type      = "host"
      read_only = false
      source    = "mysql"
    }

    restart {
      attempts = 10
      interval = "5m"
      delay    = "25s"
      mode     = "delay"
    }

    task "mysql-server" {
      driver = "docker"

      volume_mount {
        volume      = "mysql"
        destination = "/var/lib/mysql"
        read_only   = false
      }

      env = {
        "MYSQL_ROOT_PASSWORD" = "password"
      }

      config {
        image = "hashicorp/mysql-portworx-demo:latest"

        port_map {
          db = 3306
        }
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

#### Notes about the above job specification

- The service name is `mysql-server` which we will use later to connect to the
  database.

- The `read_only` argument is supplied on all of the volume-related stanzas in
  to help highlight all of the places you would need to change to make a
  read-only volume mount. Please see the [`host_volume`][host_volume spec],
  [`volume`][volume spec], and [`volume_mount`][volume_mount spec] specifications
  for more details.

- For lower-memory instances, you might need to reduce the requested memory in
  the resources stanza to harmonize with available resources in your cluster.

### Step 4: Deploy the MySQL database

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

### Step 5: Connect to MySQL

Using the mysql client (installed in [Prerequisite 1]), connect to the database
and access the information:

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

### Step 6: Add data to MySQL

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

### Step 7: Stop and purge the database job

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

In more advanced cases, the directory backing the host volume could be a mounted
network filesystem, like NFS, or cluster-aware filesystem, like glusterFS. This
can enable more complex, automatic failure-recovery scenarios in the event of a
node failure.

### Step 8: Re-deploy the database

Using the `mysql.nomad` job file from [Step 3], re-deploy the database to the
Nomad cluster.

```
==> Monitoring evaluation "61b4f648"
    Evaluation triggered by job "mysql-server"
    Allocation "8e1324d2" created: node "be8aad4e", group "mysql-server"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "61b4f648" finished with status "complete"
```

### Step 9: Verify data

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

### Step 10: Tidying up

Once you have completed this guide, you should perform the following cleanup steps:

- Stop and purge the `mysql-server` job.

- Remove the `host_volume "mysql"` stanza from your Nomad client configuration
  and restart the Nomad service on that client

- Remove the /opt/mysql/data folder and as much of the directory tree that you
  no longer require.

[Prerequisite 1]: #prerequisite-1-install-the-mysql-client
[Step 3]: #step-3-create-the-mysql-nomad-job-file
[host_volume spec]: /docs/configuration/client.html#host_volume-stanza
[volume spec]: /docs/job-specification/volume.html
[volume_mount spec]: /docs/job-specification/volume_mount.html
[password-security]: https://dev.mysql.com/doc/refman/8.0/en/password-security.html
[repo]: https://github.com/hashicorp/nomad/tree/master/terraform#provision-a-nomad-cluster-in-the-cloud
