---
layout: "guides"
page_title: "Stateful Workloads"
sidebar_current: "guides-stateful-workloads"
description: |-
  It is possible to deploy and consume stateful workloads in Nomad. Nomad can
  integrate with various storage solutions such as Portworx and REX-Ray.
---

# Stateful Workloads

Nomad provides the opportunity to integrate with various storage solutions such
as [Portworx][portworx] and [REX-Ray][rexray]. Please keep in mind that Nomad
does not actually manage storage pools or replication as these tasks are
delegated to the integration providers. Please assess all factors and risks when
utilizing such providers to run stateful workloads (such as your production
database).

In upcoming releases, Nomad will provide features to consistently mount local or
remote volumes into task environments across task drivers and storage providers.

Please refer to the specific documentation links below or in the sidebar for
more detailed information about using specific storage integrations.

- [Portworx](/guides/stateful-workloads/portworx.html)

[portworx]: https://docs.portworx.com/install-with-other/nomad
[rexray]: https://github.com/rexray/rexray
