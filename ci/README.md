# CI (unit testing)

This README describes how the Core CI Tests Github Actions works, which provides
Nomad with continuous integration unit testing.

## Steps

1. When a branch is pushed, GHA triggers `.github/workflows/test-core.yaml`.

2. The first job is `mods` which creates a pre-cache of Go modules.
  - Only useful for the followup jobs on Linux runners
  - Is keyed on `hash(go.sum)`, so a cache is re-used until deps are modified.

3. The `checks`, `test-api`, `test-*` jobs are started.
  - The checks job runs `make check`
  - The test job runs groups of tests, see below

3i. The check step also runs `make missing`
  - Invokes `tools/missing` to scan `ci/test-cores.json` && nomad source.
  - Fails the build if any packages in Nomad are not covered.

4a. The `test-*` jobs are run.
  - Configured as a matrix of "groups"; each group is a set of packages.
  - The GHA invokes `test-nomad` with $GOTEST_GROUP for each group.
  - The makefile uses `tools/missing` to translate the group into packages
  - Package groups are configured in `ci/test-core.json`

4b. The `test-api` job is run.
  - Because `api` is a submodule, invokation of test command is special.
  - The GHA invokes `test-nomad-module` with the name of the submodule.

5. The `compile` jobs are run
  - Waits on checks to complete first
  - Runs on each of `linux`, `macos`, `windows`
