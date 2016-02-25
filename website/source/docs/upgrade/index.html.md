---
layout: "docs"
page_title: "Upgrade Nomad"
sidebar_current: "docs-upgrade-upgrading"
description: |-
  Learn how to upgrade Nomad.
---

# Upgrading Nomad

Both Nomad Clients and Servers are meant to be long-running processes that
maintain communication with each other. Nomad Servers maintain quorum with other
Servers and Clients are in constant communication with Servers. As such, care
should be taken to properly upgrade Nomad to ensure minimal service disruption.

This page documents how to upgrade Nomad when a new version is released.

## Standard Upgrades

For upgrades we strive to ensure backwards compatibility. For most upgrades, the
process is simple. Assuming the current version of Nomad is A, and version B is
released.

1. On each server, install version B of Nomad.

2. Shut down version A, restart with version B on one server at a time.

    3. You can run `nomad server-members` to ensure that all servers are
       clustered and running the version B.

4. Once all the servers are upgraded, begin a rollout of clients following
   the same process.

   5. Done! You are now running the latest Nomad version. You can verify all
      Clients joined by running `nomad node-status` and checking all the clients
      are in a `ready` state.
