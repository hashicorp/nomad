# Running Nomad tests

You can run all Nomad tests with `make test`. This performs all the necessary
setup prior to running tests themselves. Run `make test-nomad` to exclude ui
tests in case you are already set up to run them.

See /ui/README.md for instructions on building the Nomad UI and running its tests.

## Some tests require `make dev` first

Test in the `api` package run against a real Nomad cluster running locally.
Therefore you need to rebuild the dev version of Nomad each time you make a change
that affects behavior of the server, in order for the tests to see the change.

Run `make dev` to rebuild Nomad for local testing.

Running all tests with `make test-nomad` already performs this build step.

### Running specific tests

If you want to run a single test from the `api` package, first run `make dev`
if you have made any changes outside of the test files.

Example when iterating on a single test:

    $ make dev && go test -v -run TestAPI_OperatorAutopilotGetSetConfiguration


