---
layout: "docs"
page_title: "Drivers"
sidebar_current: "docs-drivers"
description: |-
  Secret backends are mountable backends that store or generate secrets in Nomad.
---

# Drivers

Secret backends are the components in Nomad which store and generate
secrets.

Some secret backends, such as "generic", simply store and read
secrets verbatim. Other secret backends, such as "aws", create _dynamic
secrets_: secrets that are made on demand.

Secret backends are part of the
[mount system](#)
in Nomad. They behave very similarly to a virtual filesystem:
any read/write/delete is sent to the secret backend, and the secret
backend can choose to react to that operation however it sees fit.

For example, the "generic" backend passes through any operation back
to the configured storage backend for Nomad. A "read" turns into a
"read" of the storage backend at the same path, a "write" turns into
a write, etc. This is a lot like a normal filesystem.

The "aws" backend, on the other hand, behaves differently. When you
write to `aws/config/root`, it expects a certain format and stores that
information as configuration. You can't read from this path. When you
read from `aws/<name>`, it looks up an IAM policy named `<name>` and
generates AWS access credentials on demand and returns them. It doesn't
behave at all like a typical filesystem: you're not simply storing and
retrieving values, you're interacting with an API.

## Mounting/Unmounting Secret Backends

Secret backends can be mounted/unmounted using the CLI or the API.
There are three operations that can be performed with a secret backend
with regards to mounting:

  * **Mount** - This mounts a new secret backend. Multiple secret
    backends of the same type can be mounted at the same time by
    specifying different mount points. By default, secret backends are
    mounted to the same path as their name. This is what you want most
    of the time.

  * **Unmount** - This unmounts an existing secret backend. When a secret
    backend is unmounted, all of its secrets are revoked (if they support
    it), and all of the data stored for that backend in the physical storage
    layer is deleted.

  * **Remount** - This moves the mount point for an existing secret backend.
    This revokes all secrets, since secret leases are tied to the path they
    were created at. The data stored for the backend won't be deleted.

Once a secret backend is mounted, you can interact with it directly
at its mount point according to its own API. You can use the `vault path-help`
system to determine the paths it responds to.

## Barrier View

An important concept around secret backends is that they receive a
_barrier view_ to the configured Nomad physical storage. This is a lot
like a [chroot](http://en.wikipedia.org/wiki/Chroot).

Whenever a secret backend is mounted, a random UUID is generated. This
becomes the data root for that backend. Whenever that backend writes to
the physical storage layer, it is prefixed with that UUID folder. Since
the Nomad storage layer doesn't support relative access (such as `..`),
this makes it impossible for a mounted backend to access any other data.

This is an important security feature in Nomad: even a malicious backend
can't access the data from any other backend.
