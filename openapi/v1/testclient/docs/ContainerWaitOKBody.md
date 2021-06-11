# ContainerWaitOKBody

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Error** | [**ContainerWaitOKBodyError**](ContainerWaitOKBodyError.md) |  | 
**StatusCode** | **int64** | Exit code of the container | 

## Methods

### NewContainerWaitOKBody

`func NewContainerWaitOKBody(error_ ContainerWaitOKBodyError, statusCode int64, ) *ContainerWaitOKBody`

NewContainerWaitOKBody instantiates a new ContainerWaitOKBody object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewContainerWaitOKBodyWithDefaults

`func NewContainerWaitOKBodyWithDefaults() *ContainerWaitOKBody`

NewContainerWaitOKBodyWithDefaults instantiates a new ContainerWaitOKBody object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetError

`func (o *ContainerWaitOKBody) GetError() ContainerWaitOKBodyError`

GetError returns the Error field if non-nil, zero value otherwise.

### GetErrorOk

`func (o *ContainerWaitOKBody) GetErrorOk() (*ContainerWaitOKBodyError, bool)`

GetErrorOk returns a tuple with the Error field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetError

`func (o *ContainerWaitOKBody) SetError(v ContainerWaitOKBodyError)`

SetError sets Error field to given value.


### GetStatusCode

`func (o *ContainerWaitOKBody) GetStatusCode() int64`

GetStatusCode returns the StatusCode field if non-nil, zero value otherwise.

### GetStatusCodeOk

`func (o *ContainerWaitOKBody) GetStatusCodeOk() (*int64, bool)`

GetStatusCodeOk returns a tuple with the StatusCode field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetStatusCode

`func (o *ContainerWaitOKBody) SetStatusCode(v int64)`

SetStatusCode sets StatusCode field to given value.



[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


