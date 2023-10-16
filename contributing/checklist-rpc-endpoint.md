# New/Updated RPC Endpoint Checklist

Prefer adding a new message to changing any existing RPC messages.

## Code

* [ ] `Request` struct and `*RequestType` constant in
      `nomad/structs/structs.go`. Append the constant, old constant
      values must remain unchanged. Just add the request type to this file, all other resource definitions
      must be on their own separate file.

* [ ] In `nomad/fsm.go`, add a dispatch case to the switch statement in `(n *nomadFSM) Apply`
  * `*nomadFSM` method to decode the request and call the state method

* [ ] State method for modifying objects in a `Txn` in the `state` package, located in
      `nomad/state/`. Every new resource should have its own file and test file, named using the convention
      `nomad/state/state_store_[resource].go` and `nomad/state/state_store_[resource]_test.go`

* [ ] Handler for the request in `nomad/foo_endpoint.go`
  * RPCs are resolved by matching the method name for bound structs
	[net/rpc](https://golang.org/pkg/net/rpc/)
  * Register any new RPC structs in `nomad/server.go`
  * Authentication:
    * For RPCs that support HTTP APIs, call `Authenticate` before forwarding. Return any error after frowarding, and call `ResolveACL` to get an ACL to check.
    * For RPCs that support client-to-server RPCs _only_, use `AuthenticateClientOnly` before forwarding. Check the `AllowClientOp` ACL after forwarding.
    * For RPCs that support server-to-server RPCs _only_, use `AuthenticateServerOnly` before forwarding. Check the `AllowServerOp` ACL _before_ forwarding.
  * Authorization:
    * Use `ResolveACL` to turn the authenticated request into an ACL to check.
    * For Update/Get/Delete RPCs, check ACLs before hitting the state store.
    * For List RPCs, use ACLs as a filter on the query.
    * _Never_ check that the ACL object is `nil` to bypass authorization. The
      authorization methods in `acl/acl.go` should already handle `nil` ACL
      objects correctly (by rejecting them).

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

* [ ] If implementing a Client RPC...
  * Use `QueryOptions` instead of `WriteRequest` in the Request struct as
    `WriteRequest` is only for *Raft* writes.
  * Set `QueryOptions.AllowStale = true` in the *Server* RPC forwarder to avoid
    an infinite loop between leaders and followers when a Client RPC is
    forwarded through a follower. See
    https://github.com/hashicorp/nomad/issues/16517

## Docs

* [ ] Changelog
* [ ] [Metrics](https://www.nomadproject.io/docs/operations/metrics#server-metrics)
* [ ] [API docs](https://www.nomadproject.io/api-docs) for RPCs with an HTTP endpoint, include ACLs, params, and example response body.
