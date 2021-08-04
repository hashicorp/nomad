#Nomad OpenAPI Specification Generator

This package generates an OpenAPI specification for the Nomad HTTP API using
[kin-openapi](https://github.com/getkin/kin-openapi) and a homegrown configuration
model defined in `model.go`. Longer term efforts of generating configuration
from AST parsing is underway, but this manual stop gap measure is both expedient
and pragmatic in terms of supporting the community more immediately.

## Implementation Overview

The `kin-openapi` project is a fantastic and actively maintained project that
provides a fully functioning model for OpenAPI Specifications. It is able to
load an in-memory model for existing specifications files. While it does provide 
functionality to generate schema for `go` structs using reflection, it does not contain
a mechanism to generate a full specification from extant source. In other words,
it does not fully support the `code first` paradigm. This project provides a solution
for documenting an existing API that was written without consideration for generating
an OpenAPI specification.

### Out of Scope
 
This implementation stops short of trying to fully support every aspect of the
OpenAPI specification, but instead only provides facilities required by the Nomad
API. For example, as of this time there is no support for `OneOf` or `AnyOf`
relative to responses, nor will there be unless during the configuration implementation
we encounter an endpoint that requires that support.

Also, worth mentioning is that the current plan for the Event Stream API is to
document it separately using the AsyncAPI specification. It may be possible to use the OpenAPI
specification for that endpoint, but we feel the AsyncAPI spec will provide a better
UX, and better supports our long term goals for that feature set. Members of the
community are welcome to extend this package to support the Event Stream API as
a stop gap measure, or if it better suits their needs, but at the time of this
writing, the plan is to support that endpoint with a different solution. As always,
community input on this issue is _welcome_ and will _most definitely_ be taken into
consideration.

### `model.go`

This file contains abstractions for documenting an extant API as a set of configurations.
By convention, the struct names were selected to match their corresponding counterparts
in `kin-openapi` to ease the cognitive burden when writing the configuration
transformation code. Examples include `Operation` and `Response`.

Some elements of the configuration model broke from that convention for the sake
of clarity or simplicity. For example, the HTTP protocol, and thus the OpenAPI specification allows
for headers as both input parameters, and response output. So the `ResponseHeader`
struct was named as such to clarify its use case. Another example of the broken
convention, is that we combined the OpenAPI `Path` and `PathItem` types into a
single `Path` struct. In the OpenAPI specification, the `Paths` object is essentially
a map of path templates to `PathItems` objects. We didn't see any value in
the extra layer of abstraction, so we have a `Path` struct with a `Template` field,
and an `Operations` field that is the set of operations supported at that path.

Also, worth noting, is that we occasionally added, and reserve the right to add,
additional fields for our own purposes. For example, the `Operation` struct has
a `Handler` field that has no counterpart in the OpenAPI specification. Because
of the age of the Nomad HTTP API, and the fact that it currently uses original
techniques for path and path parameter handling, handler <==> path resolution
using AST parsing has not been as straight forward as we would like. So to mitigate
that complexity, we added the `Handler` field to the `Operation` struct. Since we
were already having to look up individual handlers for each endpoint in order to
document them, it made sense on multiple levels to include the handler name in
the configuration. This helps users find the handler more quickly, and also could
supplement our future AST based efforts.

### `v1api.go`

This file is the root of the configuration for version 1 of the Nomad HTTP API. It contains:

- The set of all `Parameters`
- The set of all `ResponseHeaders`
- The set of all shared `Responses`
- A number of variables that group common `Parameters`, `RespoonseHeaders`, and
  `Responses` for ease of reuse
- A set of factory methods to reduce the amount of boilerplate required to configure
  endpoints.
- A set of helper methods that ensure API framework level guarantees are injected
  (e.g. `getResponses`)

This file also contains the root `GetPaths` function. Its job is to invoke the path
configuration logic for each area of the API, and aggregates the results into a
single set of paths. In order to ease PRs and mitigate the potential for merge
conflicts, specific areas of the API configuration have been grouped by their path
or path template and a separate file has been created for each area (e.g. `jobs.go`).
Each of those files contains a helper function that returns a `[]*Path` for the
paths in that area.

By convention, a tag for each area is defined and configured for each operation within
an area of the API. This has a benefit in that the generated client groups sets of
functionality by tag, which aids in discoverability of endpoints and a reduction
of cognitive load for client consumers. **NOTE:** this may vary by client generator.

### `specbuilder.go`

This file contains both the `Spec` and `SpecBuilder` structs. `Spec` is a thin
wrapper over a `kin-openapi` specification model. It provides a `ValidationContext`
field to provide to the `kin-openapi` validator, and some helper methods for converting
the in-memory spec into either a byte slice or a YAML string.

The `SpecBuilder` struct is an implementation of the builder pattern and is
responsible for building a valid specification from the configuration provided by
`v1api.GetPaths`. The spec builder calls a set of helper functions that build each
field of the specification. Most functions, such as `buildInfo`, statically configure
their relative field. The exception is the `buildComponentsAndPaths` function.

The `Components` object graph in the `kin-openapi` model is what contains all the
reusable elements of the specification such as `Parameters`, `Responses`, `RequestBodies`,
and `Schemas`. `RequestBodies` and `Schemas` are JSON schema representations of
the `go` structs that are either posted to or returned from the API. The `buildComponentsAndPaths`
function iterates over the paths returned from `v1api.GetPaths`, and dynamically
builds the specification model. It does this by inspecting each path in a specific
and meaningful order in order to ensure that the `Components` members each path
depends on have been added to the object graph, before adding the `PathItem`. It
calls out to a series of adapter functions that adapt the read the passed
configuration, and adapts it to the desired `kin-openapi` model. Finally, it ensures
that all components of the in-memory spec that are actually references to shared
components have a valid reference path.

Once the `kin-openapi` graph has been built by the `SpecBuilder`, clients can
call `ToBytes` or `ToYAML` on the resulting `Spec` and they should have a valid
OpenAPI specification.

### `generator.go`

This file is forward looking at this time. The ultimate goal is to have the specification
and associated test client generated at CI time using `go:generator`, thus ensuring
the specification is always in sync with at least the `nomad/api` structs.

## Generating a specification and test client

To manually generate a schema, the easiest way is to run `TestGenSchema` in the
`generator_test.go` file. If you want to specify an output location, set the `outputPath`
field on the generator instance in the test to your desired location. For example,
there is a currently commented line in the test that can be used to generate the
specification in the location that the `make openapi` command will use to update
the generated test client.

To update the generated test client with your changes, run `make openapi` from
the root of the Nomad project.

## Contributing

The following are some guidelines for internal and community contributors that
want to help with the generator spec implementation.

### Read everything above

If you skipped strait to the contributor section, please go back and read the
explanation of implementation and generation sections.

### Create an Issue

If you want to work on a section of the API, please create an issue and use the
following checklist.

- Please state which section of the API you plan to work on, so that we do not
  duplicate effort across contributors, and in case someone is already working
  on that area.
- If you plan to work on more than one area, please create separate Issues and
  submit separate PRs for each area.
- Please add @DerekStrickland as a reviewer for your PR

### Conventions

Please adhere to the conventions documented above or implemented in the code already.
If you want to suggest new or different conventions, please do! Raising and issue
and mentioning @DerekStrickland is a great way to get a timely response.

### Configuring an Endpoint

Each API area has a file with a `get{AreaName}Paths` function that returns a slice
of `Path` pointers. For example, here is the opening of the `jobs.go` file's
`getJobsPaths` function.

```go
func (v *v1api) getJobPaths() []*Path {
	tags := []string{"Jobs"}

	return []*Path{
            {
                Template: "/job/{jobName}/plan",
                Operations: []*Operation{
                    newOperation(http.MethodPost, "jobPlan", tags, "PostJobPlan",
                      newRequestBody(objectSchema, api.JobPlanRequest{}),
                      append(queryOptions, &JobNameParam),
                      newResponseConfig(200, objectSchema, api.JobPlanResponse{}, queryMeta, "PostJobPlanResponse"),
                ),
              },
            },
```

If the path you want to configure is not present yet, then you can copy this or any
existing path config, add it to the slice, and change it accordingly. The remainder
of this section will explain the configuration options in detail.

#### Path templates

The `Template` field specifies the route to the endpoint _after_ the `v1` route segment.
It must start with a forward slash, and may or may not include templated path parameters.
In this example, the path `Template` contains the job name as part of the path
and is denoted with curly braces (e.g. `{jobName}`). Since this template includes a parameter,
a `Parameter` struct that represents it, but be defined and added to each `Operation`.
We'll discuss more about parameters and operations below.

Not all `Path` templates contain path parameters. If you are configuring a `Path` that does not
have path parameters, the following is completely valid syntax.

```go
    Template: "/jobs",
```

#### Operations

A single path may or may not support multiple operations based on HTTP method.
You can add up to 3 operations per path. Currently, by convention, we support `GET`,
`POST`, and `DELETE`. While it is true, that the `POST` endpoint handlers will
typically support `PUT`, our online documentation only documents our support for
`POST`. For version 1 of the HTTP API, we are unlikely to change that behavior.
However, on a go forward basis, we plan to standardize on `POST`. To encourage
the community to develop that habit now, and to avoid unnecessary duplication in
the configuration, we ask that you standardize on `POST` in this module.

Notice that the code example above uses the `newOperation` helper function. This
significantly reduces the boilerplate lines of code for configuration, so please use
this function. The function accepts the following arguments:

```go
method string, handler string, tags []string, operationId string, requestBody *RequestBody, params []*Parameter, responses ...*ResponseConfig
```

- _Method_ is the HTTP method the operation supports. To avoid human error, please
  use the `net/http` package constants.
- Handler is the name of the function that handles this operation. Handlers are
  defined in the `nomad/command/agent`. To find the handler for a given route, start
  with the `http.go` file in that package. Next, locate the route in the `registerHandlers`
  function. Routes that have a path parameter will call an intermediate function
  that handles the path parameter and also inspects any section of the route after
  path parameter. That means you won't find `/v1/jobs/{jobName}/plan` in that function,
  but you will find `/v1/jobs` that gets handled by `JobSpecificRequest`. `JobSpecificRequest`
  will be responsible for extracting the path parameters, and then routing the
  request to a handler based on the remainder of the route.
- _Tags_ is an array of strings indicating which area(s) of the generated client
  should include this `Path`. The value to pass here should already be defined for
  you, so please just pass the `tags` variable that has already been defined.
- _OperationId_ is the globally unique name of the operation that will be included
  in the spec. Be careful defining this, as duplicates can cause errors during spec generation.
  By convention, the operation id should be in the form of `{HttpMethod}{EntityName}`
  (e.g.`GetJobPlan`). If the `Operation` you are configuring returns a list of
  entities, please use the plural form of the entity name (e.g. `GetJobs`).
- _RequestBody_ defines the struct the handler expects in the
  request body. Notice, this also uses a helper function to reduce boilerplate.
  You will need to inspect the handler function to see if it expects a request body.
  This is typically easy to detect because the handler function will set a local
  variable equal to the result of a call to `decode` which unmarshalls the incoming
  JSON from the request body. The helper function requires two pieces of information:
  the schema type and an instance of the struct. If the request body contains a
  singular JSON object, pass `objectSchema`. If the request body contains an array
  of JSON objects, pass `arraySchema`. If no request body is expected, pass `nil`
  instead of calling the `newRequestBody` function.
- _Params_ is the list of parameters this `Operation` expects. If no parameters
  are expected, pass `nil`. Most, if not all query operations support a common
  set of parameters that have been made available to you as the `queryOptions`
  variable. Similarly, there is a variable named `writeOptions` that contains the
  set of parameters expected by operations that mutate state. If you need to add
  additional parameters, such as path parameters, you can do so with `append`
  (e.g. `append(queryOptions, &JobNameParam)`).
- _Responses_ is a varArgs of `ResponseConfigs` that represent the responses
  clients can expect from the `Operation`. This also uses a helper function to
  reduce boilerplate. Since this last argument is a varArg, you can call this
  function as many times as you need. Internally, this helper function ensures
  that common responses that could happen for any `Operation`, such as a `401`
  violation that are returned at a framework level are included. The helper function
  requires you to define a status code (e.g. 200), the schemaType of the response
  content (i.e. `objectSchema`, `arraySchema`), an instance of the struct that
  will be returned or nil if no struct is returned, an array of `ResponseHeader`
  structs or `nil` if none, and a globally unique name for the response. Like
  the `OperationId`, this is name will be added to the specification and the last
  one wins. By convention, the response name should be in the form of `{OperationId}Response`
  (e.g. `GetJobsResponse`).

** A note on request bodies and response content**

Nearly all the time, you will see that the calls to `decode` in the handler to unmarshall
incoming request bodies will create an instance of a struct from the `nomad/nomad/structs`
package, and return a struct from the `nomad/api` package. The `nomad/nomad/structs`
package contains all the structs that our RPC server accepts and returns, but not
all the fields on those structs is appropriate for the HTTP API to return. To address that,
we have been working creating an API version in the `nomad/api` package. For your
request body and your response models, you should first attempt to define the model
as the version of the struct from the `nomad/api` package. If one does not exist,
you can use the `nomad/nomad/structs` version for now, but if you can, please point
out in your PR that you had to do this. If you are feeling particularly kind, you
could also raise an issue so that we can address this inconsistency. We are working
on automation to prevent this in the future, but for now, we have to be mindful.

You may also see scenarios, where the handler returns one particular field from
the struct it gets back from the RPC server. The same `api` vs. `structs` package
rule should apply in these cases, in that, there should be a version of that struct
in the `nomad/api` package, and, ideally, we should try to return that.

### Writing Tests

Once you have added your new configuration, you should be able to generate an
updated spec, as detailed above, and then update the test client, also detailed
above. Once the client has been updated with your changes, you can add tests for
the client to the existing unit tests for each endpoint operation. See `TestHTTP_JobsList`
in the `nomad/command/agent/job_endpoint_test.go` file for an example. Please review
the assertions the existing tests make validate the response, and then repeat them
against the response you get from the test client.

Currently, there is a small set of helper functions in `/nomad/testutils/openapi`
that can be used to reduce boilerplate. Feel free to use and add to them.

## Notes

The `buildComponentsAndPaths` function is in need of a code review for
simplification. For example, it should be possible to handle reference path resolution
on the first pass. Expect this to change/improve. Ideally, you should not have to
make adjustments to this code, but it's early, and there undoubtedly cases that
will be encountered that are not fully handled. If you find yourself needing to
modify the `SpecBuilder` or anything in the `model.go` file, please reach out to
@DerekStrickland to collaborate.