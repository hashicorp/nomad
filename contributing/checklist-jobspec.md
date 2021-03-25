# New `jobspec` Entry Checklist

## Code

* [ ] Consider similar features in Consul, Kubernetes, and other tools. Is there prior art we should match? Terminology, structure, etc?
* [ ] Add structs/fields to `api/` package
  * `api/` structs usually have Canonicalize and Copy methods
  * New fields should be added to existing Canonicalize, Copy methods
  * Test the structs/fields via methods mentioned above
* [ ] Add structs/fields to `nomad/structs` package
  * `structs/` structs usually have Copy, Equals, and Validate methods
  * Validation happens in this package and _must_ be implemented
  * Note that analogous struct field names should match with `api/` package
  * Test the structs/fields via methods mentioned above
  * Implement and test other logical methods
* [ ] Add conversion between `api/` and `nomad/structs` in `command/agent/job_endpoint.go`
  * Add test for conversion
* [ ] Implement diff logic for new structs/fields in `nomad/structs/diff.go`
  * Note that fields must be listed in alphabetical order in `FieldDiff` slices in `nomad/structs/diff_test.go`
  * Add test for diff of new structs/fields
* [ ] Add change detection for new structs/feilds in `scheduler/util.go/tasksUpdated`
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
* [ ] Jobspec entry https://www.nomadproject.io/docs/job-specification/index.html
* [ ] Jobspec sidebar entry https://github.com/hashicorp/nomad/blob/main/website/data/docs-navigation.js
* [ ] Job JSON API entry https://www.nomadproject.io/api/json-jobs.html
* [ ] Sample Response output in API https://www.nomadproject.io/api/jobs.html
* [ ] Consider if it needs a guide https://www.nomadproject.io/guides/index.html
