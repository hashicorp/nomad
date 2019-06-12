# New `jobspec` Entry Checklist

## Code

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
* [ ] Add conversion between `api/` and `nomad/structs` in `command/agent/job_endpoint.go`
* [ ] Test conversion

## Docs

* [ ] Changelog
* [ ] Jobspec entry https://www.nomadproject.io/docs/job-specification/index.html
* [ ] Job JSON API entry https://www.nomadproject.io/api/json-jobs.html
* [ ] Sample Response output in API https://www.nomadproject.io/api/jobs.html
* [ ] Consider if it needs a guide https://www.nomadproject.io/guides/index.html
