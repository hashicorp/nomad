---
layout: "docs"
page_title: "Scheduling"
sidebar_current: "docs-internals-scheduling"
description: |-
  Learn about how scheduling works in Nomad.
---

# Scheduling

Scheduling is a core function of Nomad. It is the process of assigning tasks
from jobs to client machines. The design is heavily inspired by Google's work on
both [Omega: flexible, scalable schedulers for large compute clusters][Omega] and
[Large-scale cluster management at Google with Borg][Borg]. See the links below
for implementation details on scheduling in Nomad.

- [Scheduling Internals](/docs/internals/scheduling/scheduling.html) - An overview of how the scheduler works.
- [Preemption](/docs/internals/scheduling/preemption.html) - Details of preemption, an advanced scheduler feature introduced in Nomad 0.9.

[Omega]: https://research.google.com/pubs/pub41684.html
[Borg]: https://research.google.com/pubs/pub43438.html