# \JobsApi

All URIs are relative to *http://127.0.0.1:4646/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**GetJobAllocations**](JobsApi.md#GetJobAllocations) | **Get** /job/{jobName}/allocations | Gets information about a single job&#39;s allocations. See [documentation](https://www.nomadproject.io/api-docs/allocations#list-allocations). 



## GetJobAllocations

> []AllocListStub GetJobAllocations(ctx, jobName).All(all).Region(region).Stale(stale).Prefix(prefix).Namespace(namespace).PerPage(perPage).NextToken(nextToken).Execute()

Gets information about a single job's allocations. See [documentation](https://www.nomadproject.io/api-docs/allocations#list-allocations). 

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

    configuration := openapiclient.NewConfiguration()
    api_client := openapiclient.NewAPIClient(configuration)
    resp, r, err := api_client.JobsApi.GetJobAllocations(context.Background(), jobName).All(all).Region(region).Stale(stale).Prefix(prefix).Namespace(namespace).PerPage(perPage).NextToken(nextToken).Execute()
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

