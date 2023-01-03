# Writing Tests

The Nomad repository strives to maintain comprehensive unit test coverage. Any new
features, bug fixes, or refactoring should include additional or updated test cases
demonstrating correct functionality.

Each unit test should meet a few criteria:

- Use [github.com/shoenig/test](https://github.com/shoenig/test)
  - Prefer using `must.*`` functions
  - Use `test.*`` functions when cleanup must happen, etc
  - Feel free to refactor testify tests; but consider separate commits / PRs

- Undo any changes to the environment
  - Set environment variables must be unset (use `t.Setenv`)
  - Scratch files/dirs must be removed (use `t.TempDir`)

- Able to run in parallel
  - All package level `Test*` functions should start with `ci.Parallel`
  - Always use dynamic scratch dirs, files
  - Always get ports via `ci.PortAllocator.Grab()`

- Log control
  - Logging must go through the `testing.T` (use `helper/testlog.HCLogger`)
  - Avoid excessive logging in test cases - prefer failure messages
    - Annotate failures with `must.Sprint\f` post-scripts

## API tests

Testing in the `api` package requires an already-built Nomad
binary. If you're writing `api` tests, you'll need to build a Nomad
binary (ex. with `make dev`) that includes any changes your API
exercises.


# CI Plumbing

See [ci/README.md] for details on how the [Core CI Tests](https://github.com/hashicorp/nomad/actions/workflows/test-core.yaml)
Github Action works.
