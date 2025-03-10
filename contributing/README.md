Nomad Codebase Documentation
===

This directory contains some documentation about the Nomad codebase,
aimed at readers who are interested in making code contributions.

If you're looking for information on _using_ Nomad, please instead refer
to the [Nomad website](https://developer.hashicorp.com/nomad).

The [good first issue label](https://github.com/hashicorp/nomad/issues?q=is:issue+is:open+label:%22good+first+issue%22)
is used to identify issues which are suited to first time contributors.

Developing with Vagrant
---
A development environment is supplied via Vagrant to make getting started easier.

1. Install [Vagrant](https://www.vagrantup.com/docs/installation)
1. Install [Virtualbox](https://www.virtualbox.org/)
1. Bring up the Vagrant project
    ```sh
    $ git clone https://github.com/hashicorp/nomad.git
    $ cd nomad
    $ vagrant up
    ```

    The virtual machine will launch, and a provisioning script will install the
    needed dependencies within the VM.

1. SSH into the VM
    ```sh
    $ vagrant ssh
    ```

Developing without Vagrant
---
1. Install [Go 1.24.1+](https://golang.org/) *(Note: `gcc-go` is not supported)*
1. Clone this repo
   ```sh
   $ git clone https://github.com/hashicorp/nomad.git
   $ cd nomad
   ```
1. Bootstrap your environment
   ```sh
   $ make bootstrap
   ```
1. (Optionally) Set a higher ulimit, as Nomad creates many file handles during normal operations
   ```sh
   $ [ "$(ulimit -n)" -lt 1024 ] && ulimit -n 1024
   ```
1. Verify you can run smoke tests
   ```sh
   $ make test
   ```
   **Note:** You can think of this as a `smoke` test which runs a subset of
   tests and some may fail because of `operation not permitted` error which
   requires `root` access. You can use `go test` to test the specific subsystem
   which you are working on and let the CI run rest of the tests for you.

Running a development build
---
1. Compile a development binary (see the [UI README](https://github.com/hashicorp/nomad/blob/main/ui/README.md) to include the web UI in the binary)
    ```sh
    $ make dev
    # find the built binary at ./bin/nomad
    ```
1. Start the agent in dev mode
    ```sh
    $ sudo bin/nomad agent -dev
    ```
1. (Optionally) Run Consul to enable service discovery and health checks
    1. Download [Consul](https://www.consul.io/downloads)
    1. Start Consul in dev mode
        ```sh
        $ consul agent -dev
        ```

Compiling Protobufs
---
If in the course of your development you change a Protobuf file (those ending in .proto), you'll need to recompile the protos.

1. Run `make boostrap` to install the [`buf`](https://github.com/bufbuild/buf)
   command.
1. Compile Protobufs
    ```sh
    $ make proto
    ```

Building the Web UI
---
See the [UI README](https://github.com/hashicorp/nomad/blob/main/ui/README.md) for instructions.

Create a release binary
---
To create a release binary:

```sh
$ make prerelease
$ make release
$ ls ./pkg
```

This will generate all the static assets, compile Nomad for multiple
platforms and place the resulting binaries into the `./pkg` directory.

API Compatibility
--------------------
Only the `api/` and `plugins/` packages are intended to be imported by other projects. The root Nomad module does not follow semver and is not intended to be imported directly by other projects.

## Architecture

When working on Nomad, there are a few major packages that are the entrypoint
for most tasks you might be working on:

* `acl/`: The definition of ACL policies and authorization methods
* `api/`: The public-facing Go SDK for the HTTP API.
* `client/`: Most of the code that runs in the Nomad client agents.
  * `client/allocrunner/`: The code that manages a single allocation, including
    hooks for workload identity, CSI, and networking. The allocrunner calls into
    `taskrunner` for each task.
  * `client/allocrunner/taskrunner/`: The code that manages a single task within
    an allocation, including hooks for artifacts, templates, Consul service
    mesh, logging, etc. The task runner invokes the task driver found in
    `drivers`.
* `command/`: The definition of Nomad CLI commands, most of which use the HTTP
  API.
  * `command/agent/`: The parts of the Nomad agent that are neither server or
    client, including the HTTP API server and configuration parsing.
  * `command/agent/consul/`: The Consul API client for service registration.
* `drivers/`: The implementations of the built-in `docker`, `exec`, `raw_exec`,
  `java`, and `qemu` task drivers, as well as shared "executor" code.
* `e2e/` and `enos/`: Packages defining infrastructure and tests for nightly
  end-to-end testing.
* `nomad/`: The Nomad server code, including RPC handlers, running a Raft node,
  the plan applier, the eval broker, and the keyring.
  * `nomad/state/`: The in-memory state (memdb) of the Nomad servers
  * `nomad/structs/`: Type definitions used in RPC and state.
* `plugins/`: Interface definitions for task drivers, device drivers, and CSI
  drivers. Implementations can be found in `drivers` (and as external repos).
* `scheduler/`: The logic for scheduling workloads lives here, called from the
  server code in `nomad`.
* `ui/`: The web UI.
* `website/`: The documentation website.

The high level control flow for many Nomad actions (via the CLI or UI) are:

```
# Read actions:
Client -> HTTP API -> RPC -> StateStore

# Actions that change state:
Client -> HTTP API -> RPC -> Raft -> FSM -> StateStore
```

Checklists
---

When adding new features to Nomad there are often many places to make changes.
It is difficult to determine where changes must be made and easy to make
mistakes.

The following checklists are meant to be copied and pasted into PRs to give
developers and reviewers confidence that the proper changes have been made:

* [New `jobspec` entry](checklist-jobspec.md)
* [New CLI command](checklist-command.md)
* [New RPC endpoint](checklist-rpc-endpoint.md)

Tooling
---

* [Go tool versions](golang.md)
