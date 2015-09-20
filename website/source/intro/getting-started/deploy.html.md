---
layout: "intro"
page_title: "Deploy Nomad"
sidebar_current: "gettingstarted-deploy"
description: |-
  Learn how to deploy Nomad into production, how to initialize it, configure it, etc.
---

# Deploy Nomad

Up to this point, we've been working with the dev server, which
automatically authenticated us, setup in-memory storage, etc. Now that
you know the basics of Nomad, it is important to learn how to deploy
Nomad into a real environment.

On this page, we'll cover how to configure Nomad, start Nomad, the
seal/unseal process, and scaling Nomad.

## Configuring Nomad

Nomad is configured using [HCL](https://github.com/hashicorp/hcl) files.
As a reminder, these files are also JSON-compatible. The configuration
file for Nomad is relatively simple. An example is shown below:

```javascript
backend "consul" {
  address = "127.0.0.1:8500"
  path = "vault"
}

listener "tcp" {
 address = "127.0.0.1:8200"
 tls_disable = 1
}
```

Within the configuration file, there are two primary configurations:

  * `backend` - This is the physical backend that Nomad uses for
    storage. Up to this point the dev server has used "inmem" (in memory),
    but in the example above we're using [Consul](http://www.consul.io),
    a much more production-ready backend.

  * `listener` - One or more listeners determine how Nomad listens for
    API requests. In the example above we're listening on localhost port
    8200 without TLS.

For now, copy and paste the configuration above to `example.hcl`. It will
configure Nomad to expect an instance of Consul running locally.

Starting a local Consul instance takes only a few minutes. Just follow the
[Consul Getting Started Guide](https://www.consul.io/intro/getting-started/install.html)
up to the point where you have installed Consul and started it with this command:

```shell
$ consul agent -server -bootstrap-expect 1 -data-dir /tmp/consul
```

## Starting the Server

With the configuration in place, starting the server is simple, as
shown below. Modify the `-config` flag to point to the proper path
where you saved the configuration above.

```
$ vault server -config=example.hcl
==> Nomad server configuration:

         Log Level: info
           Backend: consul
        Listener 1: tcp (addr: "127.0.0.1:8200", tls: "disabled")

==> Nomad server started! Log data will stream in below:
```

Nomad outputs some information about its configuration, and then blocks.
This process should be run using a resource manager such as systemd or
upstart.

You'll notice that you can't execute any commands. We don't have any
auth information! When you first setup a Nomad server, you have to start
by _initializing_ it.

On Linux, Nomad may fail to start with the following error:

```shell
$ vault server -config=example.hcl
Error initializing core: Failed to lock memory: cannot allocate memory

This usually means that the mlock syscall is not available.
Nomad uses mlock to prevent memory from being swapped to
disk. This requires root privileges as well as a machine
that supports mlock. Please enable mlock on your system or
disable Nomad from using it. To disable Nomad from using it,
set the `disable_mlock` configuration option in your configuration
file.
```

For guidance on dealing with this issue, see the discussion of
`disable_mlock` in [Server Configuration](/docs/config/index.html).

## Initializing the Nomad

Initialization is the process of first configuring the Nomad. This
only happens once when the server is started against a new backend that
has never been used with Nomad before.

During initialization, the encryption keys are generated, unseal keys
are created, and the initial root token is setup. To initialize Nomad
use `vault init`. This is an _unauthenticated_ request, but it only works
on brand new Nomads with no data:

```
$ vault init
Key 1: 427cd2c310be3b84fe69372e683a790e01
Key 2: 0e2b8f3555b42a232f7ace6fe0e68eaf02
Key 3: 37837e5559b322d0585a6e411614695403
Key 4: 8dd72fd7d1af254de5f82d1270fd87ab04
Key 5: b47fdeb7dda82dbe92d88d3c860f605005
Initial Root Token: eaf5cc32-b48f-7785-5c94-90b5ce300e9b

Nomad initialized with 5 keys and a key threshold of 3!

Please securely distribute the above keys. Whenever a Nomad server
is started, it must be unsealed with 3 (the threshold)
of the keys above (any of the keys, as long as the total number equals
the threshold).

Nomad does not store the original master key. If you lose the keys
above such that you no longer have the minimum number (the
threshold), then your Nomad will not be able to be unsealed.
```

Initialization outputs two incredibly important pieces of information:
the _unseal keys_ and the _initial root token_. This is the
**only time ever** that all of this data is known by Nomad, and also the
only time that the unseal keys should ever be so close together.

For the purpose of this getting started guide, save all these keys
somewhere, and continue. In a real deployment scenario, you would never
save these keys together.

## Seal/Unseal

Every initialized Nomad server starts in the _sealed_ state. From
the configuration, Nomad can access the physical storage, but it can't
read any of it because it doesn't know how to decrypt it. The process
of teaching Nomad how to decrypt the data is known as _unsealing_ the
Nomad.

Unsealing has to happen every time Nomad starts. It can be done via
the API and via the command line. To unseal the Nomad, you
must have the _threshold_ number of unseal keys. In the output above,
notice that the "key threshold" is 3. This means that to unseal
the Nomad, you need 3 of the 5 keys that were generated.

-> **Note:** Nomad does not store any of the unseal key shards. Nomad
uses an algorithm known as
[Shamir's Secret Sharing](http://en.wikipedia.org/wiki/Shamir%27s_Secret_Sharing)
to split the master key into shards. Only with the threshold number of keys
can it be reconstructed and your data finally accessed.

Begin unsealing the Nomad with `vault unseal`:

```
$ vault unseal
Key (will be hidden):
Sealed: true
Key Shares: 5
Key Threshold: 3
Unseal Progress: 1
```

After pasting in a valid key and confirming, you'll see that the Nomad
is still sealed, but progress is made. Nomad knows it has 1 key out of 3.
Due to the nature of the algorithm, Nomad doesn't know if it has the
_correct_ key until the threshold is reached.

Also notice that the unseal process is stateful. You can go to another
computer, use `vault unseal`, and as long as it's pointing to the same server,
that other computer can continue the unseal process. This is incredibly
important to the design of the unseal process: multiple people with multiple
keys are required to unseal the Nomad. The Nomad can be unsealed from
multiple computers and the keys should never be together. A single malicious
operator does not have enough keys to be malicious.

Continue with `vault unseal` to complete unsealing the Nomad. Note that
all 3 keys must be different, but they can be any other keys. As long as
they're correct, you should soon see output like this:

```
$ vault unseal
Key (will be hidden):
Sealed: false
Key Shares: 5
Key Threshold: 3
Unseal Progress: 0
```

The `Sealed: false` means the Nomad is unsealed!

Feel free to play around with entering invalid keys, keys in different
orders, etc. in order to understand the unseal process. It is very important.
Once you're ready to move on, use `vault auth` to authenticate with
the root token.

As a root user, you can reseal the Nomad with `vault seal`. A single
operator is allowed to do this. This lets a single operator lock down
the Nomad in an emergency without consulting other operators.

When the Nomad is sealed again, it clears all of its state (including
the encryption key) from memory. The Nomad is secure and locked down
from access.

## Next

You now know how to configure, initialize, and unseal/seal Nomad.
This is the basic knowledge necessary to deploy Nomad into a real
environment. Once the Nomad is unsealed, you access it as you have
throughout this getting started guide (which worked with an unsealed Nomad).

Next, we have a [short tutorial](/intro/getting-started/apis.html) on using the HTTP APIs to authenticate and access secrets.
