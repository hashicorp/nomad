# JobDeregisterResponse

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**EvalCreateIndex** | Pointer to **int32** |  | [optional] 
**EvalID** | Pointer to **string** |  | [optional] 
**JobModifyIndex** | Pointer to **int32** |  | [optional] 
**KnownLeader** | Pointer to **bool** | Is there a known leader | [optional] 
**LastContact** | Pointer to **int64** | A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years. | [optional] 
**LastIndex** | Pointer to **int32** | LastIndex. This can be used as a WaitIndex to perform a blocking query | [optional] 
**RequestTime** | Pointer to **int64** | A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years. | [optional] 

## Methods

### NewJobDeregisterResponse

`func NewJobDeregisterResponse() *JobDeregisterResponse`

NewJobDeregisterResponse instantiates a new JobDeregisterResponse object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewJobDeregisterResponseWithDefaults

`func NewJobDeregisterResponseWithDefaults() *JobDeregisterResponse`

NewJobDeregisterResponseWithDefaults instantiates a new JobDeregisterResponse object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetEvalCreateIndex

`func (o *JobDeregisterResponse) GetEvalCreateIndex() int32`

GetEvalCreateIndex returns the EvalCreateIndex field if non-nil, zero value otherwise.

### GetEvalCreateIndexOk

`func (o *JobDeregisterResponse) GetEvalCreateIndexOk() (*int32, bool)`

GetEvalCreateIndexOk returns a tuple with the EvalCreateIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEvalCreateIndex

`func (o *JobDeregisterResponse) SetEvalCreateIndex(v int32)`

SetEvalCreateIndex sets EvalCreateIndex field to given value.

### HasEvalCreateIndex

`func (o *JobDeregisterResponse) HasEvalCreateIndex() bool`

HasEvalCreateIndex returns a boolean if a field has been set.

### GetEvalID

`func (o *JobDeregisterResponse) GetEvalID() string`

GetEvalID returns the EvalID field if non-nil, zero value otherwise.

### GetEvalIDOk

`func (o *JobDeregisterResponse) GetEvalIDOk() (*string, bool)`

GetEvalIDOk returns a tuple with the EvalID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEvalID

`func (o *JobDeregisterResponse) SetEvalID(v string)`

SetEvalID sets EvalID field to given value.

### HasEvalID

`func (o *JobDeregisterResponse) HasEvalID() bool`

HasEvalID returns a boolean if a field has been set.

### GetJobModifyIndex

`func (o *JobDeregisterResponse) GetJobModifyIndex() int32`

GetJobModifyIndex returns the JobModifyIndex field if non-nil, zero value otherwise.

### GetJobModifyIndexOk

`func (o *JobDeregisterResponse) GetJobModifyIndexOk() (*int32, bool)`

GetJobModifyIndexOk returns a tuple with the JobModifyIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetJobModifyIndex

`func (o *JobDeregisterResponse) SetJobModifyIndex(v int32)`

SetJobModifyIndex sets JobModifyIndex field to given value.

### HasJobModifyIndex

`func (o *JobDeregisterResponse) HasJobModifyIndex() bool`

HasJobModifyIndex returns a boolean if a field has been set.

### GetKnownLeader

`func (o *JobDeregisterResponse) GetKnownLeader() bool`

GetKnownLeader returns the KnownLeader field if non-nil, zero value otherwise.

### GetKnownLeaderOk

`func (o *JobDeregisterResponse) GetKnownLeaderOk() (*bool, bool)`

GetKnownLeaderOk returns a tuple with the KnownLeader field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetKnownLeader

`func (o *JobDeregisterResponse) SetKnownLeader(v bool)`

SetKnownLeader sets KnownLeader field to given value.

### HasKnownLeader

`func (o *JobDeregisterResponse) HasKnownLeader() bool`

HasKnownLeader returns a boolean if a field has been set.

### GetLastContact

`func (o *JobDeregisterResponse) GetLastContact() int64`

GetLastContact returns the LastContact field if non-nil, zero value otherwise.

### GetLastContactOk

`func (o *JobDeregisterResponse) GetLastContactOk() (*int64, bool)`

GetLastContactOk returns a tuple with the LastContact field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLastContact

`func (o *JobDeregisterResponse) SetLastContact(v int64)`

SetLastContact sets LastContact field to given value.

### HasLastContact

`func (o *JobDeregisterResponse) HasLastContact() bool`

HasLastContact returns a boolean if a field has been set.

### GetLastIndex

`func (o *JobDeregisterResponse) GetLastIndex() int32`

GetLastIndex returns the LastIndex field if non-nil, zero value otherwise.

### GetLastIndexOk

`func (o *JobDeregisterResponse) GetLastIndexOk() (*int32, bool)`

GetLastIndexOk returns a tuple with the LastIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLastIndex

`func (o *JobDeregisterResponse) SetLastIndex(v int32)`

SetLastIndex sets LastIndex field to given value.

### HasLastIndex

`func (o *JobDeregisterResponse) HasLastIndex() bool`

HasLastIndex returns a boolean if a field has been set.

### GetRequestTime

`func (o *JobDeregisterResponse) GetRequestTime() int64`

GetRequestTime returns the RequestTime field if non-nil, zero value otherwise.

### GetRequestTimeOk

`func (o *JobDeregisterResponse) GetRequestTimeOk() (*int64, bool)`

GetRequestTimeOk returns a tuple with the RequestTime field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetRequestTime

`func (o *JobDeregisterResponse) SetRequestTime(v int64)`

SetRequestTime sets RequestTime field to given value.

### HasRequestTime

`func (o *JobDeregisterResponse) HasRequestTime() bool`

HasRequestTime returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


