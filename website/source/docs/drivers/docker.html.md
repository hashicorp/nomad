---
layout: "docs"
page_title: "Drivers: Docker"
sidebar_current: "docs-drivers-docker"
description: |-
  The Docker task driver is used to run Docker based tasks.
---

# Docker Driver

Name: `docker`

The `docker` driver provides a first-class Docker workflow on Nomad. The Docker
driver handles downloading containers, mapping ports, and starting, watching,
and cleaning up when containers.

## Task Configuration

The `docker` driver supports the following configuration in the job specification:

* `image` - (Required) The Docker image to run. The image may include a tag or
  custom URL. By default it will be fetched from Docker Hub.

### Port Mapping

Nomad uses port binding to expose services running in containers using the port
space on the host's interface. For example, Nomad host running on `1.2.3.4` may
allocate port `22333` to a task, so you would access that service via
`1.2.3.4:22333`.

Nomad provides automatic and manual mapping schemes for Docker. You can use
either or both schemes for a task. Nomad binds both tcp and udp protocols to
ports used for Docker containers. This is not configurable.

Note: You are not required to map any ports, for example if your task is running
a crawler or aggregator and does not provide a network service. Tasks without a
port mapping will still be able to make outbound network connections.

#### Automatic Port Mapping

Typically when you create a Docker container you configure the service to start
listening on a port (or ports) when you start the container. For example, redis
starts listening on `6379` when you `Docker run redis`. Nomad supports this by
mapping the random port to the port inside the container.

You need to tell Nomad which ports your container is using so Nomad can map
allocated ports for you. You do so by specifying a **numeric port value** for
the `dynamic_ports` option in your job specification.

```
dynamic_ports = ["6379"]
# or
dynamic_ports = [6379]
```

This instructs Nomad to create a port mapping from the random port on the host
to the port inside the container. So in our example above, when you contact the
host on `1.2.3.4:22333` you will actually hit the service running inside the
container on port `6379`. You can see which port was actually bound by reading the
`NOMAD_PORT_6379` [environment variable](/docs/jobspec/environment.html).

In most cases, the automatic port mapping will be the easiest to use, but you
can also use manual port mapping (described below).

#### Manual Port Mapping

The `dynamic_ports` option takes any alphanumeric string as a label, so you could
also specify a label for the port like `http` or `admin` to designate how the
port will be used.

In this case, Nomad doesn't know which container port to map to, so it maps 1:1
with the host port. For example, `1.2.3.4:22333` will map to `22333` inside the
container.

```
dynamic_ports = ["http"]
```

Your process will need to read the `NOMAD_PORT_HTTP` environment variable to
determine which port to bind to.

## Client Requirements

Nomad requires Docker to be installed and running on the host alongside the Nomad
agent. Nomad was developed against Docker `1.8.2`.

By default Nomad communicates with the Docker daemon using the daemon's
unix socket. Nomad will need to be able to read/write to this socket. If you do
not run Nomad as root, make sure you add the Nomad user to the Docker group so
Nomad can communicate with the Docker daemon.

For example, on ubuntu you can use the `usermod` command to add the `vagrant` user to the
`docker` group so you can run Nomad without root:

    sudo usermod -G docker -a vagrant

For the best performance and security features you should use recent versions of
the Linux Kernel and Docker daemon.

## Client Configuration

The `docker` driver has the following configuration options:

* `docker.endpoint` - Defaults to `unix:///var/run/docker.sock`. You will need
  to customize this if you use a non-standard socket (http or another location).

## Client Attributes

The `docker` driver will set the following client attributes:

* `driver.Docker` - This will be set to "1", indicating the
  driver is available.

## Resource Isolation

### CPU

Nomad limits containers' CPU based on CPU shares. CPU shares allow containers to
burst past their CPU limits. CPU limits will only be imposed when there is
contention for resources. When the host is under load your process may be
throttled to stabilize QOS depending how how many shares it has. You can see how
many CPU shares are available to your process by reading `NOMAD_CPU_LIMIT`. 1000
shares are approximately equal to 1Ghz.

Please keep the implications of CPU shares in mind when you load test workloads
on Nomad.

### Memory

Nomad limits containers' memory usage based on total virtual memory. This means
that containers scheduled by Nomad cannot use swap. This is to ensure that a
swappy process does not degrade performance for other workloads on the same host.

Since memory is not an elastic resource, you will need to make sure your
container does not exceed the amount of memory allocated to it, or it will be
terminated or crash when it tries to malloc. A process can inspect its memory
limit by reading `NOMAD_MEMORY_LIMIT`, but will need to track its own memory
usage. Memory limit is expressed in megabytes so 1024 = 1Gb.

### IO

Nomad's Docker integration does not currently provide QOS around network or
filesystem IO. These will be added in a later release.

### Security

Docker provides resource isolation by way of
[cgroups and namespaces](https://docs.docker.com/introduction/understanding-docker/#the-underlying-technology).
Containers essentially have a virtual file system all to themselves. If you need
a higher degree of isolation between processes for security or other reasons, it
is recommended to use full virtualization like [QEMU](/docs/drivers/qemu.html).