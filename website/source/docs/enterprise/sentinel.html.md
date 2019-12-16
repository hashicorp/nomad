---
layout: "docs"
page_title: "Sentinel"
sidebar_current: "docs-enterprise-sentinel"
description: |-
  Nomad Enterprise adds operations, collaboration, and governance capabilities
  to Nomad. Features include Namespaces, Resource Quotas, Sentinel Policies, and
  Advanced Autopilot.
---

# Sentinel

[Nomad Enterprise] integrates with [HashiCorp Sentinel][Sentinel] for
fine-grained policy enforcement. Sentinel allows operators to express their
policies as code and have their policies automatically enforced. This allows
operators to define a "sandbox" and restrict actions to only those compliant
with policy. The Sentinel integration builds on the [ACL System][ACLs].

Sentinel also integrates with the ACL system to provide the ability to create fine
grained policy enforcements. Users must have the appropriate permissions to perform
an action and are subject to any applicable Sentinel policies.

[![Sentinel Overview][img_sentinel_overview]][img_sentinel_overview]

- **Sentinel Policies** - Policies are able to introspect on request arguments
  and use complex logic to determine if the request meets policy requirements.
  For example, a Sentinel policy may restrict Nomad jobs to only using the
  "docker" driver or prevent jobs from being modified outside of business
  hours.

- **Policy Scope** - Sentinel policies declare a "scope", which determines when
  the policies apply. Currently the only supported scope is "submit-job", which
  applies to any new jobs being submitted, or existing jobs being updated.

- **Enforcement Level** - Sentinel policies support multiple enforcement levels.
  The `advisory` level emits a warning when the policy fails, while
  `soft-mandatory` and `hard-mandatory` will prevent the operation. A
  `soft-mandatory` policy can be overridden if the user has necessary
  permissions.

### Sentinel Policies

Each Sentinel policy has a unique name, an optional description, applicable
scope, enforcement level, and a Sentinel rule definition. If multiple policies
are installed for the same scope, all of them are enforced and must pass.

Sentinel policies _cannot_ be used unless the ACL system is enabled.

### Policy Scope

Sentinel policies specify an applicable scope, which limits when the policy is
enforced. This allows policies to govern various aspects of the system.

The following table summarizes the scopes that are available for Sentinel
policies:

| Scope      | Description                                           |
| ---------- | ----------------------------------------------------- |
| submit-job | Applies to any jobs (new or updated) being registered |

### Enforcement Level

Sentinel policies specify an enforcement level which changes how a policy is
enforced. This allows for more flexibility in policy enforcement.

The following table summarizes the enforcement levels that are available:

| Enforcement Level | Description                                                            |
| ----------------- | ---------------------------------------------------------------------- |
| advisory          | Issues a warning when a policy fails                                   |
| soft-mandatory    | Prevents operation when a policy fails, issues a warning if overridden |
| hard-mandatory    | Prevents operation when a policy fails                                 |

The [`sentinel-override` capability] is required to override a `soft-mandatory`
policy. This allows a restricted set of users to have override capability when
necessary.

### Multi-Region Configuration

Nomad supports multi-datacenter and multi-region configurations. A single region
is able to service multiple datacenters, and all servers in a region replicate
their state between each other. In a multi-region configuration, there is a set
of servers per region. Each region operates independently and is loosely coupled
to allow jobs to be scheduled in any region and requests to flow transparently
to the correct region.

When ACLs are enabled, Nomad depends on an "authoritative region" to act as a
single source of truth for ACL policies, global ACL tokens, and Sentinel
policies. The authoritative region is configured in the [`server` stanza] of
agents, and all regions must share a single authoritative source. Any Sentinel
policies are created in the authoritative region first. All other regions
replicate Sentinel policies, ACL policies, and global ACL tokens to act as local
mirrors. This allows policies to be administered centrally, and for enforcement
to be local to each region for low latency.

[`sentinel-override` capability]: http://www.nomadproject.io/guides/security/acl.html#sentinel-override
[`server` stanza]: http://www.nomadproject.io/docs/configuration/server.html
[ACLs]: https://www.nomadproject.io/guides/security/acl.html
[authoritative Nomad region]: http://www.nomadproject.io/docs/configuration/server.html#authoritative_region
[HTTP API]: https://www.nomadproject.io/api/quotas.html
[img_sentinel_overview]: /assets/images/sentinel.jpg
[JSON Specification of jobs]: http://www.nomadproject.io/api/json-jobs.html
[namespaces]: namespaces
[Nomad Enterprise]: https://www.hashicorp.com/products/nomad/
[quotas commands]: http://www.nomadproject.io/docs/commands/quotas.html
[quotas]: quotas
[Sentinel policies]: sentinel
[Sentinel]: https://docs.hashicorp.com/sentinel/
