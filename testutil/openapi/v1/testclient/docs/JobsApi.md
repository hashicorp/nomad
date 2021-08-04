# \JobsApi

All URIs are relative to *https://127.0.0.1:4646/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**JobJobNamePlanPost**](JobsApi.md#JobJobNamePlanPost) | **Post** /job/{jobName}/plan | 
[**JobsGet**](JobsApi.md#JobsGet) | **Get** /jobs | 
[**JobsPost**](JobsApi.md#JobsPost) | **Post** /jobs | 



## JobJobNamePlanPost

> JobPlanResponse JobJobNamePlanPost(ctx, jobName).JobPlanRequest(jobPlanRequest).Region(region).Namespace(namespace).Index(index).Wait(wait).Stale(stale).Prefix(prefix).XNomadToken(xNomadToken).PerPage(perPage).NextToken(nextToken).Execute()



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
    region := "region_example" // string | Filters results based on the specified region. (optional)
    namespace := "namespace_example" // string | Filters results based on the specified namespace. (optional)
    index := int32(56) // int32 | If set, wait until query exceeds given index. Must be provided with WaitParam. (optional)
    wait := int32(56) // int32 | Provided with IndexParam to wait for change. (optional)
    stale := "stale_example" // string | If present, results will include stale reads. (optional)
    prefix := "prefix_example" // string | Constrains results to jobs that start with the defined prefix (optional)
    xNomadToken := "xNomadToken_example" // string | A Nomad ACL token. (optional)
    perPage := int32(56) // int32 | Maximum number of results to return. (optional)
    nextToken := "nextToken_example" // string | Indicates where to start paging for queries that support pagination. (optional)

    configuration := openapiclient.NewConfiguration()
    api_client := openapiclient.NewAPIClient(configuration)
    resp, r, err := api_client.JobsApi.JobJobNamePlanPost(context.Background(), jobName).JobPlanRequest(jobPlanRequest).Region(region).Namespace(namespace).Index(index).Wait(wait).Stale(stale).Prefix(prefix).XNomadToken(xNomadToken).PerPage(perPage).NextToken(nextToken).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `JobsApi.JobJobNamePlanPost``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `JobJobNamePlanPost`: JobPlanResponse
    fmt.Fprintf(os.Stdout, "Response from `JobsApi.JobJobNamePlanPost`: %v\n", resp)
}
```

### Path Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**jobName** | **string** | The job identifier. | 

### Other Parameters

Other parameters are passed through a pointer to a apiJobJobNamePlanPostRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **jobPlanRequest** | [**JobPlanRequest**](JobPlanRequest.md) |  | 
 **region** | **string** | Filters results based on the specified region. | 
 **namespace** | **string** | Filters results based on the specified namespace. | 
 **index** | **int32** | If set, wait until query exceeds given index. Must be provided with WaitParam. | 
 **wait** | **int32** | Provided with IndexParam to wait for change. | 
 **stale** | **string** | If present, results will include stale reads. | 
 **prefix** | **string** | Constrains results to jobs that start with the defined prefix | 
 **xNomadToken** | **string** | A Nomad ACL token. | 
 **perPage** | **int32** | Maximum number of results to return. | 
 **nextToken** | **string** | Indicates where to start paging for queries that support pagination. | 

### Return type

[**JobPlanResponse**](JobPlanResponse.md)

### Authorization

[X-Nomad-Token](../README.md#X-Nomad-Token)

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## JobsGet

> []JobListStub JobsGet(ctx).Region(region).Namespace(namespace).Index(index).Wait(wait).Stale(stale).Prefix(prefix).XNomadToken(xNomadToken).PerPage(perPage).NextToken(nextToken).Execute()



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
    region := "region_example" // string | Filters results based on the specified region. (optional)
    namespace := "namespace_example" // string | Filters results based on the specified namespace. (optional)
    index := int32(56) // int32 | If set, wait until query exceeds given index. Must be provided with WaitParam. (optional)
    wait := int32(56) // int32 | Provided with IndexParam to wait for change. (optional)
    stale := "stale_example" // string | If present, results will include stale reads. (optional)
    prefix := "prefix_example" // string | Constrains results to jobs that start with the defined prefix (optional)
    xNomadToken := "xNomadToken_example" // string | A Nomad ACL token. (optional)
    perPage := int32(56) // int32 | Maximum number of results to return. (optional)
    nextToken := "nextToken_example" // string | Indicates where to start paging for queries that support pagination. (optional)

    configuration := openapiclient.NewConfiguration()
    api_client := openapiclient.NewAPIClient(configuration)
    resp, r, err := api_client.JobsApi.JobsGet(context.Background()).Region(region).Namespace(namespace).Index(index).Wait(wait).Stale(stale).Prefix(prefix).XNomadToken(xNomadToken).PerPage(perPage).NextToken(nextToken).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `JobsApi.JobsGet``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `JobsGet`: []JobListStub
    fmt.Fprintf(os.Stdout, "Response from `JobsApi.JobsGet`: %v\n", resp)
}
```

### Path Parameters



### Other Parameters

Other parameters are passed through a pointer to a apiJobsGetRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **region** | **string** | Filters results based on the specified region. | 
 **namespace** | **string** | Filters results based on the specified namespace. | 
 **index** | **int32** | If set, wait until query exceeds given index. Must be provided with WaitParam. | 
 **wait** | **int32** | Provided with IndexParam to wait for change. | 
 **stale** | **string** | If present, results will include stale reads. | 
 **prefix** | **string** | Constrains results to jobs that start with the defined prefix | 
 **xNomadToken** | **string** | A Nomad ACL token. | 
 **perPage** | **int32** | Maximum number of results to return. | 
 **nextToken** | **string** | Indicates where to start paging for queries that support pagination. | 

### Return type

[**[]JobListStub**](JobListStub.md)

### Authorization

[X-Nomad-Token](../README.md#X-Nomad-Token)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## JobsPost

> JobRegisterResponse JobsPost(ctx).JobRegisterRequest(jobRegisterRequest).Execute()



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
    jobRegisterRequest := *openapiclient.NewJobRegisterRequest() // JobRegisterRequest | 

    configuration := openapiclient.NewConfiguration()
    api_client := openapiclient.NewAPIClient(configuration)
    resp, r, err := api_client.JobsApi.JobsPost(context.Background()).JobRegisterRequest(jobRegisterRequest).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `JobsApi.JobsPost``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `JobsPost`: JobRegisterResponse
    fmt.Fprintf(os.Stdout, "Response from `JobsApi.JobsPost`: %v\n", resp)
}
```

### Path Parameters



### Other Parameters

Other parameters are passed through a pointer to a apiJobsPostRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **jobRegisterRequest** | [**JobRegisterRequest**](JobRegisterRequest.md) |  | 

### Return type

[**JobRegisterResponse**](JobRegisterResponse.md)

### Authorization

[X-Nomad-Token](../README.md#X-Nomad-Token)

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

