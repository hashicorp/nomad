---
layout: "intro"
page_title: "Authentication"
sidebar_current: "gettingstarted-auth"
description: |-
  Authentication to Nomad gives a user access to use Nomad. Nomad can authenticate using multiple methods.
---

# Authentication

Now that we know how to use the basics of Nomad, it is important to understand
how to authenticate to Nomad itself. Up to this point, we haven't had to
authenticate because starting the Nomad server in dev mode automatically logs
us in as root. In practice, you'll almost always have to manually authenticate.

On this page, we'll talk specifically about _authentication_. On the next
page, we talk about _authorization_.
Authentication is the mechanism of assigning an identity to a Nomad user.
The access control and permissions associated with an identity are
authorization, and will not be covered on this page.

Nomad has pluggable authentication backends, making it easy to authenticate
with Nomad using whatever form works best for your organization. On this page
we'll use the token backend as well as the GitHub backend.

## Tokens

We'll first explain token authentication before going over any other
authentication backends. Token authentication is enabled by default in
Nomad and cannot be disabled. It is also what we've been using up to this
point.

When you start a dev server with `vault server -dev`, it outputs your
_root token_. The root token is the initial access token to configure Nomad.
It has root privileges, so it can perform any operation within Nomad.
We'll cover how to limit privileges in the next section.

You can create more tokens using `vault token-create`:

```
$ vault token-create
6c38f603-6441-2161-c543-ee15b7206563
```

By default, this will create a child token of your current token that
inherits all the same access control policies. The "child" concept here
is important: tokens always have a parent, and when that parent token is
revoked, children can also be revoked all in one operation. This makes it
easy when removing access for a user, to remove access for all sub-tokens
that user created as well.

After a token is created, you can revoke it with `vault token-revoke`:

```
$ vault token-revoke 6c38f603-6441-2161-c543-ee15b7206563
Revocation successful.
```

In a previous section, we use the `vault revoke` command. This command
is only used for revoking _secrets_. For revoking _tokens_, the
`vault token-revoke` command must be used.

To authenticate with a token, use the `vault auth` command:

```
$ vault auth d08e2bd5-ffb0-440d-6486-b8f650ec8c0c
Successfully authenticated! The policies that are associated
with this token are listed below:

root
```

This authenticates with Nomad. It will verify your token and let you know
what access policies the token is associated with. If you want to test
`vault auth`, make sure you create a new token first.

## Auth Backends

In addition to tokens, other authentication backends can be enabled.
Authentication backends enable alternate methods of identifying with Nomad.
These identities are tied back to a set of access policies, just like tokens.

Nomad supports other authentication backends in order to make authentication
easiest for your environment. For example, for desktop environments,
private key or GitHub based authentication may be easiest. For server
environments, some shared secret may be best. Auth backends give you
flexibility to choose what authentication you want to use.

As an example, let's authenticate using GitHub. First, enable the
GitHub authentication backend:

```
$ vault auth-enable github
Successfully enabled 'github' at 'github'!
```

Auth backends are mounted, just like secret backends, except auth
backends are always prefixed with `auth/`. So the GitHub backend we just
mounted can be accessed at `auth/github`. You can use `vault path-help` to
learn more about it.

With the backend enabled, we first have to configure it. For GitHub,
we tell it what organization users must part of, and map a team to a policy:

```
$ vault write auth/github/config organization=hashicorp
Success! Data written to: auth/github/config

$ vault write auth/github/map/teams/default value=root
Success! Data written to: auth/github/map/teams/default
```

The above configured our GitHub backend to only accept users from the
"hashicorp" organization (you should fill in your own organization)
and to map any team to the "root" policy, which is the only policy we have
right now until the next section.

With GitHub enabled, we can authenticate using `vault auth`:

```
$ vault auth -method=github token=e6919b17dd654f2b64e67b6369d61cddc0bcc7d5
Successfully authenticated! The policies that are associated
with this token are listed below:

root
```

Success! We've authenticated using GitHub. The "root" policy was associated
with my identity since we mapped that earlier. The value for "token" should be your own
[personal access token](https://help.github.com/articles/creating-an-access-token-for-command-line-use/).

You can revoke authentication from any authentication backend using
`vault token-revoke` as well, which can revoke any path prefix. For
example, to revoke all GitHub tokens, you could run the following.
**Don't run this unless you have access to another root token or you'll
get locked out.**

```
$ vault token-revoke -mode=path auth/github
```

When you're done, you can disable authentication backends with
`vault auth-disable`. This will immediately invalidate all authenticated
users from this backend.

```
$ vault auth-disable github
Disabled auth provider at path 'github'!
```

If you ran the above, you'll probably find you can't access your Nomad
anymore unless you have another root token, since it invalidated your
own session since we authenticated with GitHub above. Since we're still
operating in development mode, just restart the dev server to fix this.

## Next

In this page you learned about how Nomad authenticates users. You learned
about the built-in token system as well as enabling other authentication
backends. At this point you know how Nomad assigns an _identity_ to
a user.

The multiple authentication backends Nomad provides let you choose the
most appropriate authentication mechanism for your organization.

In this next section, we'll learn about
[access control policies](/intro/getting-started/acl.html).
