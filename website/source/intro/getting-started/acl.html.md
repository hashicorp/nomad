---
layout: "intro"
page_title: "Access Control Policies"
sidebar_current: "gettingstarted-acl"
description: |-
  Access control policies in Nomad control what a user can access.
---

# Access Control Policies (ACLs)

Access control policies in Nomad control what a user can access. In
the last section, we learned about _authentication_. This section is
about _authorization_.

Whereas for authentication Nomad has multiple options or backends that
can be enabled and used, the authorization or policies of Nomad are always
the same format. All authentication backends must map identities back to
the core policies that are configured with Nomad.

When initializing Nomad, there is always one special policy created
that can't be removed: the "root" policy. This policy is a special policy
that gives superuser access to everything in Nomad. An identity mapped to
the root policy can do anything.

## Policy Format

Policies in Nomad are formatted with
[HCL](https://github.com/hashicorp/hcl). HCL is a human-readable configuration
format that is also JSON-compatible, so you can use JSON as well. An example
policy is shown below:

```javascript
path "secret/*" {
  policy = "write"
}

path "secret/foo" {
  policy = "read"
}
```

The policy format uses a prefix matching system on the API path
to determine access control. The most specific defined policy is used,
either an exact match or the longest-prefix glob match. Since everything
in Nomad must be accessed via the API, this gives strict control over every
aspect of Nomad, including mounting backends, authenticating, as well as secret access.

In the policy above, a user could write any secret to `secret/`, except
to `secret/foo`, where only read access is allowed. Policies default to
deny, so any access to an unspecified path is not allowed. The policy
language changed slightly in Nomad 0.2, [see this page for details](/docs/concepts/policies.html).

Save the above policy as `acl.hcl`.

## Writing the Policy

To write a policy, use the `vault policy-write` command:

```
$ vault policy-write secret acl.hcl
Policy 'secret' written.
```

You can see the policies that are available with `vault policies`, and you
can see the contents of a policy with `vault policies <name>`. Only users with
root access can do this.

## Testing the Policy

To use the policy, let's create a token and assign it to that policy.
Make sure to save your root token somewhere so you can authenticate
back to a root user later.

```
$ vault token-create -policy="secret"
d97ef000-48cf-45d9-1907-3ea6ce298a29

$ vault auth d97ef000-48cf-45d9-1907-3ea6ce298a29
Successfully authenticated! The policies that are associated
with this token are listed below:

secret
```

You can now verify that you can write data to `secret/`, but only
read from `secret/foo`:

```
$ vault write secret/bar value=yes
Success! Data written to: secret/bar

$ vault write secret/foo value=yes
Error writing data to secret/foo: Error making API request.

URL: PUT http://127.0.0.1:8200/v1/secret/foo
Code: 400. Errors:

* permission denied
```

You also don't have access to `sys` according to the policy, so commands
such as `vault mounts` will not work either.

## Mapping Policies to Auth Backends

Nomad is the single policy authority, unlike auth where you can mount
multiple backends. Any mounted auth backend must map identities to these
core policies.

Use the `vault path-help` system with your auth backend to determine how the
mapping is done, since it is specific to each backend. For example,
with GitHub, it is done by team using the `map/teams/<team>` path:

```
$ vault write auth/github/map/teams/default value=secret
Success! Data written to: auth/github/map/teams/default
```

For GitHub, the "default" team is the default policy set that everyone
is assigned to no matter what team they're on.

Other auth backends use alternate, but likely similar mechanisms for
mapping policies to identity.

## Next

Policies are an important part of Nomad. While using the root token
is easiest to get up and running, you'll want to restrict access to
Nomad very quickly, and the policy system is the way to do this.

The syntax and function of policies is easy to understand and work
with, and because auth backends all must map to the central policy system,
you only have to learn this policy system.

Next, we'll cover how to [deploy Nomad](/intro/getting-started/deploy.html).
