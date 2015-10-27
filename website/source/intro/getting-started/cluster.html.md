---
layout: "intro"
page_title: "Clustering"
sidebar_current: "getting-started-cluster"
description: |-
  Join another Nomad client to create your first cluster.
---

# Clustering

We have started our first agent and run a job against it in development mode.
This demonstrates the ease of use and the workflow of Nomad, but did not show how
this could be extended to a scalable, production-grade configuration. In this step,
we will create our first real cluster with multiple nodes.

## Starting the Server

The first step is to create the config file for the server. Either download
the file from the [repository here](https://github.com/hashicorp/nomad/tree/master/demo/vagrant),
or paste this into a file called `server.hcl`:

```
# Increase log verbosity
log_level = "DEBUG"

# Setup data dir
data_dir = "/tmp/server1"

# Enable the server
server {
    enabled = true

    # Self-elect, should be 3 or 5 for production
    bootstrap_expect = 1
}
```

This is a fairly minimal server configuration file, but it
is enough to start an agent in server only mode and have it
elected as a leader. The major change that should be made for
production is to run more than one server, and to change the
corresponding `bootstrap_expect` value.

Once the file is created, start the agent in a new tab:

```
$ sudo nomad agent -config server.hcl
==> WARNING: Bootstrap mode enabled! Potentially unsafe operation.
==> Starting Nomad agent...
==> Nomad agent configuration:

                 Atlas: <disabled>
                Client: false
             Log Level: DEBUG
                Region: global (DC: dc1)
                Server: true

==> Nomad agent started! Log data will stream in below:

    [INFO] serf: EventMemberJoin: nomad.global 127.0.0.1
    [INFO] nomad: starting 4 scheduling worker(s) for [service batch _core]
    [INFO] raft: Node at 127.0.0.1:4647 [Follower] entering Follower state
    [INFO] nomad: adding server nomad.global (Addr: 127.0.0.1:4647) (DC: dc1)
    [WARN] raft: Heartbeat timeout reached, starting election
    [INFO] raft: Node at 127.0.0.1:4647 [Candidate] entering Candidate state
    [DEBUG] raft: Votes needed: 1
    [DEBUG] raft: Vote granted. Tally: 1
    [INFO] raft: Election won. Tally: 1
    [INFO] raft: Node at 127.0.0.1:4647 [Leader] entering Leader state
    [INFO] nomad: cluster leadership acquired
    [INFO] raft: Disabling EnableSingleNode (bootstrap)
    [DEBUG] raft: Node 127.0.0.1:4647 updated peer set (2): [127.0.0.1:4647]
```

We can see above that client mode is disabled, and that we are
only running as the server. This means that this server will manage
state and make scheduling decisions but will not run any tasks.
Now we need some agents to run tasks!

## Starting the Clients

Similar to the server, we must first configure the clients. Either download
the configuration for client1 and client2 from the
[repository here](https://github.com/hashicorp/nomad/tree/master/demo/vagrant), or
paste the following into `client1.hcl`:

```
# Increase log verbosity
log_level = "DEBUG"

# Setup data dir
data_dir = "/tmp/client1"

# Enable the client
client {
    enabled = true

    # For demo assume we are talking to server1. For production,
    # this should be like "nomad.service.consul:4647" and a system
    # like Consul used for service discovery.
    servers = ["127.0.0.1:4647"]
}

# Modify our port to avoid a collision with server1
ports {
    http = 5656
}
```

Copy that file to `client2.hcl` and change the `data_dir` to
be "/tmp/client2" and the `http` port to 5657. Once you've created
both `client1.hcl` and `client2.hcl`, open a tab for each and
start the agents:

```
$ sudo nomad agent -config client1.hcl
==> Starting Nomad agent...
==> Nomad agent configuration:

                 Atlas: <disabled>
                Client: true
             Log Level: DEBUG
                Region: global (DC: dc1)
                Server: false

==> Nomad agent started! Log data will stream in below:

    [DEBUG] client: applied fingerprints [host memory storage arch cpu]
    [DEBUG] client: available drivers [docker exec]
    [DEBUG] client: node registration complete
    ...
```

In the output we can see the agent is running in client mode only.
This agent will be available to run tasks but will not participate
in managing the cluster or making scheduling decisions.

Using the [`node-status` command](/docs/commands/node-status.html)
we should see both nodes in the `ready` state:

```
$ nomad node-status
ID                                    DC   Name   Class   Drain  Status
f7780117-2cae-8ee9-4b36-f34dd796ab02  dc1  nomad  <none>  false  ready
ffb5b55a-6059-9ec7-6108-23a2bbba95da  dc1  nomad  <none>  false  ready
```

We now have a simple three node cluster running. The only difference
between a demo and full production cluster is that we are running a
single server instead of three or five.

## Submit a Job

Now that we have a simple cluster, we can use it to schedule a job.
We should still have the `example.nomad` job file from before, but
verify that the `count` is still set to 3.

Then, use the [`run` command](/docs/commands/run.html) to submit the job:

```
$ nomad run example.nomad
==> Monitoring evaluation "77e5075f-2a1b-9cce-d14e-fe98cca9e17f"
    Evaluation triggered by job "example"
    Allocation "711edd85-f183-99ea-910a-6445b23d79e4" created: node "ffb5b55a-6059-9ec7-6108-23a2bbba95da", group "cache"
    Allocation "98218a8a-627c-308f-8941-acdbffe1940c" created: node "f7780117-2cae-8ee9-4b36-f34dd796ab02", group "cache"
    Allocation "e8957a7f-6fff-f61f-2878-57715c26725d" created: node "f7780117-2cae-8ee9-4b36-f34dd796ab02", group "cache"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "77e5075f-2a1b-9cce-d14e-fe98cca9e17f" finished with status "complete"
```

We can see in the output that the scheduler assigned two of the
tasks for one of the client nodes and the remaining task to the
second client.

We can again use the [`status` command](/docs/commands/status.html) to verify:

```
$ nomad status example
ID          = example
Name        = example
Type        = service
Priority    = 50
Datacenters = dc1
Status      = <none>

==> Evaluations
ID                                    Priority  TriggeredBy   Status
77e5075f-2a1b-9cce-d14e-fe98cca9e17f  50        job-register  complete

==> Allocations
ID                                    EvalID                                NodeID                                TaskGroup  Desired  Status
711edd85-f183-99ea-910a-6445b23d79e4  77e5075f-2a1b-9cce-d14e-fe98cca9e17f  ffb5b55a-6059-9ec7-6108-23a2bbba95da  cache      run      running
98218a8a-627c-308f-8941-acdbffe1940c  77e5075f-2a1b-9cce-d14e-fe98cca9e17f  f7780117-2cae-8ee9-4b36-f34dd796ab02  cache      run      running
e8957a7f-6fff-f61f-2878-57715c26725d  77e5075f-2a1b-9cce-d14e-fe98cca9e17f  f7780117-2cae-8ee9-4b36-f34dd796ab02  cache      run      running
```

We can see that all our tasks have been allocated and are running.
Once we are satisfied that our job is happily running, we can tear
it down with `nomad stop`.

## Next Steps

We've now concluded the getting started guide, however there are a number
of [next steps](next-steps.html) to get started with Nomad.

