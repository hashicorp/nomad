# \JobsApi

All URIs are relative to *http://127.0.0.1:4646/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**GetJobAllocations**](JobsApi.md#GetJobAllocations) | **Get** /job/{jobName}/allocations | Gets information about a single job&#39;s allocations. See https://www.nomadproject.io/api-docs/allocations#list-allocations. 
[**GetJobEvaluations**](JobsApi.md#GetJobEvaluations) | **Get** /job/{jobName}/evaluations | Gets information about a single job&#39;s evaluations. See [documentation](https://www.nomadproject.io/api-docs/evaluations#list-evaluations). 
[**GetJobSummary**](JobsApi.md#GetJobSummary) | **Get** /job/{jobName}/summary | This endpoint reads summary information about a job. https://www.nomadproject.io/api-docs/jobs#read-job-summary.
[**GetJobs**](JobsApi.md#GetJobs) | **Get** /jobs | List all known jobs registered with Nomad. See https://www.nomadproject.io/api-docs/jobs#list-jobs.
[**PostJobEvaluateRequest**](JobsApi.md#PostJobEvaluateRequest) | **Post** /job/{jobName}/evaluate | Creates a new evaluation for the given job. This can be used to force run the scheduling logic if necessary. See https://www.nomadproject.io/api-docs/jobs#create-job-evaluation.
[**PostJobPlanRequest**](JobsApi.md#PostJobPlanRequest) | **Post** /job/{jobName}/plan | This endpoint invokes a dry-run of the scheduler for the job. See https://www.nomadproject.io/api-docs/jobs#create-job-plan
[**PostJobsParseRequest**](JobsApi.md#PostJobsParseRequest) | **Post** /jobs/parse | Parses a HCL jobspec and produce the equivalent JSON encoded job. See https://www.nomadproject.io/api-docs/jobs#parse-job.
[**PutJobForceRequest**](JobsApi.md#PutJobForceRequest) | **Post** /job/{jobName}/periodic/force | Forces a new instance of the periodic job. A new instance will be created even if it violates the job&#39;s prohibit_overlap settings. As such, this should be only used to immediately run a periodic job. See [documentation](https://www.nomadproject.io/docs/commands/job/periodic-force).



## GetJobAllocations

> []AllocListStub GetJobAllocations(ctx, jobName).All(all).Region(region).Stale(stale).Prefix(prefix).Namespace(namespace).PerPage(perPage).NextToken(nextToken).Index(index).Wait(wait).XNomadToken(xNomadToken).Execute()

Gets information about a single job's allocations. See https://www.nomadproject.io/api-docs/allocations#list-allocations. 

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
    all := int32(56) // int32 | Flag indicating whether to constrain by job creation index or not. (optional)
    region := "region_example" // string | Filters results based on the specified region (optional)
    stale := "stale_example" // string | If present, results will include stale reads (optional)
    prefix := "prefix_example" // string | Constrains results to jobs that start with the defined prefix (optional)
    namespace := "namespace_example" // string | Filters results based on the specified namespace (optional)
    perPage := int32(56) // int32 | Maximum number of results to return (optional)
    nextToken := "nextToken_example" // string | Indicates where to start paging for queries that support pagination (optional)
    index := int32(56) // int32 | If set, wait until query exceeds given index. Must be provided with WaitParam. (optional)
    wait := int32(56) // int32 | Provided with IndexParam to wait for change (optional)
    xNomadToken := "xNomadToken_example" // string | A Nomad ACL token (optional)

    configuration := openapiclient.NewConfiguration()
    api_client := openapiclient.NewAPIClient(configuration)
    resp, r, err := api_client.JobsApi.GetJobAllocations(context.Background(), jobName).All(all).Region(region).Stale(stale).Prefix(prefix).Namespace(namespace).PerPage(perPage).NextToken(nextToken).Index(index).Wait(wait).XNomadToken(xNomadToken).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `JobsApi.GetJobAllocations``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `GetJobAllocations`: []AllocListStub
    fmt.Fprintf(os.Stdout, "Response from `JobsApi.GetJobAllocations`: %v\n", resp)
}
```

### Path Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**jobName** | **string** | The job identifier. | 

### Other Parameters

Other parameters are passed through a pointer to a apiGetJobAllocationsRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **all** | **int32** | Flag indicating whether to constrain by job creation index or not. | 
 **region** | **string** | Filters results based on the specified region | 
 **stale** | **string** | If present, results will include stale reads | 
 **prefix** | **string** | Constrains results to jobs that start with the defined prefix | 
 **namespace** | **string** | Filters results based on the specified namespace | 
 **perPage** | **int32** | Maximum number of results to return | 
 **nextToken** | **string** | Indicates where to start paging for queries that support pagination | 
 **index** | **int32** | If set, wait until query exceeds given index. Must be provided with WaitParam. | 
 **wait** | **int32** | Provided with IndexParam to wait for change | 
 **xNomadToken** | **string** | A Nomad ACL token | 

### Return type

[**[]AllocListStub**](AllocListStub.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetJobEvaluations

> []Evaluation GetJobEvaluations(ctx, jobName).Region(region).Stale(stale).Prefix(prefix).Namespace(namespace).PerPage(perPage).NextToken(nextToken).Index(index).Wait(wait).XNomadToken(xNomadToken).Execute()

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
    index := int32(56) // int32 | If set, wait until query exceeds given index. Must be provided with WaitParam. (optional)
    wait := int32(56) // int32 | Provided with IndexParam to wait for change (optional)
    xNomadToken := "xNomadToken_example" // string | A Nomad ACL token (optional)

    configuration := openapiclient.NewConfiguration()
    api_client := openapiclient.NewAPIClient(configuration)
    resp, r, err := api_client.JobsApi.GetJobEvaluations(context.Background(), jobName).Region(region).Stale(stale).Prefix(prefix).Namespace(namespace).PerPage(perPage).NextToken(nextToken).Index(index).Wait(wait).XNomadToken(xNomadToken).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `JobsApi.GetJobEvaluations``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `GetJobEvaluations`: []Evaluation
    fmt.Fprintf(os.Stdout, "Response from `JobsApi.GetJobEvaluations`: %v\n", resp)
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
 **index** | **int32** | If set, wait until query exceeds given index. Must be provided with WaitParam. | 
 **wait** | **int32** | Provided with IndexParam to wait for change | 
 **xNomadToken** | **string** | A Nomad ACL token | 

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


## GetJobSummary

> JobSummary GetJobSummary(ctx).Region(region).Stale(stale).Prefix(prefix).Namespace(namespace).PerPage(perPage).NextToken(nextToken).Index(index).Wait(wait).XNomadToken(xNomadToken).Execute()

This endpoint reads summary information about a job. https://www.nomadproject.io/api-docs/jobs#read-job-summary.

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
    index := int32(56) // int32 | If set, wait until query exceeds given index. Must be provided with WaitParam. (optional)
    wait := int32(56) // int32 | Provided with IndexParam to wait for change (optional)
    xNomadToken := "xNomadToken_example" // string | A Nomad ACL token (optional)

    configuration := openapiclient.NewConfiguration()
    api_client := openapiclient.NewAPIClient(configuration)
    resp, r, err := api_client.JobsApi.GetJobSummary(context.Background()).Region(region).Stale(stale).Prefix(prefix).Namespace(namespace).PerPage(perPage).NextToken(nextToken).Index(index).Wait(wait).XNomadToken(xNomadToken).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `JobsApi.GetJobSummary``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `GetJobSummary`: JobSummary
    fmt.Fprintf(os.Stdout, "Response from `JobsApi.GetJobSummary`: %v\n", resp)
}
```

### Path Parameters



### Other Parameters

Other parameters are passed through a pointer to a apiGetJobSummaryRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **region** | **string** | Filters results based on the specified region | 
 **stale** | **string** | If present, results will include stale reads | 
 **prefix** | **string** | Constrains results to jobs that start with the defined prefix | 
 **namespace** | **string** | Filters results based on the specified namespace | 
 **perPage** | **int32** | Maximum number of results to return | 
 **nextToken** | **string** | Indicates where to start paging for queries that support pagination | 
 **index** | **int32** | If set, wait until query exceeds given index. Must be provided with WaitParam. | 
 **wait** | **int32** | Provided with IndexParam to wait for change | 
 **xNomadToken** | **string** | A Nomad ACL token | 

### Return type

[**JobSummary**](JobSummary.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetJobs

> []JobListStub GetJobs(ctx).Region(region).Stale(stale).Prefix(prefix).Namespace(namespace).PerPage(perPage).NextToken(nextToken).Index(index).Wait(wait).XNomadToken(xNomadToken).Execute()

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
    index := int32(56) // int32 | If set, wait until query exceeds given index. Must be provided with WaitParam. (optional)
    wait := int32(56) // int32 | Provided with IndexParam to wait for change (optional)
    xNomadToken := "xNomadToken_example" // string | A Nomad ACL token (optional)

    configuration := openapiclient.NewConfiguration()
    api_client := openapiclient.NewAPIClient(configuration)
    resp, r, err := api_client.JobsApi.GetJobs(context.Background()).Region(region).Stale(stale).Prefix(prefix).Namespace(namespace).PerPage(perPage).NextToken(nextToken).Index(index).Wait(wait).XNomadToken(xNomadToken).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `JobsApi.GetJobs``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `GetJobs`: []JobListStub
    fmt.Fprintf(os.Stdout, "Response from `JobsApi.GetJobs`: %v\n", resp)
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
 **index** | **int32** | If set, wait until query exceeds given index. Must be provided with WaitParam. | 
 **wait** | **int32** | Provided with IndexParam to wait for change | 
 **xNomadToken** | **string** | A Nomad ACL token | 

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


## PostJobEvaluateRequest

> JobRegisterResponse PostJobEvaluateRequest(ctx, jobName).JobEvaluateRequest(jobEvaluateRequest).Region(region).Namespace(namespace).XNomadToken(xNomadToken).Execute()

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
    region := "region_example" // string | Filters results based on the specified region (optional)
    namespace := "namespace_example" // string | Filters results based on the specified namespace (optional)
    xNomadToken := "xNomadToken_example" // string | A Nomad ACL token (optional)

    configuration := openapiclient.NewConfiguration()
    api_client := openapiclient.NewAPIClient(configuration)
    resp, r, err := api_client.JobsApi.PostJobEvaluateRequest(context.Background(), jobName).JobEvaluateRequest(jobEvaluateRequest).Region(region).Namespace(namespace).XNomadToken(xNomadToken).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `JobsApi.PostJobEvaluateRequest``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `PostJobEvaluateRequest`: JobRegisterResponse
    fmt.Fprintf(os.Stdout, "Response from `JobsApi.PostJobEvaluateRequest`: %v\n", resp)
}
```

### Path Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**jobName** | **string** | The job identifier. | 

### Other Parameters

Other parameters are passed through a pointer to a apiPostJobEvaluateRequestRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **jobEvaluateRequest** | [**JobEvaluateRequest**](JobEvaluateRequest.md) |  | 
 **region** | **string** | Filters results based on the specified region | 
 **namespace** | **string** | Filters results based on the specified namespace | 
 **xNomadToken** | **string** | A Nomad ACL token | 

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


## PostJobPlanRequest

> JobPlanResponse PostJobPlanRequest(ctx, jobName).JobPlanRequest(jobPlanRequest).Region(region).Namespace(namespace).XNomadToken(xNomadToken).Execute()

This endpoint invokes a dry-run of the scheduler for the job. See https://www.nomadproject.io/api-docs/jobs#create-job-plan

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
    jobPlanRequest := *openapiclient.NewJobPlanRequest() // JobPlanRequest | 
    region := "region_example" // string | Filters results based on the specified region (optional)
    namespace := "namespace_example" // string | Filters results based on the specified namespace (optional)
    xNomadToken := "xNomadToken_example" // string | A Nomad ACL token (optional)

    configuration := openapiclient.NewConfiguration()
    api_client := openapiclient.NewAPIClient(configuration)
    resp, r, err := api_client.JobsApi.PostJobPlanRequest(context.Background(), jobName).JobPlanRequest(jobPlanRequest).Region(region).Namespace(namespace).XNomadToken(xNomadToken).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `JobsApi.PostJobPlanRequest``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `PostJobPlanRequest`: JobPlanResponse
    fmt.Fprintf(os.Stdout, "Response from `JobsApi.PostJobPlanRequest`: %v\n", resp)
}
```

### Path Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**jobName** | **string** | The job identifier. | 

### Other Parameters

Other parameters are passed through a pointer to a apiPostJobPlanRequestRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **jobPlanRequest** | [**JobPlanRequest**](JobPlanRequest.md) |  | 
 **region** | **string** | Filters results based on the specified region | 
 **namespace** | **string** | Filters results based on the specified namespace | 
 **xNomadToken** | **string** | A Nomad ACL token | 

### Return type

[**JobPlanResponse**](JobPlanResponse.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## PostJobsParseRequest

> Job PostJobsParseRequest(ctx).JobsParseRequest(jobsParseRequest).Execute()

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
    resp, r, err := api_client.JobsApi.PostJobsParseRequest(context.Background()).JobsParseRequest(jobsParseRequest).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `JobsApi.PostJobsParseRequest``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `PostJobsParseRequest`: Job
    fmt.Fprintf(os.Stdout, "Response from `JobsApi.PostJobsParseRequest`: %v\n", resp)
}
```

### Path Parameters



### Other Parameters

Other parameters are passed through a pointer to a apiPostJobsParseRequestRequest struct via the builder pattern


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
    resp, r, err := api_client.JobsApi.PutJobForceRequest(context.Background(), jobName).Region(region).Namespace(namespace).XNomadToken(xNomadToken).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `JobsApi.PutJobForceRequest``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `PutJobForceRequest`: PeriodicForceResponse
    fmt.Fprintf(os.Stdout, "Response from `JobsApi.PutJobForceRequest`: %v\n", resp)
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

