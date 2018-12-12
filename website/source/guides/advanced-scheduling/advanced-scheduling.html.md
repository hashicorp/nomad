---
layout: "guides"
page_title: "Advanced Scheduling Features"
sidebar_current: "guides-advanced-scheduling"
description: |-
    Introduce advanced scheduling features including affinity and spread.
---

# Advanced Scheduling with Nomad

The Nomad [scheduler][scheduling] uses a bin packing algorithm to optimize the resource utilization and density of applications in your Nomad cluster. This feature, in addition to the ability to restrict which nodes a workload runs on with the [constraint][constraint-stanza] stanza, provides the Nomad operator an efficient and powerful way to schedule jobs.

With the release of Nomad 0.9, we are now introducing new features that allow
Nomad operators to exercise greater control over scheduling decisions for their
workloads. Please refer to the specific documentation below or in the sidebar for more detailed information about each feature.

- [Affinity](/guides/advanced-scheduling/affinity.html)
- [Spread](/guides/advanced-scheduling/spread.html)

[constraint-stanza]: /docs/job-specification/constraint.html
[scheduling]: /docs/internals/scheduling.html

