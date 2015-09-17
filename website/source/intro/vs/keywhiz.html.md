---
layout: "intro"
page_title: "Nomad vs. Keywhiz"
sidebar_current: "vs-other-keywhiz"
description: |-
  Comparison between Nomad and Keywhiz.
---

# Nomad vs. Keywhiz

Keywhiz is a secret management solution built by Square. Keywhiz
has a client/server architecture based on a RESTful API. Clients of
Keywhiz access secrets through the API by authenticating with a client
certificate or cookie. To allow for flexible consumption of secrets by arbitrary
software, clients may also make use of a FUSE filesystem to expose secrets
as files on disk, and use Unix file permissions for access control.  Human
operators may authenticate using a cookie-based authentication either via command
line utilities or through a management web interface.

Nomad similarly is designed as a comprehensive secret management
solution. The client interaction with Nomad is flexible
both for authentication and usage of secrets. Nomad supports [mTLS
authentication](/docs/auth/cert.html) along with many [other
mechanisms](/docs/auth/index.html). The goal being to make it easy to
authenticate as a machine for programmatic access and as a human for
operator usage.

Nomad and Keywhiz expose secrets via an API. The Nomad
[ACL system](/docs/concepts/policies.html) is used
to protect secrets and gate access, similarly to the
Keywhiz ACL system.  With Nomad, All auditing is done
server side using [audit backends](/docs/audit/index.html).

Keywhiz focuses on storage and distribution of secrets and supports
rotation through secret versioning, which is possible in the Keywhiz UI and
command-line utilities. Nomad also supports dynamic secrets and generating credentials
on-demand for fine-grained security controls, but adds first class support
for non-repudiation. Key rotation is a first class concern for Keywhiz and Nomad, so
that no external systems need to be used.

Lastly Nomad forces a mandatory lease contract with clients. All secrets read
from Nomad have an associated lease which enables operators to audit key usage,
perform key rolling, and ensure automatic revocation. Nomad provides multiple
revocation mechanisms to give operators a clear "break glass" procedure after
a potential compromise.

