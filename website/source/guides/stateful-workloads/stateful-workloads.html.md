---
layout: "guides"
page_title: "Stateful Workloads"
sidebar_current: "guides-stateful-workloads"
description: |-
  It is possible to deploy and consume stateful workloads in Nomad. Nomad can
  integrate with various storage solutions such as Portworx and REX-Ray.
---

# Stateful Workloads

The Docker task driver's support for [volumes][docker-volumes] enables Nomad to
integrate with software-defined storage (SDS) solutions like
[Portworx][portworx] to support stateful workloads. Please keep in mind that
Nomad does not actually manage storage pools or replication as these tasks are
delegated to the SDS providers. Please assess all factors and risks when
utilizing such providers to run stateful workloads (such as your production
database).

Nomad will be adding first class features in the near future that will allow a
user to mount local or remote storage volumes into task environments in a
consistent way across all task drivers and storage providers.

Please refer to the specific documentation links below or in the sidebar for
more detailed information about using specific storage integrations.

- [Portworx](/guides/stateful-workloads/portworx.html)

[docker-volumes]: /docs/drivers/docker.html#volumes
[portworx]: https://docs.portworx.com/install-with-other/nomad