---
layout: "guides"
page_title: "ACLs"
sidebar_current: "guides-acl"
description: |-
  Nomad provides an optional Access Control List (ACL) system which can be used to control
  access to data and APIs. The ACL is Capability-based, relying on tokens which are
  associated with policies to determine which fine grained rules can be applied.
---

# ACL System

Nomad provides an optional Access Control List (ACL) system which can be used to control access to data and APIs. The ACL is [Capability-based](https://en.wikipedia.org/wiki/Capability-based_security), relying on tokens which are associated with policies to determine which fine grained rules can be applied. Nomad's capability based ACL system is very similar to the design of [AWS IAM](https://aws.amazon.com/iam/).

# ACL System Overview

The ACL system is designed to be easy to use and fast to enforce while providing administrative insight. At the highest level, there are three major components to the ACL system:

![ACL Overview](/assets/images/acl.jpg)

 * **ACL Policies**. No permissions are granted by default, making Nomad a default-deny or whitelist system. Policies allow a set of capabilities or actions to be granted or whitelisted. For example, a "readonly" policy might only grant the ability to list and inspect running jobs, but not to submit new ones.

 * **ACL Tokens**. Requests to Nomad are authenticated by using bearer token. Each ACL token has a public Accessor ID which is used to name a token, and a Secret ID which is used to make requests to Nomad. The Secret ID is provided using a request header (`X-Nomad-Token`) and is used to authenticate the caller. Tokens are either `management` or `client` types. The `management` tokens are effectively "root" in the system, and can perform any operation. The `client` tokens are associated with one or more ACL policies which grant specific capabilities.

 * **Capabilities**. Capabilties are the set of actions that can be performed. This includes listing jobs, submitting jobs, querying nodes, etc. A `management` token is granted all capabilities, while `client` tokens are granted specific capabilties via ACL Policies. The full set of capabilities is discussed below in the rule specifications.

### ACL Policies

An ACL policy is a named set of rules. Each policy must have a unique name, an optional description, and a rule set.
A client ACL token can be associated with multiple policies, and a request is allowed if _any_ of the associated policies grant the capability.
Management tokens cannot be associated with policies because they are granted all capabilities.

The special `anonymous` policy can be defined to grant capabilities to requests which are made anonymously. An anonymous request is a request made to Nomad without the `X-Nomad-Token` header specified. This can be used to allow anonymous users to list jobs and view their status, while requiring authenticated requests to submit new jobs or modify existing jobs. By default, there is no `anonymous` policy set meaning all anonymous requests are denied.

### ACL Tokens

ACL tokens are used to authenticate requests and determine if the caller is authorized to perform an action. Each ACL token has a public Accessor ID which is used to identify the token, a Secret ID which is used to make requests to Nomad, and an optional human readable name. All `client` type tokens are associated with one or more policies, and can perform an action if any associated policy allows it. Tokens can be associated with policies which do not exist, which are the equivalent of granting no capabilities. The `management` type tokens cannot be associated with policies, but can perform any action.

When ACL tokens are created, they can be optionally marked as `Global`. This causes them to be created in the authoritative region and replicated to all other regions. Otherwise, tokens are created locally in the region the request was made and not replicated. Local tokens cannot be used for cross-region requests since they are not replicated between regions.

### Capabilities and Scope

The following table summarizes the ACL Rules that are available for constructing policy rules:

| Policy     | Scope                                        |
| ---------- | -------------------------------------------- |
| [namespace](#namespace-rules) | Job related operations by namespace          |
| [agent](#agent-rules) | Utility operations in the Agent API          |
| [node](#node-rules) | Node-level catalog operations                |
| [operator](#operator-rules) | Cluster-level operations in the Operator API |
| [quota](#quota-rules) | Quota specification related operations |

Constructing rules from these policies is covered in detail in the Rule Specification section below.

### Multi-Region Configuration

Nomad supports multi-datacenter and multi-region configurations. A single region is able to service multiple datacenters, and all servers in a region replicate their state between each other. In a multi-region configuration, there is a set of servers per region. Each region operates independently and is loosely coupled to allow jobs to be scheduled in any region and requests to flow transparently to the correct region.

When ACLs are enabled, Nomad depends on an "authoritative region" to act as a single source of truth for ACL policies and global ACL tokens. The authoritative region is configured in the [`server` stanza](/docs/agent/configuration/server.html) of agents, and all regions must share a single a single authoritative source. Any ACL policies or global ACL tokens are created in the authoritative region first. All other regions replicate ACL policies and global ACL tokens to act as local mirrors. This allows policies to be administered centrally, and for enforcement to be local to each region for low latency.

Global ACL tokens are used to allow cross region requests. Standard ACL tokens are created in a single target region and not replicated. This means if a request takes place between regions, global tokens must be used so that both regions will have the token registered.

# Configuring ACLs

ACLs are not enabled by default, and must be enabled. Clients and Servers need to set `enabled` in the [`acl` stanza](/docs/agent/configuration/acl.html). This enables the [ACL Policy](/api/acl-policies.html) and [ACL Token](/api/acl-tokens.html) APIs, as well as endpoint enforcement.

For multi-region configurations, all servers must be configured to use a single [authoritative region](/docs/agent/configuration/server.html#authoritative_region). The authoritative region is responsible for managing ACL policies and global tokens. Servers in other regions will replicate policies and global tokens to act as a mirror, and must have their [`replication_token`](/docs/agent/configuration/acl.html#replication_token) configured.

# Bootstrapping ACLs

Bootstrapping ACLs on a new cluster requires a few steps, outlined below:

### Enable ACLs on Nomad Servers

The APIs needed to manage policies and tokens are not enabled until ACLs are enabled. To begin, we need to enable the ACLs on the servers. If a multi-region setup is used, the authoritiative region should be enabled first. For each server:

1. Set `enabled = true` in the [`acl` stanza](/docs/agent/configuration/acl.html#enabled).
1. Set `authoritative_region` in the [`server` stanza](/docs/agent/configuration/server.html#authoritative_region).
1. For servers outside the authoritative region, set `replication_token` in the [`acl` stanza](/docs/agent/configuration/acl.html#replication_token). Replication tokens should be `management` type tokens which are either created in the authoritative region, or created as Global tokens.
1. Restart the Nomad server to pick up the new configuration.

Please take care to restart the servers one at a time, and ensure each server has joined and is operating correctly before restarting another.

### Generate the initial token

Once the ACL system is enabled, we need to generate our initial token. This first token is used to bootstrap the system and care should be taken not to lose it. Once the ACL system is enabled, we use the [Bootstrap CLI](/docs/commands/acl/bootstrap.html):

```text
$ nomad acl bootstrap
Accessor ID  = 5b7fd453-d3f7-6814-81dc-fcfe6daedea5
Secret ID    = 9184ec35-65d4-9258-61e3-0c066d0a45c5
Name         = Bootstrap Token
Type         = management
Global       = true
Policies     = n/a
Create Time  = 2017-09-11 17:38:10.999089612 +0000 UTC
Create Index = 7
Modify Index = 7
```

Once the initial bootstrap is performed, it cannot be performed again unless [reset](#reseting-acl-bootstrap). Make sure to save this AccessorID and SecretID.
The bootstrap token is a `management` type token, meaning it can perform any operation. It should be used to setup the ACL policies and create additional ACL tokens. The bootstrap token can be deleted and is like any other token, so care should be taken to not revoke all management tokens.

### Enable ACLs on Nomad Clients

To enforce client endpoints, we need to enable ACLs on clients as well. This is simpler than servers, and we just need to set `enabled = true` in the [`acl` stanza](/docs/agent/configuration/acl.html). Once configured, we need to restart the client for the change.


### Set an Anonymous Policy (Optional)

The ACL system uses a whitelist or default-deny model. This means by default no permissions are granted.
For clients making requests without ACL tokens, we may want to grant some basic level of access. This is done by setting rules
on the special "anonymous" policy. This policy is applied to any requests made without a token.

To permit anonymous users to read, we can setup the following policy:

```text
# Store our token secret ID
$ export NOMAD_TOKEN="BOOTSTRAP_SECRET_ID"

# Write out the payload
$ cat > payload.json <<EOF
{
    "Name": "anonymous",
    "Description": "Allow read-only access for anonymous requests",
    "Rules": "
        namespace \"default\" {
            policy = \"read\"
        }
        agent {
            policy = \"read\"
        }
        node {
            policy = \"read\"
        }
    "
}
EOF

# Install the policy
$ curl --request POST \
    --data @payload.json \
    -H "X-Nomad-Token: $NOMAD_TOKEN" \
    https://nomad.rocks/v1/acl/policy/anonymous

# Verify anonymous request works
$ curl https://nomad.rocks/v1/jobs
```

# Rule Specification

A core part of the ACL system is the rule language which is used to describe the policy that must be enforced.
We make use of the [HashiCorp Configuration Language (HCL)](https://github.com/hashicorp/hcl/) to specify rules.
This language is human readable and interoperable with JSON making it easy to machine-generate. Policies can contain any number of rules.

Policies typically have several dispositions:

* `read`: allow the resource to be read but not modified
* `write`: allow the resource to be read and modified
* `deny`: do not allow the resource to be read or modified. Deny takes precedence when multiple policies are associated with a token.

Specification in the HCL format looks like:

```text
# Allow read only access to the default namespace
namespace "default" {
    policy = "read"
}

# Allow writing to the `foo` namespace
namespace "foo" {
    policy = "write"
}

agent {
    policy = "read"
}

node {
    policy = "read"
}

quota {
    policy = "read"
}
```

This is equivalent to the following JSON input:

```json
{
    "namespace": {
        "default": {
            "policy": "read"
        },
        "foo": {
            "policy": "write"
        }
    },
    "agent": {
        "policy": "read"
    },
    "node": {
        "policy": "read"
    },
    "quota": {
        "policy": "read"
    }
}
```

The [ACL Policy](/api/acl-policies.html) API allows either HCL or JSON to be used to define the content of the rules section.

### Namespace Rules

The `namespace` policy controls access to a namespace, including the [Jobs API](/api/jobs.html), [Deployments API](/api/deployments.html), [Allocations API](/api/allocations.html), and [Evaluations API](/api/evaluations.html).

```
namespace "default" {
    policy = "write"
}

namespace "sensitive" {
    policy = "read"
}
```

Namespace rules are keyed by the namespace name they apply to. When no namespace is specified, the "default" namespace is the one used. For example, the above policy grants writeaccess to the default namespace, and read access to the sensitive namespace. In addition to the coarse grained `policy` specification, the `namespace` stanza allows setting a more fine grained list of `capabilities`. This includes:

* `deny` - When multiple policies are associated with a token, deny will take precedence and prevent any capabilities.
* `list-jobs` - Allows listing the jobs and seeing coarse grain status.
* `read-job` - Allows inspecting a job and seeing fine grain status.
* `submit-job` - Allows jobs to be submitted or modified.
* `dispatch-job` - Allows jobs to be dispatched
* `read-logs` - Allows the logs associated with a job to be viewed.
* `read-fs` - Allows the filesystem of allocations associated to be viewed.
* `sentinel-override` - Allows soft mandatory policies to be overridden.

The coarse grained policy dispositions are shorthand for the fine grained capabilities:

* `deny` policy - ["deny"]
* `read` policy - ["list-jobs", "read-job"]
* `write` policy - ["list-jobs", "read-job", "submit-job", "read-logs", "read-fs", "dispatch-job"]

When both the policy short hand and a capabilities list are provided, the capabilities are merged:

```
# Allow reading jobs and submitting jobs, without allowing access
# to view log output or inspect the filesystem
namespace "default" {
    policy = "read"
    capabilities = ["submit-job"]
}
```

### Node Rules

The `node` policy controls access to the [Node API](/api/nodes.html) such as listing nodes or triggering a node drain.
Node rules are specified for all nodes using the `node` key:

```
node {
    policy = "read"
}
```

There's only one node policy allowed per rule set, and its value is set to one of the policy dispositions.

### Agent Rules

The `agent` policy controls access to the utility operations in the [Agent API](/api/agent.html), such as join and leave.
Agent rules are specified for all agents using the `agent` key:

```
agent {
    policy = "write"
}
```

There's only one agent policy allowed per rule set, and its value is set to one of the policy dispositions.


### Operator Rules

The `operator` policy controls access to the [Operator API](/api/operator.html). Operator rules look like:

```
operator {
    policy = "read"
}
```

There's only one operator policy allowed per rule set, and its value is set to one of the policy dispositions. In the example above, the token could be used to query the operator endpoints for diagnostic purposes but not make any changes.

### Quota Rules

The `quota` policy controls access to the quota specification operations in the [Quota API](/api/quotas.html), such as quota creation and deletion.
Quota rules are specified for all quotas using the `quota` key:

```
quota {
    policy = "write"
}
```

There's only one quota policy allowed per rule set, and its value is set to one of the policy dispositions.

# Advanced Topics

### Outages and Mulit-Region Replication

The ACL system takes some steps to ensure operation during outages. Clients nodes maintain a limited
cache of ACL tokens and ACL policies that have recently or frequently been used, associated with a time-to-live (TTL).

When the region servers are unavailable, the clients will automatically ignore the cache TTL,
and extend the cache until the outage has recovered. For any policies or tokens that are not cached,
they will be treated as missing and denied access until the outage has been resolved.

Nomad servers have all the policies and tokens locally and can continue serving requests even if
quorum is lost. The tokens and policies may become stale during this period as data is not actively
replicating, but will be automatically fixed when the outage has been resolved.

In a multi-region setup, there is a single authoritative region which is the source of truth for
ACL policies and global ACL tokens. All other regions asychronously replicate from the authoritative
region. When replication is interrupted, the existing data is used for request processing and may
become stale. When the authoritative region is reachable, replication will resume and repair any
inconsistency.

### Reseting ACL Bootstrap

If all management tokens are lost, it is possible to reset the ACL bootstrap so that it can be performed again.
First, we need to determine the reset index, this can be done by calling the reset endpoint:

```
$ nomad acl bootstrap

Error bootstrapping: Unexpected response code: 500 (ACL bootstrap already done (reset index: 7))
```

Here we can see the `reset index`. To reset the ACL system, we create the
`acl-bootstrap-reset` file in the data directory of the **leader** node:

```
$ echo 7 >> /nomad-data-dir/server/acl-bootstrap-reset
```

With the reset key setup, we can bootstrap like normal:

```
$ nomad acl bootstrap
Accessor ID  = 52d3353d-d7b9-d945-0591-1af608732b76
Secret ID    = 4b0a41ca-6d32-1853-e64b-de0d347e4525
Name         = Bootstrap Token
Type         = management
Global       = true
Policies     = n/a
Create Time  = 2017-09-11 18:38:11.929089612 +0000 UTC
Create Index = 11
Modify Index = 11
```

If we attempt to bootstrap again, we will get a mismatch on the reset index:

```
$ nomad acl bootstrap

Error bootstrapping: Unexpected response code: 500 (Invalid bootstrap reset index (specified 7, reset index: 11))
```

This is because the reset file is in place, but with the incorrect index.
The reset file can be deleted, but Nomad will not reset the bootstrap until the index is corrected.

