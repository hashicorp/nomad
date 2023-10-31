# New `jobspec` Entry Checklist

## Code

* [x] Consider similar features in Consul, Kubernetes, and other tools. Is there prior art we should match? Terminology, structure, etc?
* [t] Add structs/fields to `api/` package
  * `api/` structs usually have Canonicalize and Copy methods
  * New fields should be added to existing Canonicalize, Copy methods
  * Test the structs/fields via methods mentioned above
* [t] Add structs/fields to `nomad/structs` package
  * `structs/` structs usually have Copy, Equal, and Validate methods
    * `Validate` methods in this package _must_ be implemented
    * `Equal` methods are used when comparing one job to another (e.g. did this thing get updated?)
    * `Copy` methods ensure modifications do not modify the copy of a job in the state store
      * Use `slices.CloneFunc` and `maps.CloneFunc` to ensure creation of deep copies
  * Note that analogous struct field names should match with `api/` package
  * Test the structs/fields via methods mentioned above
  * Implement and test other logical methods
* [t] Add conversion between `api/` and `nomad/structs/` in `command/agent/job_endpoint.go`
  * Add test for conversion
* [x] Determine JSON encoding strategy for responses from RPC (see "JSON Encoding" below)
  * [x] Write `nomad/structs/` to `api/` conversions if necessary and write tests
* [t] Implement diff logic for new structs/fields in `nomad/structs/diff.go`
  * Note that fields must be listed in alphabetical order in `FieldDiff` slices in `nomad/structs/diff_test.go`
  * Add test for diff of new structs/fields
* [x] Add change detection for new structs/fields in `scheduler/util.go/tasksUpdated`
  * Might be covered by `.Equals` but might not be, check.
  * Should return true if the task must be replaced as a result of the change.

## HCL1 (deprecated)

New jobspec entries should only be added to `jobspec2`. It makes use of HCL2
and the `api` package for automatic parsing. Before, additional parsing was
required in the original `jobspec` package.

* [ ] ~~Parse in `jobspec/parse.go`~~ (HCL1 only)
* [ ] ~~Test in `jobspec/parse_test.go` (preferably with a
  `jobspec/text-fixtures/<feature>.hcl` test file)~~ (HCL1 only)

## Docs

* [ ] Changelog
* [x] Jobspec entry https://developer.hashicorp.com/nomad/docs/job-specification/index.html
* [x] Jobspec sidebar entry https://github.com/hashicorp/nomad/blob/main/website/data/docs-navigation.js
* [ ] Job JSON API entry https://developer.hashicorp.com/nomad/api/json-jobs.html
* [ ] Sample Response output in API https://developer.hashicorp.com/nomad/api/jobs.html
* [ ] Consider if it needs a guide https://developer.hashicorp.com/nomad/guides/index.html

## JSON Encoding

As a general rule, HTTP endpoints (under `command/agent/`) will make RPC calls that return structs belonging to 
`nomad/structs/`. These handlers ultimately return an object that is encoded by the Nomad HTTP server. The encoded form
needs to match the Nomad API; specifically, it should have the form of the corresponding struct from `api/`. There are
a few ways that this can be accomplished:
* directly return the struct from the RPC call, if it has the same shape as the corresponding struct in `api/`. 
  This is convenient when possible, resulting in the least work for the developer. 
  Examples of this approach include [GET `/v1/evaluation/:id`](https://github.com/hashicorp/nomad/blob/v1.0.
  0/command/agent/eval_endpoint.go#L88).
* convert the struct from the RPC call to the appropriate `api/` struct.
  This approach is the most developer effort, but it does have a strong guarantee that the HTTP response matches the 
  API, due to the explicit conversion (assuming proper implementation, which requires tests).
  Examples of this approach include [GET `/v1/volume/csi/:id`](https://github.com/hashicorp/nomad/blob/v1.0.0/command/agent/csi_endpoint.go#L108)
* convert to an intermediate struct with the same shape as the `api/` struct.
  This approach strikes a balance between the former two approaches. 
  This conversion can be performed in-situ in the agent HTTP handler, as long as the conversion doesn't need to 
  appear in other handlers. 
  Otherwise, it is possible to register an extension on the JSON encoding used by the HTTP agent; these extensions
  can be put in `nomad/jsonhandles/extensions.go`.
