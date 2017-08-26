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
and cleaning up after containers.

## Task Configuration

```hcl
task "webservice" {
  driver = "docker"

  config {
    image = "redis:3.2"
    labels {
      group = "webservice-cache"
    }
  }
}
```

The `docker` driver supports the following configuration in the job spec.  Only
`image` is required.

* `image` - The Docker image to run. The image may include a tag or custom URL
  and should include `https://` if required. By default it will be fetched from
  Docker Hub. If the tag is omitted or equal to `latest` the driver will always
  try to pull the image. If the image to be pulled exists in a registry that
  requires authentication credentials must be provided to Nomad. Please see the
  [Authentication section](#authentication).

    ```hcl
    config {
      image = "https://hub.docker.internal/redis:3.2"
    }
    ```

* `args` - (Optional) A list of arguments to the optional `command`. If no
  `command` is specified, the arguments are passed directly to the container.
  References to environment variables or any [interpretable Nomad
  variables](/docs/runtime/interpolation.html) will be interpreted before
  launching the task. For example:

    ```hcl
    config {
      args = [
        "-bind", "${NOMAD_PORT_http}",
        "${nomad.datacenter}",
        "${MY_ENV}",
        "${meta.foo}",
      ]
    }
    ```

* `auth` - (Optional) Provide authentication for a private registry (see below).

* `auth_soft_fail` `(bool: false)` - Don't fail the task on an auth failure.
  Attempt to continue without auth.

* `command` - (Optional) The command to run when starting the container.

    ```hcl
    config {
      command = "my-command"
    }
    ```

* `dns_search_domains` - (Optional) A list of DNS search domains for the container
  to use.

* `dns_options` - (Optional) A list of DNS options for the container to use.

* `dns_servers` - (Optional) A list of DNS servers for the container to use
  (e.g. ["8.8.8.8", "8.8.4.4"]). Requires Docker v1.10 or greater.

* `extra_hosts` - (Optional) A list of hosts, given as host:IP, to be added to
  `/etc/hosts`.

* `force_pull` - (Optional) `true` or `false` (default). Always pull latest image
  instead of using existing local image. Should be set to `true` if repository tags
  are mutable.

* `hostname` - (Optional) The hostname to assign to the container. When
  launching more than one of a task (using `count`) with this option set, every
  container the task starts will have the same hostname.

* `interactive` - (Optional) `true` or `false` (default). Keep STDIN open on
  the container.

* `ipc_mode` - (Optional) The IPC mode to be used for the container. The default
  is `none` for a private IPC namespace. Other values are `host` for sharing
  the host IPC namespace or the name or id of an existing container. Note that
  it is not possible to refer to Docker containers started by Nomad since their
  names are not known in advance. Note that setting this option also requires the
  Nomad agent to be configured to allow privileged containers.

* `ipv4_address` - (Optional) The IPv4 address to be used for the container when
  using user defined networks. Requires Docker 1.13 or greater.

* `ipv6_address` - (Optional) The IPv6 address to be used for the container when
  using user defined networks. Requires Docker 1.13 or greater.

* `labels` - (Optional) A key-value map of labels to set to the containers on
  start.

    ```hcl
    config {
      labels {
        foo = "bar"
        zip = "zap"
      }
    }
    ```

* `load` - (Optional) A list of paths to image archive files. If
  this key is not specified, Nomad assumes the `image` is hosted on a repository
  and attempts to pull the image. The `artifact` blocks can be specified to
  download each of the archive files. The equivalent of `docker load -i path`
  would be run on each of the archive files.

    ```hcl
    artifact {
      source = "http://path.to/redis.tar"
    }
    config {
      load = ["redis.tar"]
      image = "redis"
    }
    ```

* `logging` - (Optional) A key-value map of Docker logging options. The default
  value is `syslog`.

    ```hcl
    config {
      logging {
        type = "fluentd"
        config {
          fluentd-address = "localhost:24224"
          tag = "your_tag"
        }
      }
    }
    ```

* `mac_address` - (Optional) The MAC address for the container to use (e.g.
  "02:68:b3:29:da:98").

* `network_aliases` - (Optional) A list of network-scoped aliases, provide a way for a
  container to be discovered by an alternate name by any other container within
  the scope of a particular network. Network-scoped alias is supported only for
  containers in user defined networks

    ```hcl
    config {
      network_mode = "user-network"
      network_aliases = [
        "${NOMAD_TASK_NAME}",
        "${NOMAD_TASK_NAME}-${NOMAD_ALLOC_INDEX}"
      ]
    }
    ```

* `network_mode` - (Optional) The network mode to be used for the container. In
  order to support userspace networking plugins in Docker 1.9 this accepts any
  value. The default is `bridge` for all operating systems but Windows, which
  defaults to `nat`. Other networking modes may not work without additional
  configuration on the host (which is outside the scope of Nomad).  Valid values
  pre-docker 1.9 are `default`, `bridge`, `host`, `none`, or `container:name`.

* `pid_mode` - (Optional) `host` or not set (default). Set to `host` to share
  the PID namespace with the host. Note that this also requires the Nomad agent
  to be configured to allow privileged containers.
  See below for more details.

* `port_map` - (Optional) A key-value map of port labels (see below).

* `privileged` - (Optional) `true` or `false` (default). Privileged mode gives
  the container access to devices on the host. Note that this also requires the
  nomad agent and docker daemon to be configured to allow privileged
  containers.

* `security_opt` - (Optional) A list of string flags to pass directly to
  [`--security-opt`](https://docs.docker.com/engine/reference/run/#security-configuration).
  For example:


    ```hcl
    config {
      security_opt = [
        "credentialspec=file://gmsaUser.json",
      ]
    }
    ```

* `shm_size` - (Optional) The size (bytes) of /dev/shm for the container.

* `SSL` - (Optional) If this is set to true, Nomad uses SSL to talk to the
  repository. The default value is `true`. **Deprecated as of 0.5.3**

* `tty` - (Optional) `true` or `false` (default). Allocate a pseudo-TTY for the
  container.

* `uts_mode` - (Optional) `host` or not set (default). Set to `host` to share
  the UTS namespace with the host. Note that this also requires the Nomad agent
  to be configured to allow privileged containers.

* `userns_mode` - (Optional) `host` or not set (default). Set to `host` to use
  the host's user namespace when user namespace remapping is enabled on the
  docker daemon.

* `volumes` - (Optional) A list of `host_path:container_path` strings to bind
  host paths to container paths. Mounting host paths outside of the allocation
  directory can be disabled on clients by setting the `docker.volumes.enabled`
  option set to false. This will limit volumes to directories that exist inside
  the allocation directory.

    ```hcl
    config {
      volumes = [
        # Use absolute paths to mount arbitrary paths on the host
        "/path/on/host:/path/in/container",

        # Use relative paths to rebind paths already in the allocation dir
        "relative/to/task:/also/in/container"
      ]
    }
    ```

* `volume_driver` - (Optional) The name of the volume driver used to mount
  volumes. Must be used along with `volumes`.
  Using a `volume_driver` also allows to use `volumes` with a named volume as
  well as absolute paths. If `docker.volumes.enabled` is false then volume
  drivers are disallowed.

    ```hcl
    config {
      volumes = [
        # Use named volume created outside nomad.
        "name-of-the-volume:/path/in/container"
      ]
      # Name of the Docker Volume Driver used by the container
      volume_driver = "flocker"
    }
    ```

* `work_dir` - (Optional) The working directory inside the container.

* `mounts` - (Optional) A list of
  [mounts](https://docs.docker.com/engine/reference/commandline/service_create/#add-bind-mounts-or-volumes)
  to be mounted into the container. Only volume type mounts are supported.

    ```hcl
    config {
      mounts = [
        {
          target = "/path/in/container"
          source = "name-of-volume"
          readonly = false
          volume_options {
            no_copy = false
            labels {
              foo = "bar"
            }
            driver_config {
              name = "flocker"
              options = {
                foo = "bar"
              }
            }
          }
        }
      ]
    }
    ```

### Container Name

Nomad creates a container after pulling an image. Containers are named
`{taskName}-{allocId}`. This is necessary in order to place more than one
container from the same task on a host (e.g. with count > 1). This also means
that each container's name is unique across the cluster.

This is not configurable.

### Authentication

If you want to pull from a private repo (for example on dockerhub or quay.io),
you will need to specify credentials in your job via:

 * the `auth` option in the task config.

 * by storing explicit repository credentials or by specifying Docker
   `credHelpers` in a file and setting the [docker.auth.config](#auth_file)
   value on the client.

 * by specifying a [docker.auth.helper](#auth_helper) on the client

The `auth` object supports the following keys:

* `username` - (Optional) The account username.

* `password` - (Optional) The account password.

* `email` - (Optional) The account email.

* `server_address` - (Optional) The server domain/IP without the protocol.
  Docker Hub is used by default.

Example task-config:

```hcl
task "example" {
  driver = "docker"

  config {
    image = "secret/service"

    auth {
      username = "dockerhub_user"
      password = "dockerhub_password"
    }
  }
}
```

Example docker-config, using two helper scripts in $PATH,
"docker-credential-ecr" and "docker-credential-vault":

```json
{
  "auths": {
    "internal.repo": { "auth": "`echo -n '<username>:<password>' | base64 -w0`" }
  },
  "credHelpers": {
      "<XYZ>.dkr.ecr.<region>.amazonaws.com": "ecr-login"
  },
  "credsStore": "secretservice"
}
```


!> **Be Careful!** At this time these credentials are stored in Nomad in plain
text. Secrets management will be added in a later release.

## Networking

Docker supports a variety of networking configurations, including using host
interfaces, SDNs, etc. Nomad uses `bridged` networking by default, like Docker.

You can specify other networking options, including custom networking plugins
in Docker 1.9. **You may need to perform additional configuration on the host
in order to make these work.** This additional configuration is outside the
scope of Nomad.

### Allocating Ports

You can allocate ports to your task using the port syntax described on the
[networking page](/docs/job-specification/network.html). Here is a recap:

```hcl
task "example" {
  driver = "docker"

  resources {
    network {
      port "http" {}
      port "https" {}
    }
  }
}
```

### Forwarding and Exposing Ports

A Docker container typically specifies which port a service will listen on by
specifying the `EXPOSE` directive in the `Dockerfile`.

Because dynamic ports will not match the ports exposed in your Dockerfile,
Nomad will automatically expose all of the ports it allocates to your
container.

These ports will be identified via environment variables. For example:

```hcl
port "http" {}
```

If Nomad allocates port `23332` to your task for `http`, `23332` will be
automatically exposed and forwarded to your container, and the driver will set
an environment variable `NOMAD_PORT_http` with the value `23332` that you can
read inside your container.

This provides an easy way to use the `host` networking option for better
performance.

### Using the Port Map

If you prefer to use the traditional port-mapping method, you can specify the
`port_map` option in your job specification. It looks like this:

```hcl
task "example" {
  driver = "docker"

  config {
    image = "redis"

    port_map {
      redis = 6379
    }
  }

  resources {
    network {
      mbits = 20
      port "redis" {}
    }
  }
}
```

If Nomad allocates port `23332` to your task, the Docker driver will
automatically setup the port mapping from `23332` on the host to `6379` in your
container, so it will just work!

Note that by default this only works with `bridged` networking mode. It may
also work with custom networking plugins which implement the same API for
expose and port forwarding.

### Advertising Container IPs

*New in Nomad 0.6.*

When using network plugins like `weave` that assign containers a routable IP
address, that address will automatically be used in any `service`
advertisements for the task. You may override what address is advertised by
using the `address_mode` parameter on a `service`. See
[service](/docs/job-specification/service.html) for details.

### Networking Protocols

The Docker driver configures ports on both the `tcp` and `udp` protocols.

This is not configurable.

### Other Networking Modes

Some networking modes like `container` or `none` will require coordination
outside of Nomad. First-class support for these options may be improved later
through Nomad plugins or dynamic job configuration.

## Client Requirements

Nomad requires Docker to be installed and running on the host alongside the
Nomad agent. Nomad was developed against Docker `1.8.2` and `1.9`.

By default Nomad communicates with the Docker daemon using the daemon's Unix
socket. Nomad will need to be able to read/write to this socket. If you do not
run Nomad as root, make sure you add the Nomad user to the Docker group so
Nomad can communicate with the Docker daemon.

For example, on Ubuntu you can use the `usermod` command to add the `vagrant`
user to the `docker` group so you can run Nomad without root:

    sudo usermod -G docker -a vagrant

For the best performance and security features you should use recent versions
of the Linux Kernel and Docker daemon.

## Client Configuration

The `docker` driver has the following [client configuration
options](/docs/agent/configuration/client.html#options):

* `docker.endpoint` - Defaults to `unix:///var/run/docker.sock`. You will need
  to customize this if you use a non-standard socket (HTTP or another
  location).

* `docker.auth.config` <a id="auth_file"></a>- Allows an operator to specify a
  JSON file which is in the dockercfg format containing authentication
  information for a private registry, from either (in order) `auths`,
  `credHelpers` or `credsStore`.

* `docker.auth.helper` <a id="auth_helper"></a>- Allows an operator to specify
  a [credsStore](https://docs.docker.com/engine/reference/commandline/login/#credential-helper-protocol)
  -like script on $PATH to lookup authentication information from external
  sources.

* `docker.tls.cert` - Path to the server's certificate file (`.pem`). Specify
  this along with `docker.tls.key` and `docker.tls.ca` to use a TLS client to
  connect to the docker daemon. `docker.endpoint` must also be specified or
  this setting will be ignored.

* `docker.tls.key` - Path to the client's private key (`.pem`). Specify this
  along with `docker.tls.cert` and `docker.tls.ca` to use a TLS client to
  connect to the docker daemon. `docker.endpoint` must also be specified or
  this setting will be ignored.

* `docker.tls.ca` - Path to the server's CA file (`.pem`). Specify this along
  with `docker.tls.cert` and `docker.tls.key` to use a TLS client to connect to
  the docker daemon. `docker.endpoint` must also be specified or this setting
  will be ignored.

* `docker.cleanup.image` Defaults to `true`. Changing this to `false` will
  prevent Nomad from removing images from stopped tasks.

* `docker.cleanup.image.delay` A time duration that defaults to `3m`. The delay
  controls how long Nomad will wait between an image being unused and deleting
  it. If a tasks is received that uses the same image within the delay, the
  image will be reused.

* `docker.volumes.enabled`: Defaults to `true`. Allows tasks to bind host paths
  (`volumes`) inside their container and use volume drivers (`volume_driver`).
  Binding relative paths is always allowed and will be resolved relative to the
  allocation's directory.

* `docker.volumes.selinuxlabel`: Allows the operator to set a SELinux
  label to the allocation and task local bind-mounts to containers. If used
  with `docker.volumes.enabled` set to false, the labels will still be applied
  to the standard binds in the container.

* `docker.privileged.enabled` Defaults to `false`. Changing this to `true` will
  allow containers to use `privileged` mode, which gives the containers full
  access to the host's devices. Note that you must set a similar setting on the
  Docker daemon for this to work.

Note: When testing or using the `-dev` flag you can use `DOCKER_HOST`,
`DOCKER_TLS_VERIFY`, and `DOCKER_CERT_PATH` to customize Nomad's behavior. If
`docker.endpoint` is set Nomad will **only** read client configuration from the
config file.

An example is given below:

```hcl
client {
  options {
    "docker.cleanup.image" = "false"
  }
}
```

## Client Attributes

The `docker` driver will set the following client attributes:

* `driver.docker` - This will be set to "1", indicating the driver is
  available.
* `driver.docker.bridge_ip` - The IP of the Docker bridge network if one
  exists.
* `driver.docker.version` - This will be set to version of the docker server.

Here is an example of using these properties in a job file:

```hcl
job "docs" {
  # Require docker version higher than 1.2.
  constraint {
    attribute = "${driver.docker.version}"
    operator  = ">"
    version   = "1.2"
  }
}
```

## Resource Isolation

### CPU

Nomad limits containers' CPU based on CPU shares. CPU shares allow containers
to burst past their CPU limits. CPU limits will only be imposed when there is
contention for resources. When the host is under load your process may be
throttled to stabilize QoS depending on how many shares it has. You can see how
many CPU shares are available to your process by reading `NOMAD_CPU_LIMIT`.
1000 shares are approximately equal to 1 GHz.

Please keep the implications of CPU shares in mind when you load test workloads
on Nomad.

### Memory

Nomad limits containers' memory usage based on total virtual memory. This means
that containers scheduled by Nomad cannot use swap. This is to ensure that a
swappy process does not degrade performance for other workloads on the same
host.

Since memory is not an elastic resource, you will need to make sure your
container does not exceed the amount of memory allocated to it, or it will be
terminated or crash when it tries to malloc. A process can inspect its memory
limit by reading `NOMAD_MEMORY_LIMIT`, but will need to track its own memory
usage. Memory limit is expressed in megabytes so 1024 = 1 GB.

### IO

Nomad's Docker integration does not currently provide QoS around network or
filesystem IO. These will be added in a later release.

### Security

Docker provides resource isolation by way of
[cgroups and namespaces](https://docs.docker.com/introduction/understanding-docker/#the-underlying-technology).
Containers essentially have a virtual file system all to themselves. If you
need a higher degree of isolation between processes for security or other
reasons, it is recommended to use full virtualization like
[QEMU](/docs/drivers/qemu.html).

## Docker For Mac Caveats

Docker For Mac runs docker inside a small VM and then allows access to parts of
the host filesystem into that VM. At present, nomad uses a syslog server bound to
a Unix socket within a path that both the host and the VM can access to forward
log messages back to nomad. But at present, Docker For Mac does not work for
Unix domain sockets (https://github.com/docker/for-mac/issues/483) in one of
these shared paths.

As a result, using nomad with the docker driver on OS X/macOS will work, but no
logs will be available to nomad. Users must use the native docker facilities to
examine the logs of any jobs running under docker.

In the future, we will resolve this issue, one way or another.
