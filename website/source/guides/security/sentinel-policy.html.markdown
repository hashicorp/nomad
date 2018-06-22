---
layout: "guides"
page_title: "Sentinel Policies"
sidebar_current: "guides-security-sentinel"
description: |-
 Nomad integrates with Sentinel for fine-grained policy enforcement. Sentinel allows operators to express their policies as code, and have their policies automatically enforced. This allows operators to define a "sandbox" and restrict actions to only those compliant with policy. The Sentinel integration builds on the ACL System.
---

# Sentinel Policies

[Nomad Enterprise](/docs/enterprise/index.html) integrates with [HashiCorp Sentinel](https://docs.hashicorp.com/sentinel) for fine-grained policy enforcement. Sentinel allows operators to express their policies as code, and have their policies automatically enforced. This allows operators to define a "sandbox" and restrict actions to only those compliant with policy. The Sentinel integration builds on the [ACL System](/guides/security/acl.html).

~> **Enterprise Only!** This functionality only exists in Nomad Enterprise.
This is not present in the open source version of Nomad.

# Sentinel Overview

Sentinel integrates with the ACL system, and provides the ability to do fine grained policy enforcement. Users must have appropriate permissions to perform an action, and then are subject to any applicable Sentinel policies:

![Sentinel Overview](/assets/images/sentinel.jpg)

 * **Sentinel Policies**. Policies are able to introspect on request arguments and use complex logic to determine if the request meets policy requirements. For example, a Sentinel policy may restrict Nomad jobs to only using the "docker" driver, or prevent jobs from being modified outside of business hours.

 * **Policy Scope**. Sentinel policies declare a "scope", which determines when the policies apply. Currently the only supported scope is "submit-job", which applies to any new jobs being submitted, or existing jobs being updated.

 * **Enforcement Level**. Sentinel policies support multiple enforcement levels. The `advisory` level emits a warning when the policy fails, while `soft-mandatory` and `hard-mandatory` will prevent the operation. A `soft-mandatory` policy can be overridden if the user has necessary permissions.

### Sentinel Policies

Each Sentinel policy has a unique name, an optional description, applicable scope, enforcement level, and a Sentinel rule definition.
If multiple policies are installed for the same scope, all of them are enforced and must pass.

Sentinel policies _cannot_ be used unless the ACL system is enabled.

### Policy Scope

Sentinel policies specify an applicable scope, which limits when the policy is enforced. This allows policies to govern various aspects of the system.

The following table summarizes the scopes that are available for Sentinel policies:

| Scope      | Description                                           |
| ---------- | ----------------------------------------------------- |
| submit-job | Applies to any jobs (new or updated) being registered |


### Enforcement Level

Sentinel policies specify an enforcement level which changes how a policy is enforced. This allows for more flexibility in policy enforcement.

The following table summarizes the enforcement levels that are available:

| Enforcement Level | Description                                                            |
| ----------------- | ---------------------------------------------------------------------- |
| advisory          | Issues a warning when a policy fails                                   |
| soft-mandatory    | Prevents operation when a policy fails, issues a warning if overridden |
| hard-mandatory    | Prevents operation when a policy fails                                 |

The [`sentinel-override` capability](/guides/security/acl.html#sentinel-override) is required to override a `soft-mandatory` policy. This allows a restricted set of users to have override capability when necessary.

## Multi-Region Configuration

Nomad supports multi-datacenter and multi-region configurations. A single region is able to service multiple datacenters, and all servers in a region replicate their state between each other. In a multi-region configuration, there is a set of servers per region. Each region operates independently and is loosely coupled to allow jobs to be scheduled in any region and requests to flow transparently to the correct region.

When ACLs are enabled, Nomad depends on an "authoritative region" to act as a single source of truth for ACL policies, global ACL tokens, and Sentinel policies. The authoritative region is configured in the [`server` stanza](/docs/configuration/server.html) of agents, and all regions must share a single authoritative source. Any Sentinel policies are created in the authoritative region first. All other regions replicate Sentinel policies, ACL policies, and global ACL tokens to act as local mirrors. This allows policies to be administered centrally, and for enforcement to be local to each region for low latency.

## Configuring Sentinel Policies

Sentinel policies are tied to the ACL system, which is not enabled by default.
See the [ACL guide](/guides/security/acl.html) for details on how to configure ACLs.

## Example: Installing Sentinel Policies

This example shows how to install a Sentinel policy. It assumes that ACLs have already
been bootstrapped (see the [ACL guide](/guides/security/acl.html)), and that a `NOMAD_TOKEN` environment variable
is set to a management token.

First, create a Sentinel policy, named `test.sentinel`:

```
# Test policy always fails for demonstration purposes
main = rule { false }
```

Then, install this as an `advisory` policy which issues a warning on failure:

```
$ nomad sentinel apply -level=advisory test-policy test.sentinel
Successfully wrote "test-policy" Sentinel policy!
```

Use `nomad job init` to create a job file and attempt to submit it:

```
$ nomad job init
Example job file written to example.nomad

$ nomad job run example.nomad
Job Warnings:
1 warning(s):

* test-policy : Result: false (allowed failure based on level)

FALSE - test-policy:2:1 - Rule "main"


==> Monitoring evaluation "f43ac28d"
    Evaluation triggered by job "example"
    Evaluation within deployment: "11e01124"
    Allocation "2618f3b4" created: node "add8ce93", group "cache"
    Allocation "5c2674f2" created: node "add8ce93", group "cache"
    Allocation "9937811f" created: node "add8ce93", group "cache"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "f43ac28d" finished with status "complete"
```

We can see our policy failed, but the job was accepted because of an `advisory` enforcement level.

Next, let's change `test.sentinel` to only allow "exec" based drivers:

```
# Test policy only allows exec based tasks
main = rule { all_drivers_exec }

# all_drivers_exec checks that all the drivers in use are exec
all_drivers_exec = rule {
    all job.task_groups as tg {
        all tg.tasks as task {
            task.driver is "exec"
        }
    }
}
```

Then install the updated policy at a soft mandatory level:

```
$ nomad sentinel apply -level=soft-mandatory test-policy test.sentinel
Successfully wrote "test-policy" Sentinel policy!
```

With our new policy, attempt to submit the same job, which uses the "docker" driver:

```
$ nomad run example.nomad
Error submitting job: Unexpected response code: 500 (1 error(s) occurred:

* test-policy : Result: false

FALSE - test-policy:2:1 - Rule "main"
  FALSE - test-policy:6:5 - all job.task_groups as tg {
	all tg.tasks as task {
		task.driver is "exec"
	}
}

FALSE - test-policy:5:1 - Rule "all_drivers_exec"
)
```

Because our policy is failing, the job was rejected. Since this is a `soft-mandatory` policy,
submit with the `-policy-override` flag set:

```
$ nomad job run -policy-override example.nomad
Job Warnings:
1 warning(s):

* test-policy : Result: false (allowed failure based on level)

FALSE - test-policy:2:1 - Rule "main"
  FALSE - test-policy:6:5 - all job.task_groups as tg {
	all tg.tasks as task {
		task.driver is "exec"
	}
}

FALSE - test-policy:5:1 - Rule "all_drivers_exec"


==> Monitoring evaluation "16195b50"
    Evaluation triggered by job "example"
    Evaluation within deployment: "11e01124"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "16195b50" finished with status "complete"
```

This time, the job was accepted but with a warning that our policy is failing but was overridden.

# Policy Specification

Sentinel policies are specified in the [Sentinel
Language](https://docs.hashicorp.com/sentinel/). The language is designed to be
easy to read and write, while being fast to evaluate. There is no limitation on
how complex policies can be, but they are in the execution path so care should
be taken to avoid adversely impacting performance.

In each scope, there are different objects made available for introspection, such a job being submitted. Policies can
inspect these objects to apply fine-grained policies.

### Scope `submit-job`

The following objects are made available in the `submit-job` scope:

| Object | Description               |
| ------ | ------------------------- |
| `job`  | The job being submitted   |

See the [Sentinel Job Object](/guides/security/sentinel/job.html) for details on the fields that are available.

