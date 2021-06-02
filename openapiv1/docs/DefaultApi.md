# \DefaultApi

All URIs are relative to *http://127.0.0.1:4646/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**EvaluateJob**](DefaultApi.md#EvaluateJob) | **Put** /job/{jobName}/evaluate | Creates a new evaluation for the given job. This can be used to force run the scheduling logic if necessary. See https://www.nomadproject.io/api-docs/jobs#create-job-evaluation.
[**GetJobEvaluations**](DefaultApi.md#GetJobEvaluations) | **Get** /job/{jobName}/evaluations | Gets information about a single job&#39;s evaluations. See [documentation](https://www.nomadproject.io/api-docs/evaluations#list-evaluations). 
[**GetJobs**](DefaultApi.md#GetJobs) | **Get** /jobs | List all known jobs registered with Nomad. See https://www.nomadproject.io/api-docs/jobs#list-jobs.
[**ParseJobSpec**](DefaultApi.md#ParseJobSpec) | **Put** /jobs/parse | Parses a HCL jobspec and produce the equivalent JSON encoded job. See https://www.nomadproject.io/api-docs/jobs#parse-job.
[**PutJobForceRequest**](DefaultApi.md#PutJobForceRequest) | **Put** /job/{jobName}/periodic/force | Forces a new instance of the periodic job. A new instance will be created even if it violates the job&#39;s prohibit_overlap settings. As such, this should be only used to immediately run a periodic job. See [documentation](https://www.nomadproject.io/docs/commands/job/periodic-force).



## EvaluateJob

> JobRegisterResponse EvaluateJob(ctx, jobName).JobEvaluateRequest(jobEvaluateRequest).Execute()

Creates a new evaluation for the given job. This can be used to force run the scheduling logic if necessary. See https://www.nomadproject.io/api-docs/jobs#create-job-evaluation.

### Example

```go
package main

import (
    "context"
    "fmt"
    "os"
    openapiclient "./openapi"
)

func main() {
    jobName := "jobName_example" // string | The job identifier.
    jobEvaluateRequest := *openapiclient.NewJobEvaluateRequest() // JobEvaluateRequest | 

    configuration := openapiclient.NewConfiguration()
    api_client := openapiclient.NewAPIClient(configuration)
    resp, r, err := api_client.DefaultApi.EvaluateJob(context.Background(), jobName).JobEvaluateRequest(jobEvaluateRequest).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `DefaultApi.EvaluateJob``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `EvaluateJob`: JobRegisterResponse
    fmt.Fprintf(os.Stdout, "Response from `DefaultApi.EvaluateJob`: %v\n", resp)
}
```

### Path Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**jobName** | **string** | The job identifier. | 

### Other Parameters

Other parameters are passed through a pointer to a apiEvaluateJobRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **jobEvaluateRequest** | [**JobEvaluateRequest**](JobEvaluateRequest.md) |  | 

### Return type

[**JobRegisterResponse**](JobRegisterResponse.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetJobEvaluations

> []Evaluation GetJobEvaluations(ctx, jobName).Region(region).Stale(stale).Prefix(prefix).Namespace(namespace).PerPage(perPage).NextToken(nextToken).Execute()

Gets information about a single job's evaluations. See [documentation](https://www.nomadproject.io/api-docs/evaluations#list-evaluations). 

### Example

```go
package main

import (
    "context"
    "fmt"
    "os"
    openapiclient "./openapi"
)

func main() {
    jobName := "jobName_example" // string | The job identifier.
    region := "region_example" // string | Filters results based on the specified region (optional)
    stale := "stale_example" // string | If present, results will include stale reads (optional)
    prefix := "prefix_example" // string | Constrains results to jobs that start with the defined prefix (optional)
    namespace := "namespace_example" // string | Filters results based on the specified namespace (optional)
    perPage := int32(56) // int32 | Maximum number of results to return (optional)
    nextToken := "nextToken_example" // string | Indicates where to start paging for queries that support pagination (optional)

    configuration := openapiclient.NewConfiguration()
    api_client := openapiclient.NewAPIClient(configuration)
    resp, r, err := api_client.DefaultApi.GetJobEvaluations(context.Background(), jobName).Region(region).Stale(stale).Prefix(prefix).Namespace(namespace).PerPage(perPage).NextToken(nextToken).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `DefaultApi.GetJobEvaluations``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `GetJobEvaluations`: []Evaluation
    fmt.Fprintf(os.Stdout, "Response from `DefaultApi.GetJobEvaluations`: %v\n", resp)
}
```

### Path Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**jobName** | **string** | The job identifier. | 

### Other Parameters

Other parameters are passed through a pointer to a apiGetJobEvaluationsRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **region** | **string** | Filters results based on the specified region | 
 **stale** | **string** | If present, results will include stale reads | 
 **prefix** | **string** | Constrains results to jobs that start with the defined prefix | 
 **namespace** | **string** | Filters results based on the specified namespace | 
 **perPage** | **int32** | Maximum number of results to return | 
 **nextToken** | **string** | Indicates where to start paging for queries that support pagination | 

### Return type

[**[]Evaluation**](Evaluation.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetJobs

> []JobListStub GetJobs(ctx).Region(region).Stale(stale).Prefix(prefix).Namespace(namespace).PerPage(perPage).NextToken(nextToken).Execute()

List all known jobs registered with Nomad. See https://www.nomadproject.io/api-docs/jobs#list-jobs.

### Example

```go
package main

import (
    "context"
    "fmt"
    "os"
    openapiclient "./openapi"
)

func main() {
    region := "region_example" // string | Filters results based on the specified region (optional)
    stale := "stale_example" // string | If present, results will include stale reads (optional)
    prefix := "prefix_example" // string | Constrains results to jobs that start with the defined prefix (optional)
    namespace := "namespace_example" // string | Filters results based on the specified namespace (optional)
    perPage := int32(56) // int32 | Maximum number of results to return (optional)
    nextToken := "nextToken_example" // string | Indicates where to start paging for queries that support pagination (optional)

    configuration := openapiclient.NewConfiguration()
    api_client := openapiclient.NewAPIClient(configuration)
    resp, r, err := api_client.DefaultApi.GetJobs(context.Background()).Region(region).Stale(stale).Prefix(prefix).Namespace(namespace).PerPage(perPage).NextToken(nextToken).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `DefaultApi.GetJobs``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `GetJobs`: []JobListStub
    fmt.Fprintf(os.Stdout, "Response from `DefaultApi.GetJobs`: %v\n", resp)
}
```

### Path Parameters



### Other Parameters

Other parameters are passed through a pointer to a apiGetJobsRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **region** | **string** | Filters results based on the specified region | 
 **stale** | **string** | If present, results will include stale reads | 
 **prefix** | **string** | Constrains results to jobs that start with the defined prefix | 
 **namespace** | **string** | Filters results based on the specified namespace | 
 **perPage** | **int32** | Maximum number of results to return | 
 **nextToken** | **string** | Indicates where to start paging for queries that support pagination | 

### Return type

[**[]JobListStub**](JobListStub.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## ParseJobSpec

> Job ParseJobSpec(ctx).JobsParseRequest(jobsParseRequest).Execute()

Parses a HCL jobspec and produce the equivalent JSON encoded job. See https://www.nomadproject.io/api-docs/jobs#parse-job.

### Example

```go
package main

import (
    "context"
    "fmt"
    "os"
    openapiclient "./openapi"
)

func main() {
    jobsParseRequest := *openapiclient.NewJobsParseRequest() // JobsParseRequest | 

    configuration := openapiclient.NewConfiguration()
    api_client := openapiclient.NewAPIClient(configuration)
    resp, r, err := api_client.DefaultApi.ParseJobSpec(context.Background()).JobsParseRequest(jobsParseRequest).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `DefaultApi.ParseJobSpec``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `ParseJobSpec`: Job
    fmt.Fprintf(os.Stdout, "Response from `DefaultApi.ParseJobSpec`: %v\n", resp)
}
```

### Path Parameters



### Other Parameters

Other parameters are passed through a pointer to a apiParseJobSpecRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **jobsParseRequest** | [**JobsParseRequest**](JobsParseRequest.md) |  | 

### Return type

[**Job**](Job.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## PutJobForceRequest

> PeriodicForceResponse PutJobForceRequest(ctx, jobName).Region(region).Namespace(namespace).XNomadToken(xNomadToken).Execute()

Forces a new instance of the periodic job. A new instance will be created even if it violates the job's prohibit_overlap settings. As such, this should be only used to immediately run a periodic job. See [documentation](https://www.nomadproject.io/docs/commands/job/periodic-force).

### Example

```go
package main

import (
    "context"
    "fmt"
    "os"
    openapiclient "./openapi"
)

func main() {
    jobName := "jobName_example" // string | The job identifier.
    region := "region_example" // string | Filters results based on the specified region (optional)
    namespace := "namespace_example" // string | Filters results based on the specified namespace (optional)
    xNomadToken := "xNomadToken_example" // string | A Nomad ACL token (optional)

    configuration := openapiclient.NewConfiguration()
    api_client := openapiclient.NewAPIClient(configuration)
    resp, r, err := api_client.DefaultApi.PutJobForceRequest(context.Background(), jobName).Region(region).Namespace(namespace).XNomadToken(xNomadToken).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `DefaultApi.PutJobForceRequest``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `PutJobForceRequest`: PeriodicForceResponse
    fmt.Fprintf(os.Stdout, "Response from `DefaultApi.PutJobForceRequest`: %v\n", resp)
}
```

### Path Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**jobName** | **string** | The job identifier. | 

### Other Parameters

Other parameters are passed through a pointer to a apiPutJobForceRequestRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **region** | **string** | Filters results based on the specified region | 
 **namespace** | **string** | Filters results based on the specified namespace | 
 **xNomadToken** | **string** | A Nomad ACL token | 

### Return type

[**PeriodicForceResponse**](PeriodicForceResponse.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

