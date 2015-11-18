---
layout: "docs"
page_title: "Service Discovery in Nomad"
sidebar_current: "docs-service-discovery"
description: |-
  Learn how to add service discovery to jobs
  ---

# Service Discovery

Nomad enables dynamic scheduling on compute resources to run services at scale
on compute nodes. Size of an application cluster varies depending
on volume of traffic, health of services, etc thereby the network topology of
services are constanly changing. This introduces the challenge of discovery of services, 
Nomad solves this problem by integrating with [Consul](https://consul.io) to provide service
discovery and health checks of services.

Operators have to run Consul agents on a Nomad compute node. Nomad also makes
running an application like Consul agent on every single node in a data centre
simple by providing system jobs. Nomad connects to Consul on it's default http
port but operators can override it.

## Configuration

* `consul.address`: This is a Nomad client config which can be used to override
  the default Consul Agent HTTP port that Nomad uses to connect to Consul. The
  default for this is "127.0.0.1:8500"

## Service Deginition Syntax

The service blocks in a Task definition defines a service which Nomad will
register with Consul. Multiple Service blocks are allowed in a Task definition,
which makes it usable for cases when an application listens on multiple ports
each exposing a specific service.

### Example 

A brief example of a service definition in a Task
```
group "database" {
    task "mysql" {
        driver = "docker"
        service {
            tags = ["master", "mysql"]
            port = "db"
            check {
                type = "tcp"
                delay = "10s"
                timeout = "2s"
            }
        }
        reources {
            cpu = 500
            memory = 1024
            network {
                mbits = 10
                port "db" {
                }
            }
        }
    }
}

```

* `name`: Nomad automatically determines the name of a Task. By default the name
  of a service is $(job-name)-$(task-group)-$(task-name). Users can explictly
  name the service by specifying this option. If multiple services are defined
  for a Task then all the services have to be explictly named. Nomad will add
  the prefix $(job-name)-${task-group}-${task-name} prefix to each user defined
  names.

* `tags`: A list of tags associated with this Service.

* `port`: The port indicates the port associated with the Service. Users are
  reruired to specify a valid port label here which they have defined in the
  resources block. This could be a label to either a dynamic or a static port. If
  an incorrect port label is specified, Nomad doesn't register the service with
  Consul.

* `check`: A check block defines a health check associated with the service.
  Mutliple check blocks are allowed for a service. Nomad currently supports only
  the `http` and `tcp` Consul Checks.

### Check Syntax 
* `type`: This indicates the check types supported by Nomad. Valid options are
  currently `http` and `tcp`. In the future Nomad will add support for more
  Consul checks.

* `delay`: This indicates the frequency of the health checks that Consul with
  perform.

* `timeout`: This indicates how long Consul will wait for a health check query
  to succeed.

* `path`: The path of the http endpoint which Consul will query to query the
  health of a service if the type of the check is `http`. Nomad will add the ip
  of the service and the port, users are only required to add the relative url
  of the health check endpoint.

* `protocol`: This indicates the protocol for the http checks. Valid options are
  `http` and `https`.


## Assumptions 

* Consul 0.6 is needed for using the TCP checks.

* The Service Discovery feature in Nomad depends on Operators making sure that the
  Nomad client can reach the consul agent.

* Nomad assumes that it controls the life cycle of all the externally
  discoverable services running on a host.

* Tasks running inside Nomad also needs to reach out to the Consul agent if they
  want to use any Consul APIs. Ex: A task running inside a docker container in
  the bridge mode won't be able to talk to a Consul Agent running on the
  loopback interface of the host since the container in the bridge mode has it's
  own network interface and doesn't see interfaces on the global network
  namespaces. There are a couple of ways to solve this, one way is to run the
  container in the host networking mode, or make the Consul agent listen on an
  interface on the network namespace of the container.






