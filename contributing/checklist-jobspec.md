# New `jobspec` Entry Checklist

## Code

* [ ] Consider similar features in Consul, Kubernetes, and other tools. Is
  there prior art we should match? Terminology, structure, etc?
* [ ] Parse in `jobspec/parse.go`
* [ ] Test in `jobspec/parse_test.go` (preferably with a
  `jobspec/text-fixtures/<feature>.hcl` test file)
* [ ] Add structs/fields to `api/` package
  * structs usually have Canonicalize, Copy, and Merge methods
  * New fields should be added to existing Canonicalize, Copy, and Merge
    methods
  * Test the struct/field via all methods mentioned above
* [ ] Add structs/fields to `nomad/structs` package
  * Validation happens in this package and must be implemented
  * Implement other methods and tests from `api/` package
  * Note that analogous struct field names should match with `api/` package
* [ ] Add conversion between `api/` and `nomad/structs` in `command/agent/job_endpoint.go`
* [ ] Add check for job diff in `nomad/structs/diff.go`
  * Note that fields must be listed in alphabetical order in `FieldDiff` slices in `nomad/structs/diff_test.go`
* [ ] Test conversion

## Docs

* [ ] Changelog
* [ ] Jobspec entry https://www.nomadproject.io/docs/job-specification/index.html
* [ ] Jobspec sidebar entry https://github.com/hashicorp/nomad/blob/master/website/source/layouts/docs.erb
* [ ] Job JSON API entry https://www.nomadproject.io/api/json-jobs.html
* [ ] Sample Response output in API https://www.nomadproject.io/api/jobs.html
* [ ] Consider if it needs a guide https://www.nomadproject.io/guides/index.html
