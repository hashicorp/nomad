# JobDispatchResponse

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**DispatchedJobID** | Pointer to **string** |  | [optional] 
**EvalCreateIndex** | Pointer to **int32** |  | [optional] 
**EvalID** | Pointer to **string** |  | [optional] 
**JobCreateIndex** | Pointer to **int32** |  | [optional] 
**LastIndex** | Pointer to **int32** | LastIndex. This can be used as a WaitIndex to perform a blocking query | [optional] 
**RequestTime** | Pointer to **int64** | A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years. | [optional] 

## Methods

### NewJobDispatchResponse

`func NewJobDispatchResponse() *JobDispatchResponse`

NewJobDispatchResponse instantiates a new JobDispatchResponse object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewJobDispatchResponseWithDefaults

`func NewJobDispatchResponseWithDefaults() *JobDispatchResponse`

NewJobDispatchResponseWithDefaults instantiates a new JobDispatchResponse object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetDispatchedJobID

`func (o *JobDispatchResponse) GetDispatchedJobID() string`

GetDispatchedJobID returns the DispatchedJobID field if non-nil, zero value otherwise.

### GetDispatchedJobIDOk

`func (o *JobDispatchResponse) GetDispatchedJobIDOk() (*string, bool)`

GetDispatchedJobIDOk returns a tuple with the DispatchedJobID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDispatchedJobID

`func (o *JobDispatchResponse) SetDispatchedJobID(v string)`

SetDispatchedJobID sets DispatchedJobID field to given value.

### HasDispatchedJobID

`func (o *JobDispatchResponse) HasDispatchedJobID() bool`

HasDispatchedJobID returns a boolean if a field has been set.

### GetEvalCreateIndex

`func (o *JobDispatchResponse) GetEvalCreateIndex() int32`

GetEvalCreateIndex returns the EvalCreateIndex field if non-nil, zero value otherwise.

### GetEvalCreateIndexOk

`func (o *JobDispatchResponse) GetEvalCreateIndexOk() (*int32, bool)`

GetEvalCreateIndexOk returns a tuple with the EvalCreateIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEvalCreateIndex

`func (o *JobDispatchResponse) SetEvalCreateIndex(v int32)`

SetEvalCreateIndex sets EvalCreateIndex field to given value.

### HasEvalCreateIndex

`func (o *JobDispatchResponse) HasEvalCreateIndex() bool`

HasEvalCreateIndex returns a boolean if a field has been set.

### GetEvalID

`func (o *JobDispatchResponse) GetEvalID() string`

GetEvalID returns the EvalID field if non-nil, zero value otherwise.

### GetEvalIDOk

`func (o *JobDispatchResponse) GetEvalIDOk() (*string, bool)`

GetEvalIDOk returns a tuple with the EvalID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEvalID

`func (o *JobDispatchResponse) SetEvalID(v string)`

SetEvalID sets EvalID field to given value.

### HasEvalID

`func (o *JobDispatchResponse) HasEvalID() bool`

HasEvalID returns a boolean if a field has been set.

### GetJobCreateIndex

`func (o *JobDispatchResponse) GetJobCreateIndex() int32`

GetJobCreateIndex returns the JobCreateIndex field if non-nil, zero value otherwise.

### GetJobCreateIndexOk

`func (o *JobDispatchResponse) GetJobCreateIndexOk() (*int32, bool)`

GetJobCreateIndexOk returns a tuple with the JobCreateIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetJobCreateIndex

`func (o *JobDispatchResponse) SetJobCreateIndex(v int32)`

SetJobCreateIndex sets JobCreateIndex field to given value.

### HasJobCreateIndex

`func (o *JobDispatchResponse) HasJobCreateIndex() bool`

HasJobCreateIndex returns a boolean if a field has been set.

### GetLastIndex

`func (o *JobDispatchResponse) GetLastIndex() int32`

GetLastIndex returns the LastIndex field if non-nil, zero value otherwise.

### GetLastIndexOk

`func (o *JobDispatchResponse) GetLastIndexOk() (*int32, bool)`

GetLastIndexOk returns a tuple with the LastIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLastIndex

`func (o *JobDispatchResponse) SetLastIndex(v int32)`

SetLastIndex sets LastIndex field to given value.

### HasLastIndex

`func (o *JobDispatchResponse) HasLastIndex() bool`

HasLastIndex returns a boolean if a field has been set.

### GetRequestTime

`func (o *JobDispatchResponse) GetRequestTime() int64`

GetRequestTime returns the RequestTime field if non-nil, zero value otherwise.

### GetRequestTimeOk

`func (o *JobDispatchResponse) GetRequestTimeOk() (*int64, bool)`

GetRequestTimeOk returns a tuple with the RequestTime field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetRequestTime

`func (o *JobDispatchResponse) SetRequestTime(v int64)`

SetRequestTime sets RequestTime field to given value.

### HasRequestTime

`func (o *JobDispatchResponse) HasRequestTime() bool`

HasRequestTime returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


