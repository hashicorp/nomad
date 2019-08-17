---
layout: "guides"
page_title: "Inspecting the Cluster"
sidebar_current: "guides-web-ui-inspecting-the-cluster"
description: |-
  Learn how to inspect the state of the cluster from the Web UI.
---

# Inspecting the Cluster

The Web UI can be a powerful tool for keeping tabs on the state of the Nomad cluster from an
operator's perspective. The Web UI will show all client nodes, show driver health for client nodes,
driver status, resource utilization, allocations by client node, and more.

## Reviewing All Clients

From any page, the Clients List page can be accessed from the left-hand navbar. On narrow screens
this navbar may need to opened from the top-right menu button. Here you see every client in the
cluster. The table of clients is searchable, sortable, and filterable. Each client row in the table
show basic information, such as the Node ID, name, state, address, datacenter, and how many
allocations are running in it.

This view will also live-update as the state of client nodes change.

~> Screenshot (Clients list)

## Filtering Clients

If your Nomad cluster has many client nodes, it can be useful to filter the list of all client nodes
down to only those matching certain facets. The Web UI has three facets you can filter by:

1. **Class:** The node of the client, including a dynamically generated list based on the node class
   of each client node in the cluster.
2. **State:** The state of the cluster, including Initializing, Ready, Down, Ineligible, and
   Draining.
3. **Datacenter:** The datacenter the client node is in, including a dynamicaly generated list based
   on all the datacenters in the cluster.

~> Screenshot (Zoom in on client filters, with one open)

## Inspecting an Individual Client

From the Clients List page, clicking a client node in the table will direct you to the Client Detail
page for the client node. This page includes all information about the client node is live-updated
to always present up-to-date information.

~> Screenshot (Client Detail)

### Resource Utilization

Nomad as APIs for reading point-in-time resource utilization metrics for client nodes. The Web UI
uses these metrics to create time-series graphics for the current session.

When viewing a client node, resource utilization will automatically start logging.

~> Screenshot (resource utilization)

### Allocations

Allocations belong to jobs and are placed on client nodes. The Client Detail page will list all
allocations for a client node, including completed, failed, and lost allocations, until they are
garbage-collected.

This is presented in a searchable table which can additionally be filtered to only preempted
allocations.

~> Screenshot (client node alloc table)

### Client Events

Client nodes will also emit events on meaningful state changes, such as when the node becomes ready
for scheduling or when a driver becomes unhealthy.

~> Screenshot (client node events)

### Driver Status

Task drivers are additional services running on a client node. Nomad will fingerprint and
communicate with the task driver to determine if the driver is available and healthy. This
information is reported through the Web UI on the Client Detail page.

~> Screenshot (driver status)

### Attributes

In order to allow job authors to constrain the placement of their jobs, Nomad fingerprints the
hardware of the node the client agent is running on. This is a deeply nested document of properties
that the Web UI presents in a scannable way.

In addition to the hardware attributes, Nomad operators can annotate
[a client node with metadata](/docs/configuration/client.html#meta) as part of the client configuration. This metadata
is also presented on the Client Detail page.

~> Screenshot (client attributes)

## When a Node is Draining

A routine part of maintaining a Nomad cluster is draining nodes of allocations. This can be in
preparation of performing operating system upgrades or decommissioning an old node in favor of a new
VM.

Drains are [performed from the CLI](/guides/operations/node-draining.html) but the status of a drain
can be seen from the Web UI. A client node will state if it is actively draining or ineligible for
scheduling.

Since drains can be configured in a variety of ways, the Client Detail page will also present the
details of how the drain is performed.

~> Screenshot (client detail drain)

## Reviewing All Servers

Where client nodes are used to run your jobs, server nodes are used to run Nomad and maintain
availability. From any page, the Servers List page can be accessed from the left-hand navbar.

Here you can see every server node. This will be a small list—typically three, but maybe five or
seven.

~> Screenshot (servers list)

## Inspecting an Individual Server

Clicking a server node on the Servers List will expand the tags table for the server node.

~> Screenshot (server node)

## Access Control

Depending on the size of your team and the details of you Nomad deployment, you may wish to control
which features different internal users have access to. This includes limiting who has access to see
and manage client nodes and see and manage server nodes.

Nomad has an access contro list system for doing just that.

By default, all features—read and write—are available to all users of the Web UI. Check out the
[Securing the Web UI with ACLs](/guides/web-ui/securing.html) guide to learn how to prevent
anonymous users from having write permissions as well as how to continue to use Web UI write
features as a privileged user.
