---
layout: "intro"
page_title: "Running Nomad"
sidebar_current: "getting-started-running"
description: |-
  Learn about the Nomad agent, and the lifecycle of running and stopping.
---

# Running Nomad

Nomad relies on a long running agent on every machine in the cluster.
The agent can run either in server or client mode. Each region must
have at least one server, though a cluster of 3 or 5 servers is recommended.
A single server deployment is _**highly**_ discouraged as data loss is inevitable
in a failure scenario.

All other agents run in client mode. A client is a very lightweight
process that registers the host machine, performs heartbeating, and runs any tasks
that are assigned to it by the servers. The agent must be run on every node that
is part of the cluster so that the servers can assign work to those machines.

## Starting the Agent

For simplicity, we will run a single Nomad agent in development mode. This mode
is used to quickly start an agent that is acting as a client and server to test
job configurations or prototype interactions. It should _**not**_ be used in
production as it does not persist state.

```
vagrant@nomad:~$ sudo nomad agent -dev
    No configuration files loaded
==> Starting Nomad agent...
==> Nomad agent configuration:

                 Atlas: <disabled>
                Client: true
             Log Level: DEBUG
                Region: global (DC: dc1)
                Server: true

==> Nomad agent started! Log data will stream in below:

    2016/08/23 21:26:03 [INFO] serf: EventMemberJoin: nomad.global 127.0.0.1
    2016/08/23 21:26:03.853216 [INFO] nomad: starting 1 scheduling worker(s) for [service batch system _core]
    2016/08/23 21:26:03.854711 [INFO] client: using state directory /tmp/NomadClient632906511
    2016/08/23 21:26:03.854732 [INFO] client: using alloc directory /tmp/NomadClient174428962
    2016/08/23 21:26:03.854761 [DEBUG] client: built-in fingerprints: [arch cgroup cpu env_aws env_gce host memory network nomad storage]
    2016/08/23 21:26:03.854879 [INFO] fingerprint.cgroups: cgroups are available
    2016/08/23 21:26:03.854951 [DEBUG] fingerprint.cpu: frequency: 2294 MHz
    2016/08/23 21:26:03.854954 [DEBUG] fingerprint.cpu: core count: 1
    2016/08/23 21:26:03 [INFO] raft: Node at 127.0.0.1:4647 [Follower] entering Follower state (Leader: "")
    2016/08/23 21:26:03.861876 [INFO] nomad: adding server nomad.global (Addr: 127.0.0.1:4647) (DC: dc1)
    2016/08/23 21:26:03.861924 [DEBUG] client: fingerprinting cgroup every 15s
    2016/08/23 21:26:05 [WARN] raft: Heartbeat timeout from "" reached, starting election
    2016/08/23 21:26:05 [INFO] raft: Node at 127.0.0.1:4647 [Candidate] entering Candidate state
    2016/08/23 21:26:05 [DEBUG] raft: Votes needed: 1
    2016/08/23 21:26:05 [DEBUG] raft: Vote granted from 127.0.0.1:4647. Tally: 1
    2016/08/23 21:26:05 [INFO] raft: Election won. Tally: 1
    2016/08/23 21:26:05 [INFO] raft: Node at 127.0.0.1:4647 [Leader] entering Leader state
    2016/08/23 21:26:05 [INFO] raft: Disabling EnableSingleNode (bootstrap)
    2016/08/23 21:26:05 [DEBUG] raft: Node 127.0.0.1:4647 updated peer set (2): [127.0.0.1:4647]
    2016/08/23 21:26:05.685611 [INFO] nomad: cluster leadership acquired
    2016/08/23 21:26:05.685685 [DEBUG] leader: reconciling job summaries at index: 0
    2016/08/23 21:26:05.855402 [DEBUG] fingerprint.env_aws: Error querying AWS Metadata URL, skipping
    2016/08/23 21:26:07.855934 [DEBUG] fingerprint.env_gce: Could not read value for attribute "machine-type"
    2016/08/23 21:26:07.855942 [DEBUG] fingerprint.env_gce: Error querying GCE Metadata URL, skipping
    2016/08/23 21:26:07.860576 [DEBUG] fingerprint.network: Detected interface lo with IP 127.0.0.1 during fingerprinting
    2016/08/23 21:26:07.861368 [DEBUG] fingerprint.network: Unable to read link speed from /sys/class/net/lo/speed
    2016/08/23 21:26:07.861376 [DEBUG] fingerprint.network: Unable to read link speed; setting to default 100
    2016/08/23 21:26:07.863896 [DEBUG] client: applied fingerprints [arch cgroup cpu host memory network nomad storage]
    2016/08/23 21:26:07.863914 [DEBUG] driver.exec: exec driver is enabled
    2016/08/23 21:26:07.864114 [DEBUG] client: fingerprinting exec every 15s
    2016/08/23 21:26:07.864966 [DEBUG] driver.docker: using client connection initialized from environment
    2016/08/23 21:26:07.867270 [DEBUG] client: available drivers [exec raw_exec docker]
    2016/08/23 21:26:07.868498 [DEBUG] client: fingerprinting docker every 15s
    2016/08/23 21:26:07.872181 [DEBUG] client: node registration complete
    2016/08/23 21:26:07.872209 [DEBUG] client: periodically checking for node changes at duration 5s
    2016/08/23 21:26:07.872352 [DEBUG] client: updated allocations at index 1 (pulled 0) (filtered 0)
    2016/08/23 21:26:07.872411 [DEBUG] client: allocs: (added 0) (removed 0) (updated 0) (ignore 0)
    2016/08/23 21:26:07.879168 [DEBUG] client: state updated to ready
    2016/08/23 21:26:07.879907 [DEBUG] consul.syncer: error in syncing: 1 error(s) occurred:

* Get http://127.0.0.1:8500/v1/agent/services: dial tcp 127.0.0.1:8500: getsockopt: connection refused
```

As you can see, the Nomad agent has started and has output some log
data. From the log data, you can see that our agent is running in both
client and server mode, and has claimed leadership of the cluster.
Additionally, the local client has been registered and marked as ready.

-> **Note:** Typically any agent running in client mode must be run with root level
privilege. Nomad makes use of operating system primitives for resource isolation
which require elevated permissions. The agent will function as non-root, but
certain task drivers will not be available.

## Cluster Nodes

If you run [`nomad node-status`](/docs/commands/node-status.html) in another terminal, you
can see the registered nodes of the Nomad cluster:

```text
$ vagrant ssh
...

$ nomad node-status
ID        Datacenter  Name   Class   Drain  Status
171a583b  dc1         nomad  <none>  false  ready
```

The output shows our Node ID, which is a randomly generated UUID,
its datacenter, node name, node class, drain mode and current status.
We can see that our node is in the ready state, and task draining is
currently off.

The agent is also running in server mode, which means it is part of
the [gossip protocol](/docs/internals/gossip.html) used to connect all
the server instances together. We can view the members of the gossip
ring using the [`server-members`](/docs/commands/server-members.html) command:

```text
$ nomad server-members
Name          Address    Port  Status  Leader  Protocol  Build  Datacenter  Region
nomad.global  127.0.0.1  4648  alive   true    2         0.4.1  dc1         global
```

The output shows our own agent, the address it is running on, its
health state, some version information, and the datacenter and region.
Additional metadata can be viewed by providing the `-detailed` flag.

## <a name="stopping"></a>Stopping the Agent

You can use `Ctrl-C` (the interrupt signal) to halt the agent.
By default, all signals will cause the agent to forcefully shutdown.
The agent [can be configured](/docs/agent/config.html) to gracefully
leave on either the interrupt or terminate signals.

After interrupting the agent, you should see it leave the cluster
and shut down:

```
^C==> Caught signal: interrupt
    [DEBUG] http: Shutting down http server
    [INFO] agent: requesting shutdown
    [INFO] client: shutting down
    [INFO] nomad: shutting down server
    [WARN] serf: Shutdown without a Leave
    [INFO] agent: shutdown complete
```

By gracefully leaving, Nomad clients update their status to prevent
further tasks from being scheduled and to start migrating any tasks that are
already assigned. Nomad servers notify their peers they intend to leave.
When a server leaves, replication to that server stops. If a server fails,
replication continues to be attempted until the node recovers. Nomad will
automatically try to reconnect to _failed_ nodes, allowing it to recover from
certain network conditions, while _left_ nodes are no longer contacted.

If an agent is operating as a server, a graceful leave is important to avoid
causing a potential availability outage affecting the
[consensus protocol](/docs/internals/consensus.html). If a server does
forcefully exit and will not be returning into service, the
[`server-force-leave` command](/docs/commands/server-force-leave.html) should
be used to force the server from a _failed_ to a _left_ state.

## Next Steps

If you shut down the development Nomad agent as instructed above, ensure that it is back up and running again and let's try to [run a job](jobs.html)!
