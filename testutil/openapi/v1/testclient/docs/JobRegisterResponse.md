# JobRegisterResponse

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**EvalCreateIndex** | Pointer to **int32** |  | [optional] 
**EvalID** | Pointer to **string** |  | [optional] 
**JobModifyIndex** | Pointer to **int32** |  | [optional] 
**KnownLeader** | Pointer to **bool** |  | [optional] 
**LastContact** | Pointer to **int64** |  | [optional] 
**LastIndex** | Pointer to **int32** |  | [optional] 
**RequestTime** | Pointer to **int64** |  | [optional] 
**Warnings** | Pointer to **string** |  | [optional] 

## Methods

### NewJobRegisterResponse

`func NewJobRegisterResponse() *JobRegisterResponse`

NewJobRegisterResponse instantiates a new JobRegisterResponse object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewJobRegisterResponseWithDefaults

`func NewJobRegisterResponseWithDefaults() *JobRegisterResponse`

NewJobRegisterResponseWithDefaults instantiates a new JobRegisterResponse object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetEvalCreateIndex

`func (o *JobRegisterResponse) GetEvalCreateIndex() int32`

GetEvalCreateIndex returns the EvalCreateIndex field if non-nil, zero value otherwise.

### GetEvalCreateIndexOk

`func (o *JobRegisterResponse) GetEvalCreateIndexOk() (*int32, bool)`

GetEvalCreateIndexOk returns a tuple with the EvalCreateIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEvalCreateIndex

`func (o *JobRegisterResponse) SetEvalCreateIndex(v int32)`

SetEvalCreateIndex sets EvalCreateIndex field to given value.

### HasEvalCreateIndex

`func (o *JobRegisterResponse) HasEvalCreateIndex() bool`

HasEvalCreateIndex returns a boolean if a field has been set.

### GetEvalID

`func (o *JobRegisterResponse) GetEvalID() string`

GetEvalID returns the EvalID field if non-nil, zero value otherwise.

### GetEvalIDOk

`func (o *JobRegisterResponse) GetEvalIDOk() (*string, bool)`

GetEvalIDOk returns a tuple with the EvalID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEvalID

`func (o *JobRegisterResponse) SetEvalID(v string)`

SetEvalID sets EvalID field to given value.

### HasEvalID

`func (o *JobRegisterResponse) HasEvalID() bool`

HasEvalID returns a boolean if a field has been set.

### GetJobModifyIndex

`func (o *JobRegisterResponse) GetJobModifyIndex() int32`

GetJobModifyIndex returns the JobModifyIndex field if non-nil, zero value otherwise.

### GetJobModifyIndexOk

`func (o *JobRegisterResponse) GetJobModifyIndexOk() (*int32, bool)`

GetJobModifyIndexOk returns a tuple with the JobModifyIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetJobModifyIndex

`func (o *JobRegisterResponse) SetJobModifyIndex(v int32)`

SetJobModifyIndex sets JobModifyIndex field to given value.

### HasJobModifyIndex

`func (o *JobRegisterResponse) HasJobModifyIndex() bool`

HasJobModifyIndex returns a boolean if a field has been set.

### GetKnownLeader

`func (o *JobRegisterResponse) GetKnownLeader() bool`

GetKnownLeader returns the KnownLeader field if non-nil, zero value otherwise.

### GetKnownLeaderOk

`func (o *JobRegisterResponse) GetKnownLeaderOk() (*bool, bool)`

GetKnownLeaderOk returns a tuple with the KnownLeader field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetKnownLeader

`func (o *JobRegisterResponse) SetKnownLeader(v bool)`

SetKnownLeader sets KnownLeader field to given value.

### HasKnownLeader

`func (o *JobRegisterResponse) HasKnownLeader() bool`

HasKnownLeader returns a boolean if a field has been set.

### GetLastContact

`func (o *JobRegisterResponse) GetLastContact() int64`

GetLastContact returns the LastContact field if non-nil, zero value otherwise.

### GetLastContactOk

`func (o *JobRegisterResponse) GetLastContactOk() (*int64, bool)`

GetLastContactOk returns a tuple with the LastContact field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLastContact

`func (o *JobRegisterResponse) SetLastContact(v int64)`

SetLastContact sets LastContact field to given value.

### HasLastContact

`func (o *JobRegisterResponse) HasLastContact() bool`

HasLastContact returns a boolean if a field has been set.

### GetLastIndex

`func (o *JobRegisterResponse) GetLastIndex() int32`

GetLastIndex returns the LastIndex field if non-nil, zero value otherwise.

### GetLastIndexOk

`func (o *JobRegisterResponse) GetLastIndexOk() (*int32, bool)`

GetLastIndexOk returns a tuple with the LastIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLastIndex

`func (o *JobRegisterResponse) SetLastIndex(v int32)`

SetLastIndex sets LastIndex field to given value.

### HasLastIndex

`func (o *JobRegisterResponse) HasLastIndex() bool`

HasLastIndex returns a boolean if a field has been set.

### GetRequestTime

`func (o *JobRegisterResponse) GetRequestTime() int64`

GetRequestTime returns the RequestTime field if non-nil, zero value otherwise.

### GetRequestTimeOk

`func (o *JobRegisterResponse) GetRequestTimeOk() (*int64, bool)`

GetRequestTimeOk returns a tuple with the RequestTime field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetRequestTime

`func (o *JobRegisterResponse) SetRequestTime(v int64)`

SetRequestTime sets RequestTime field to given value.

### HasRequestTime

`func (o *JobRegisterResponse) HasRequestTime() bool`

HasRequestTime returns a boolean if a field has been set.

### GetWarnings

`func (o *JobRegisterResponse) GetWarnings() string`

GetWarnings returns the Warnings field if non-nil, zero value otherwise.

### GetWarningsOk

`func (o *JobRegisterResponse) GetWarningsOk() (*string, bool)`

GetWarningsOk returns a tuple with the Warnings field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetWarnings

`func (o *JobRegisterResponse) SetWarnings(v string)`

SetWarnings sets Warnings field to given value.

### HasWarnings

`func (o *JobRegisterResponse) HasWarnings() bool`

HasWarnings returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


