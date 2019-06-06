# New `jobspec` Entry Checklist

## Code

1. [ ] Parse in `jobspec/parse.go`
2. [ ] Test in `jobspec/parse_test.go` (preferably with a
  `jobspec/text-fixtures/<feature>.hcl` test file)
3. [ ] Add structs/fields to `api/` package
  * structs usually have Canonicalize, Copy, and Merge methods
  * New fields should be added to existing Canonicalize, Copy, and Merge
    methods
  * Test the struct/field via all methods mentioned above
4. [ ] Add structs/fields to `nomad/structs` package
  * Validation happens in this package and must be implemented
  * Implement other methods and tests from `api/` package
5. [ ] Add conversion between `api/` and `nomad/structs` in `command/agent/job_endpoint.go`
6. [ ] Test conversion

## Docs

1. [ ] Changelog
2. [ ] Jobspec entry https://www.nomadproject.io/docs/job-specification/index.html
3. [ ] Job JSON API entry https://www.nomadproject.io/api/json-jobs.html
4. [ ] Sample Response output in API https://www.nomadproject.io/api/jobs.html
5. [ ] Consider if it needs a guide https://www.nomadproject.io/guides/index.html
