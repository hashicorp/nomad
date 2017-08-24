---
layout: "guides"
page_title: "ACLs"
sidebar_current: "guides-acl"
description: |-
  Don't panic! This is a critical first step. Depending on your deployment
  configuration, it may take only a single server failure for cluster
  unavailability. Recovery requires an operator to intervene, but recovery is
  straightforward.
---

# ACL System

Nomad provides an optional Access Control List (ACL) system which can be used to control access to data and APIs. The ACL is [Capability-based](https://en.wikipedia.org/wiki/Capability-based_security), relying on tokens which are associated with policies to determine which fine grained rules can be applied. It is very similar to [AWS IAM](https://aws.amazon.com/iam/) in many ways.

# ACL System Overview

The ACL system is designed to be easy to use, fast to enforce, and flexible to new policies, all while providing administrative insight. At the highest level, there are three major components to the ACL system:

![ACL Overview](/assets/images/acl.jpg)

 * **ACL Policies**. No permissions are granted by default, making Nomad a default-deny or whitelist system. Policies allow a set of capabilities or actions to be granted or whitelisted. For example, a "readonly" policy might only grant the ability to list and inspect running jobs, but not to submit new ones.

 * **ACL Tokens**. Requests to Nomad are authenticated by using bearer token. Each ACL token has a public Accessor ID which is used to name a token, and a Secret ID which is used to make requests to Nomad. The Secret ID is provided using a request header (`X-Nomad-Token`) and is used to authenticate the caller. Token are either `management` or `client` types. The `management` tokens are effectively "root" in the system, and can perform any operation. The `client` tokens are associated with one or more ACL policies which grant specific capabilities.

 * **Capabilities**. Capabilties are the set of actions that can be performed. This includes listing jobs, submitting jobs, querying nodes, etc. A `management` token is granted all capabilities, while `client` tokens are granted specific capabilties via ACL Policies. The full set of capabilities is discussed below in the rule specifications.

### ACL Policies

An ACL policy is a named set of rules. Each policy must have a unique name, an optional description, and a rule set.
A client ACL token can be associated with multiple policies, and a request is allowed if _any_ of the associated policies grant the capability.
Management tokens cannot be associated with policies because they are granted all capabilities.

The special `anonymous` policy can be defined to grant capabilities to requests which are made anonymously. If a request is made to Nomad without the `X-Nomad-Token` header specified, then it is an anonymous request. This can be used to allow anonymous users to list jobs and view their status, while requiring authenticated requests to submit new jobs or modify existing jobs. By default, there is no `anonymous` policy set meaning all anonymous requests are denied.

### ACL Tokens

ACL tokens are used to authenticate requests and determine if the caller is authorized to perform an action. Each ACL token has a public Accessor ID which is used to identify the token, a Secret ID which is used to make requests to Nomad, and an optional human readable name. All `client` type tokens are associated with one or more policies, and can perform an action if any associated policy allows it. Tokens can be associated with policies which do not exist, which are the equivalent of granting no capabilities. The `management` type tokens cannot be associated with policies, but can perform any action.

When ACL tokens are created, they can be optionally marked as `Global`. This causes them to be created in the authoritative region and replicated to all other regions. Otherwise, tokens are created locally in the region the request was made and not replicated. Local tokens cannot be used for cross-region requests since they are not replicated between regions.

### Capabilities and Scope

The following table summarizes the ACL Rules that are available for constructing policy rules:

| Policy     | Scope                                        |
| ---------- | -------------------------------------------- |
| namespace  | Job related operations by namespace          |
| agent      | Utility operations in the Agent API          |
| node       | Node-level catalog operations                |
| operator   | Cluster-level operations in the Operator API |

Constructing rules from these policies is covered in detail in the Rule Specification section below.

### Multi-Region Configuration

Nomad supports multi-datacenter and multi-region configurations. A single region is able to service multiple datacenters, and all servers in a region replicate their state between each other. In a multi-region configuration, there is a set of servers per region. Each region operates independently and is loosely coupled to allow jobs to be scheduled in any region and requests to flow transparently to the correct region.

When ACLs are enabled, Nomad depends on an "authoritative region" to act as a single source of truth for ACL policies and global ACL tokens. The authoritative region is configured in the `server` stanza of agents, and all regions must share a single a single authoritative source. Any ACL policies or global ACL tokens are created in the authoritative region first. All other regions replicate ACL policies and global ACL tokens to act as local mirrors. This allows policies to be administered centrally, and for enforcement to be local to each region for low latency.

Global ACL tokens are used to allow cross region requests. Standard ACL tokens are created in a single target region and not replicated. This means if a request takes place between regions, global tokens must be used so that both regions will have the token registered.

# Configuring ACLs

# Bootstrapping ACLs

# Rule Specification

# Advanced Topics

