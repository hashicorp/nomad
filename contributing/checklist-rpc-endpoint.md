# New/Updated RPC Endpoint Checklist

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

* [ ] Wrapper for the HTTP request in `command/agent/foo_endpoint.go`
  * Backwards compatibility requires a new endpoint, an upgraded
    client or server may be forwarding this request to an old server,
    without support for the new RPC
  * RPCs triggered by an internal process may not need support
  * Check ACLs as an optimization

* [ ] Endpoint added/updated in the [`nomad-openapi`](https://github.com/hashicorp/nomad-openapi) repository.
  * New endpoints will need to be configured in that repository's `generator` package.
  * Updated endpoints may require the `generator` configuration to change, especially if parameters or headers change.
  * If the accepted or returned `struct` schema changes, the Nomad version references in `generator/go.mod` will need
    to be updated. Once the version is updated, regenerate the spec and all all clients so that the new schema is
    reflected in the spec and thus the generated models.
  * If `QueryOptions`, `QueryMeta`, `WriteOptions`, or `WriteMeta` change, the `v1` framework will need to updated to
    support the change.

* [ ] `nomad/core_sched.go` sends many RPCs
  * `ServersMeetMinimumVersion` asserts that the server cluster is
    upgraded, so use this to guard sending the new RPC, else send the old RPC
  * Version must match the actual release version!

## Docs

* [ ] Changelog
* [ ] [Metrics](https://www.nomadproject.io/docs/operations/metrics#server-metrics)
* [ ] [API docs](https://www.nomadproject.io/api-docs) for RPCs with an HTTP endpoint, include ACLs, params, and example response body.
