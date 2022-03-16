# Writing Tests

The Nomad repository strives to maintain comprehensive unit test coverage. Any new
features, bug fixes, or refactoring should include additional or updated test cases
demonstrating correct functionality.

Each unit test should meet a few criteria:

- Use testify
  - Prefer using require.* functions

- Undo any changes to the environment
  - Set environment variables must be unset
  - Scratch files/dirs must be removed (use t.TempDir)
  - Consumed ports must be freed (e.g. TestServer.Cleanup, freeport.Return)

- Able to run in parallel
  - All package level Test* functions should start with ci.Parallel
  - Always use dynamic scratch dirs, files
  - Always get ports from helpers (TestServer, TestClient, TestAgent, freeport.Get)

- Log control
  - Logging must go through the testing.T (use helper/testlog.HCLogger)
  - Avoid excessive logging in test cases - prefer failure messages