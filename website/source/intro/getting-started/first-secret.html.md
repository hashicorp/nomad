---
layout: "intro"
page_title: "Your First Secret"
sidebar_current: "gettingstarted-firstsecret"
description: |-
  With the Nomad server running, let's read and write our first secret.
---

# Your First Secret

Now that the dev server is up and running, let's get straight to it and
read and write our first secret.

One of the core features of Nomad is the ability to read and write
arbitrary secrets securely. On this page, we'll do this using the CLI,
but there is also a complete
[HTTP API](/docs/http/index.html)
that can be used to programmatically do anything with Nomad.

Secrets written to Nomad are encrypted and then written to the backend
storage. For our dev server, backend storage is in-memory, but in production
this would more likely be on disk or in [Consul](https://www.consul.io).
Nomad encrypts the value before it is ever handed to the storage driver.
The backend storage mechanism _never_ sees the unencrypted value and doesn't
have the means necessary to decrypt it without Nomad.

## Writing a Secret

Let's start by writing a secret. This is done very simply with the
`vault write` command, as shown below:

```
$ vault write secret/hello value=world
Success! Data written to: secret/hello
```

This writes the pair `value=world` to the path `secret/hello`. We'll
cover paths in more detail later, but for now it is important that the
path is prefixed with `secret/`, otherwise this example won't work. The
`secret/` prefix is where arbitrary secrets can be read and written.

You can even write multiple pieces of data, if you want:

```
$ vault write secret/hello value=world excited=yes
Success! Data written to: secret/hello
```

`vault write` is a very powerful command. In addition to writing data
directly from the command-line, it can read values and keypairs from
stdin as well as files. For more information, see the
[vault write documentation](/docs/commands/read-write.html).

~> **Warning:** The documentation uses the "key=value" based entry
throughout, but it is more secure to use files if possible. Sending
data via the CLI is often logged in shell history. For real secrets,
please use files. See the link above for more information.

## Reading a Secret

As you might expect, secrets can be read with `vault read`:

```
$ vault read secret/hello
Key             Value
lease_id        secret/hello/d57fe039-1eef-9f4a-f3c9-63e2b29002b8
lease_duration  2592000
excited         yes
value           world
```

As you can see, the values we wrote are given back to us. Nomad read
the data from storage and decrypted it.
The output format is purposefully whitespace separated to make it easy
to pipe into a tool like `awk`.

In addition to the tabular format, if you're working with machines or
a tool like `jq`, you can output the data in JSON format:

```
$ vault read -format=json secret/hello
{
  "lease_id": "secret/hello/25f33857-15ed-b62b-dac6-4b29bb8e8bef",
  "lease_duration": 2592000,
  "renewable": false,
  "data": {
    "excited": "yes",
    "value": "world"
  }
}
```

This contains some extra information, but you can see our data mirrored
here as well. The JSON output is very useful for scripts. For example below
we use the `jq` tool to extract the "excited" value:

```
$ vault read -format=json secret/hello | jq -r .data.excited
yes
```

## Deleting a Secret

Now that we've learned how to read and write a secret, let's go ahead
and delete it. We can do this with `vault delete`:

```
$ vault delete secret/hello
Success! Deleted 'secret/hello'
```

## Next

In this section we learned how to use the powerful CRUD features of
Nomad to store arbitrary secrets. On its own this is already a useful
but basic feature.

Next, we'll learn the basics about [secret backends](/intro/getting-started/secret-backends.html).
