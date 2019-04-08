Upgrade Tests
=============

This directory has a set of scripts for Nomad upgrade testing. The cluster's configs
are also included here. For ease of testing, the configs and node names use names like
`"server1"` and `"client1"` and the data directories are in `"/tmp/server1"`, "/tmp/client1"`, etc

Start a cluster
===============
Use `run_cluster.sh` to start a 3 server, 2 client cluster locally.
The script takes the path to the Nomad binary. For example, to run a
0.8.7 Nomad cluster do:

```
$ ./run_cluster.sh /path/to/Nomad0.8.7
```
Run cluster also assumes that `consul` exists in your path and runs a dev agent

Stop nodes
==========
Use the `kill_node` script to stop nodes. Note that for quorum, you can only
stop one server at a time. For example, to stop server1 do :
```
$ ./kill_node.sh server1
```
This does `kill -9` and is not meant to test graceful shutdowns.

To stop client1 do :
```
$ ./kill_node.sh client1
```

Start nodes
===========
Use the `run_node` script to start nodes. In general, upgrade testing involves
shutting down a node that's running an older version of nomad and starting it
back up with a newer binary. For example, to run Nomad 0.9 as server1 do:
```
$ ./run_node.sh /path/to/Nomad0.9 server1
```
This will use the same data directory, and should rejoin the quorum

Another example - to start client1 do :
```
$ ./run_node.sh /path/to/Nomad0.9 client1
```