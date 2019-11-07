---
layout: "guides"
page_title: "Installing Nomad for Production"
sidebar_current: "guides-install-production"
description: |-
  Learn how to install Nomad for Production.
---

#Installing Nomad for Production

This section covers how to install Nomad for production.  

There are multiple steps to cover for a successful Nomad deployment:

##Installing Nomad
This page lists the two primary methods to installing Nomad and how to verify a successful installation.

Please refer to [Installing Nomad](/guides/install/index.html) sub-section.

##Hardware Requirements
This page details the recommended machine resources (instances), port requirements, and network topology for Nomad.

Please refer to [Hardware Requirements](/guides/install/production/requirements.html) sub-section.

##Setting Nodes with Nomad Agent
These pages explain the Nomad agent process and how to set the server and client nodes in the cluster.

Please refer to [Set Server & Client Nodes](/guides/install/production/nomad-agent.html) and [Nomad Agent documentation](/docs/commands/agent.html) pages.

##Reference Architecture
This document provides recommended practices and a reference architecture for HashiCorp Nomad production deployments. This reference architecture conveys a general architecture that should be adapted to accommodate the specific needs of each implementation.

Please refer to [Reference Architecture](/guides/install/production/reference-architecture.html) sub-section.

##Install Guide Based on Reference Architecture
This guide provides an end-to-end walkthrough of the steps required to install a single production-ready Nomad cluster as defined in the Reference Architecture section.

Please refer to [Reference Install Guide](/guides/install/production/deployment-guide.html) sub-section.
