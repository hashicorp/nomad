---
layout: "docs"
page_title: "Service Discovery"
sidebar_current: "docs-discovery"
description: >
  Service Discovery integration in Nomad enables automatic discovery of
  running tasks and services.
---

# Service Discovery

Nomad provides fast and extensible abilities for scheduling work onto machines
in the cluster. The next logical step in using the application is to query for
where the service can be found. Typically this involves an IP address and port
numbers to connect to the service. Nomad itself does not implement or expose
service discovery primitives, but provides seamless integration with existing
tools which solve this problem elegantly.

Service discovery integration is optional. The supported providers are detailed
in this section and can be found in the navigation bar to the left.

## Exposing tasks with service discovery

In the context of service discovery, a task may be simple or complex, depending
on its networking requirements. A simple service with static port bindings may
need only a single entry in a discovery back-end. However, tasks which leverage
Nomad's [dynamic ports](/docs/jobspec/index.html#dynamic_ports) must advertise
additional information to convey port details to consumers.

The following items will be registered in each enabled discovery back-end:

* Basic task location. This is always registered, but contains no information
  about network ports. This can be used to locate the node a task is running on
  if the port is known beforehand by consumers.

* Dynamic ports, by label. If dynamic ports are used within a task, then each
  port label will be registered with the enabled discovery back-ends. This
  registration contains the port number allocated at runtime to the
  task.

Each discovery back-end will handle service registration differently. Refer to
the specific providers documentation for details.
