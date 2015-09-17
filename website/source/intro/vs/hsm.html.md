---
layout: "intro"
page_title: "Nomad vs. HSMs"
sidebar_current: "vs-other-hsm"
description: |-
  Comparison between Nomad and HSM systems.
---

# Nomad vs. HSMs

A [hardware security module (HSM)](http://en.wikipedia.org/wiki/Hardware_security_module)
is a hardware device that is meant to secure various secrets. They generally
have very strong security models and adhere to many compliance regulations.

The primary issue with HSMs is that they are expensive and not very
cloud friendly. Amazon provides CloudHSM, but the minimum price point to
even begin using CloudHSM is in the thousands of US dollars.

Once an HSM is up and running, configuring it is generally very tedious,
and the "API" to request secrets is also difficult to use. Example: CloudHSM
requires SSH and setting up various keypairs manually. It is difficult to
automate.

Nomad **doesn't replace an HSM**. There are many benefits to an HSM if
you can afford it. Instead, an HSM is a fantastic potential secret backend
for Nomad. This would allow Nomad to access the HSM data via the Nomad API,
making it significantly easier to use an HSM, while also retaining all the
audit logs. In fact, you'd have multiple audit logs: requests to Nomad
as well as to the HSM.

Nomad can also do many things that HSMs cannot currently do, such
as generating _dynamic secrets_. Instead of storing AWS access keys directly
within Nomad, Nomad can generate access keys according to a specific
policy on the fly. Nomad has the potential of doing this for any
system through its mountable secret backend system.

For most companies, an HSM is overkill, and Nomad is enough. For companies
that can afford an HSM, it can be used with Nomad to get the best of both
worlds.
