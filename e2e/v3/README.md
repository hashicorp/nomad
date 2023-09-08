# e2e | v3

(!) These packages are experimental and breaking changes will be made. Also,
expect bugs. Like a lot of bugs. Especially on non-happy paths.

The `e2e/v3/` set of packages provide utilities for creating Nomad e2e tests in
a way that is convenient, reliable, and debuggable.

- `v3/cluster3` - establish and verify the state of the cluster
- `v3/jobs3` - manage nomad jobs and wait for deployments, etc.
- `v3/namespaces3` - manage nomad namespaces
- `v3/util3` - helper methods specific to the `v3` utilities

## Examples

#### simple

The simplest example, where we expect a cluster with a leader and at least one
Linux client in a ready state. The test case will submit the `sleep.hcl` job,
wait for the deployment to become succesfull (or fail / timeout), then cleanup
the job.

```go
func TestExample(t *testing.T) {
    cluster3.Establish(t,
        cluster3.Leader(),
        cluster3.LinuxClients(1),
    )

    t.Run("testSleep", testSleep)
}

func testSleep(t *testing.T) {
    cleanup := jobs3.Submit(t, "./input/sleep.hcl")
    t.Cleanup(cleanup)
}
```
