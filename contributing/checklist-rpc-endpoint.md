# New RPC Endpoint Checklist

Prefer adding a new message to changing any existing RPC messages.

## Code

* [ ] `Request` struct and `*RequestType` constant in
      `nomad/structs/structs.go`. Append the constant, old constant
      values must remain unchanged

* [ ] In `nomad/fsm.go`, add a dispatch case to the switch statement in `(n *nomadFSM) Apply`
  * `*nomadFSM` method to decode the request and call the state method

* [ ] State method for modifying objects in a `Txn` in `nomad/state/state_store.go`
  * `nomad/state/state_store_test.go`

* [ ] Handler for the request in `nomad/foo_endpoint.go`
  * RPCs are resolved by matching the method name for bound structs
	[net/rpc](https://golang.org/pkg/net/rpc/)
  * Check ACLs for security, list endpoints filter by ACL
  * Register new RPC struct in `nomad/server.go`
  * Check ACLs to enforce security

* Wrapper for the HTTP request in `command/agent/foo_endpoint.go`
  * Backwards compatibility requires a new endpoint, an upgraded
    client or server may be forwarding this request to an old server,
    without support for the new RPC
  * RPCs triggered by an internal process may not need support
  * Check ACLs as an optimization

* [ ] `nomad/core_sched.go` sends many RPCs
  * `ServersMeetMinimumVersion` asserts that the server cluster is
    upgraded, so use this to gaurd sending the new RPC, else send the old RPC
  * Version must match the actual release version!

## Docs

* [ ] Changelog
* [ ] [Metrics](https://www.nomadproject.io/docs/operations/metrics#server-metrics)
* [ ] [API docs](https://www.nomadproject.io/api-docs) for RPCs with an HTTP endpoint, include ACLs, params, and example response body.
