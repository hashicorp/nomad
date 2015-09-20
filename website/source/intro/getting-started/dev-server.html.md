---
layout: "intro"
page_title: "Starting the Server"
sidebar_current: "gettingstarted-devserver"
description: |-
  After installing Nomad, the next step is to start the server.
---

# Starting the Nomad Server

With Nomad installed, the next step is to start a Nomad server.

Nomad operates as a client/server application. The Nomad server is the
only piece of the Nomad architecture that interacts with the data
storage and backends. All operations done via the Nomad CLI interact
with the server over a TLS connection.

In this page, we'll start and interact with the Nomad server to understand
how the server is started, and understanding the seal/unseal process.

## Starting the Dev Server

To start, we're going to start the Nomad _dev server_. The dev server
is a built-in flag to start a pre-configured server that is not very
secure but useful for playing with Nomad locally. Later in the getting
started guide we'll configure and start a real server.

To start the Nomad dev server, run `vault server -dev`:

```
$ vault server -dev
WARNING: Dev mode is enabled!

In this mode, Nomad is completely in-memory and unsealed.
Nomad is configured to only have a single unseal key. The root
token has already been authenticated with the CLI, so you can
immediately begin using the Nomad CLI.

The only step you need to take is to set the following
environment variable since Nomad will be talking without TLS:

    export VAULT_ADDR='http://127.0.0.1:8200'

The unseal key and root token are reproduced below in case you
want to seal/unseal the Nomad or play with authentication.

Unseal Key: 2252546b1a8551e8411502501719c4b3
Root Token: 79bd8011-af5a-f147-557e-c58be4fedf6c

==> Nomad server configuration:

         Log Level: info
           Backend: inmem
        Listener 1: tcp (addr: "127.0.0.1:8200", tls: "disabled")

...
```

You should see output similar to that above. As you can see, when you
start a dev server, Nomad warns you loudly. The dev server stores all
its data in-memory (but still encrypted), listens on localhost without TLS, and
automatically unseals and shows you the unseal key and root access key.
We'll go over what all this means shortly.

The important thing about the dev server is that it is meant for
development only. **Do not run the dev server in production.** Even if it
was run in production, it wouldn't be very useful since it stores data in-memory
and every restart would clear all your secrets.

With the dev server running, do the following three things before anything
else:

  1. Copy and run the `export VAULT_ADDR ...` command from your terminal
     output. This will configure the Nomad client to talk to our dev server.

  2. Save the unseal key somewhere. Don't worry about _how_ to save this
     securely. For now, just save it anywhere.

  3. Do the same as step 2, but with the root token. We'll use this later.

## Verify the Server is Running

Verify the server is running by running `vault status`. This should
succeed and exit with exit code 0. If you see an error about opening
a connection, make sure you copied and executed the `export VAULT_ADDR...`
command from above properly.

If it ran successful, the output should look like below:

```
$ vault status
Sealed: false
Key Shares: 1
Key Threshold: 1
Unseal Progress: 0

High-Availability Enabled: false
```

If the output looks different, especially if the numbers are different
or the Nomad is sealed, then restart the dev server and try again. The
only reason these would ever be different is if you're running a dev
server from going through this guide previously.

We'll cover what this output means later in the guide.

## Next

Congratulations! You've started your first Nomad server. We haven't stored
any secrets yet, but we'll do that in the next section.

Next, we're going to
[read and write our first secrets](/intro/getting-started/first-secret.html).
