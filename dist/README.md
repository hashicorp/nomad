# Dist

The `dist` folder contains sample configs for various platforms.

## Overview

"server" refers to a Nomad agent configured in server mode. Typically you will have 3 or 5 server nodes in your cluster. These are responsible for scheduling and tracking cluster state so these nodes are generally 100% dedicated to this purpose, or may be shared with a similar service like consul leader nodes.

"client" refers to the Nomad agent that is installed on every machine in your cluster, and is responsible for running jobs.

On unixes we will place agent configs under `/etc/nomad/`. Data is stored under `/var/lib/nomad/`.

## Agent Configs

Nomad accepts either a file or directory for configuration. When using a configuration directory, files are read in deterministic order and each subsequent config appends (or overrides) options.

The following files are provided:

- `_common.conf`
- `server.conf`
- `client.conf`

These should be placed in `/etc/nomad/` depending on the node's role. You should use `server.conf` to configure a node as a server (which is responsible for scheduling) or `client.conf` to configure a node as a client (which is responsible for running workloads). Both type of agents should include `_common.conf`.

See <https://nomadproject.io/docs/agent/config.html> for details about how to configure Nomad.

## Upstart

On systems using upstart the basic upstart file under `upstart/nomad.conf` starts and stops the nomad agent. Place it under `/etc/init/nomad.conf`.

You can control Nomad with `start|stop|restart nomad`.

## Systemd

On systems using systemd the basic systemd file under `systemd/nomad.service` starts and stops the nomad agent. Place it under `/etc/systemd/system/nomad.service`.

You can control Nomad with `systemctl start|stop|restart nomad`.