---
layout: "intro"
page_title: "Nomad vs. Custom Solutions"
sidebar_current: "vs-other-custom"
description: |-
  Comparison between Nomad and writing a custom solution.
---

# Nomad vs. Custom Solutions

Many organizations resort to custom solutions for storing secrets,
whether that be Dropbox, encrypted disk images, encrypted SQL columns,
etc.

These systems require time and resources to build and maintain.
Storing secrets is also an incredibly important piece of infrastructure
that must be done correctly. This increases the pressure to maintain
the internal systems.

Nomad is designed for secret storage. It provides a simple interface
on top of a strong security model to meet your secret storage needs.

Furthermore, Nomad is an open source tool. This means that the tool is
as good as the entire community working together to improve it. This
isn't just features and bug fixes, but finding potential security holes.
Additionally, since it is an open source, your own security teams can
review and contribute to Nomad and verify it meets your standards
for security.
