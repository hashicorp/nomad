# Running Nomad tests

Run `make test` to run all Nomad tests.
This performs all the necessary setup prior to running tests themselves,
and sets relevant build tags etc.

## Excluding UI tests

Run `make test-nomad` to exclude ui tests in case you are already set up to run them.
See /ui/README.md for instructions on building the Nomad UI and running its tests.

## Iterating on a specific test

If you want to iterate on a specific test using standard `go test` tool rather than
the make recipes, there are some additional steps you need to take.

### Always set the `nomad_test` tag.

When running tests or building binaries to run tests against (see below) you should
always use the `nomad_test` tag. This ensures you have mock drivers available, and
may have other significance for tests in future.

### Tests that need a Nomad binary

Some tests run against a built Nomad binary. They expect this binary to either be
in your `$PATH`, `$GOPATH/bin`, or current working directory (checked in that order).

Usually, you should run `make dev` prior to each run of these tests, if you have made
and changes outside of the tests themselves. This will ensure your changes are
reflected in the test results. You'll also need to specify the `bin_test` tag to
signal to the test suite that the Nomad binary is ready.

The `bin_test` tag was added to help prevent running the tests against an
arbitrary binary accidentally, rather than a specific binary you want to test.

## A typical testing session

If you've found a broken test and want to quickly iterate on it, first try something
simple, e.g.:

   $ go test -v -tags nomad_test -run TestAPI_OperatorAutopilotGetSetConfiguration ./api

If that works, then the test does not require you to first set up a test Nomad binary.
If not, you will see a message similar to:

```
        server_off.go:10: This test requires the bin_test tag to be set.
                        ==> Tip: Ensure you've built your latest changes to nomad by
                            running 'make dev' first, then run this test again with
                            "-tags bin_test"
```

You can now follow that advice to run your test against the relevant binary.

You'll now be running the following command each time...


   $ make dev && go test -v -tags nomad_test,bin_test -run TestAPI_OperatorAutopilotGetSetConfiguration ./api
