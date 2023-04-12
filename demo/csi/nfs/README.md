# NFS plugins demo

As easy* as `../hostpath` to run locally, but with separate Controller and 
Node Plugins from
[democratic-csi](https://github.com/democratic-csi/democratic-csi).

It is backed by NFS test server container
[atlassian/nfs-server-test](https://hub.docker.com/r/atlassian/nfs-server-test)
for easy setup.

## Usage

### Setup Nomad

Run on linux, as provided in this repo's Vagrantfile:

```
vagrant up linux
```

Create NFS dir on the host:

```
sudo mkdir -p /srv/host-nfs
```

Run a Nomad agent using the `agent.hcl` in this directory:

```
sudo nomad agent -config=agent.hcl
```

You need that agent config to provide the host volume used by NFS and
the controller plugin, and to allow privileged mode for the node plugin.

### Job setup

The setup script runs all the things for the demo:

```
./setup.sh
```

### Observe the demo web server

On the host machine (or elsewhere if you have ports open),
a web server shows the date stamp of the time of its first launch.

The web server listens on port `8080`, so from the host machine:

```
curl localhost:8080
```

That date is stored in NFS, so restarts or reschedules or purges and re-runs
of the web service will all show the same date.  That file will not be removed
until the volume is deleted, either manually or during the following clean-up.

### Clean up demo parts

`./teardown.sh` deletes all the things created during Job setup.

It does not delete the Nomad data dir from `/tmp/nomad`,
nor `/srv/host-nfs`.
