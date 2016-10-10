---
layout: "intro"
page_title: "Nomad vs. Docker Swarm"
sidebar_current: "vs-other-swarm"
description: |-
  Comparison between Nomad and Docker Swarm
---

# Nomad vs. Docker Swarm

Docker Swarm is the native orchestration solution for Docker. It provides
an API compatible with the Docker Remote API, and allows containers to
be scheduled across many machines.

Nomad differs in many ways with Docker Swarm, most obviously Docker Swarm
can only be used to run Docker containers, while Nomad is more general purpose.
Nomad supports virtualized, containerized and standalone applications, including Docker.
Nomad is designed with extensible drivers and support will be extended to all
common drivers.

Docker Swarm provides API compatibility with their remote API, which focuses
on the container abstraction, but also higher-level abstractions like services and tasks,
providing a self-contained unique experience for Devs and Ops as well.
Nomad also uses a higher-level abstraction of jobs. Jobs contain task groups, 
which are sets of tasks. This allows more complex
applications to be expressed and easily managed without reasoning about the
individual containers that compose the application. 

Not surprisingly, both Nomad and Docker Swarm share modern design principles.
No external dependencies for coordination or storage,
distributed, highly available, multi-datacenter support, multi-region configurations.

As Docker Swarm, Nomad is distributed and highly available by default.
No need to use external systems for coordination to support replication.
Plus, you get TLS-level security out-of-the-box in Docker Swarm.
When replication is enabled, Swarm uses an active/standby model,
meaning the other servers cannot be used to make scheduling decisions.
Swarm also did not support multiple failure isolation regions or federation,
until the new filtering ability with labels.
