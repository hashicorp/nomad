# Nomad Codebase Documentation

This directory contains some documentation about the Nomad codebase,
aimed at readers who are interested in making code contributions.

If you're looking for information on _using_ Nomad, please instead refer
to the [Nomad website](https://nomadproject.io).

## Architecture

The code for Nomad's major components is organized as:

* `api/` provides a Go client for Nomad's HTTP API.
* `client/` contains Nomad's client agent code.
* `command/` contains Nomad's CLI code.
* `nomad/` contains Nomad's server agent code.
* `ui/` contains Nomad's UI code.
* `website/` contains Nomad's website and documentation.

The high level control flow for many Nomad actions (via the CLI or UI) are:

```
# Read actions:
Client -> HTTP API -> RPC -> StateStore

# Actions that change state:
Client -> HTTP API -> RPC -> Raft -> FSM -> StateStore
```

## Checklists

When adding new features to Nomad there are often many places to make changes.
It is difficult to determine where changes must be made and easy to make
mistakes.

The following checklists are meant to be copied and pasted into PRs to give
developers and reviewers confidence that the proper changes have been made:

* [New `jobspec` entry](checklist-jobspec.md)
* [New CLI command](checklist-command.md)
* [New RPC endpoint](checklist-rpc-endpoint.md)

## Tooling

* [Go tool versions](golang.md)
