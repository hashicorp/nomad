---
layout: "intro"
page_title: "Introduction"
sidebar_current: "what"
description: |-
  Welcome to the intro guide to Nomad! This guide is the best place to start with Nomad. We cover what Nomad is, what problems it can solve, how it compares to existing software, and contains a quick start for using Nomad.
---

# Introduction to Nomad

Welcome to the intro guide to Nomad! This guide is the best
place to start with Nomad. We cover what Nomad is, what
problems it can solve, how it compares to existing software,
and contains a quick start for using Nomad.

If you are already familiar with the basics of Nomad, the
[documentation](/docs/index.html) provides a better reference
guide for all available features as well as internals.

## What is Nomad?

Nomad is a tool for securely accessing _secrets_. A secret is anything
that you want to tightly control access to, such as API keys, passwords,
certificates, and more. Nomad provides a unified interface to any
secret, while providing tight access control and recording a detailed
audit log.

A modern system requires access to a multitude of secrets: database
credentials, API keys for external services, credentials for
service-oriented architecture communication, etc. Understanding who is
accessing what secrets is already very difficult and platform-specific.
Adding on key rolling, secure storage, and detailed audit logs is almost
impossible without a custom solution. This is where Nomad steps in.

Examples work best to showcase Nomad. Please see the
[use cases](/intro/use-cases.html).

The key features of Nomad are:

* **Secure Secret Storage**: Arbitrary key/value secrets can be stored
  in Nomad. Nomad encrypts these secrets prior to writing them to persistent
  storage, so gaining access to the raw storage isn't enough to access
  your secrets. Nomad can write to disk, [Consul](http://www.consul.io),
  and more.

* **Dynamic Secrets**: Nomad can generate secrets on-demand for some
  systems, such as AWS or SQL databases. For example, when an application
  needs to access an S3 bucket, it asks Nomad for credentials, and Nomad
  will generate an AWS keypair with valid permissions on demand. After
  creating these dynamic secrets, Nomad will also automatically revoke them
  after the lease is up.

* **Data Encryption**: Nomad can encrypt and decrypt data without storing
  it. This allows security teams to define encryption parameters and
  developers to store encrypted data in a location such as SQL without
  having to design their own encryption methods.

* **Leasing and Renewal**: All secrets in Nomad have a _lease_ associated
  with it. At the end of the lease, Nomad will automatically revoke that
  secret. Clients are able to renew leases via built-in renew APIs.

* **Revocation**: Nomad has built-in support for secret revocation. Nomad
  can revoke not only single secrets, but a tree of secrets, for example
  all secrets read by a specific user, or all secrets of a particular type.
  Revocation assists in key rolling as well as locking down systems in the
  case of an intrusion.

## Next Steps

See the page on [Nomad use cases](/intro/use-cases.html) to see the
multiple ways Nomad can be used. Then see
[how Nomad compares to other software](/intro/vs/index.html)
to see how it fits into your existing infrastructure. Finally, continue onwards with
the [getting started guide](/intro/getting-started/install.html) to use
Nomad to read, write, and create real secrets and see how it works in practice.
