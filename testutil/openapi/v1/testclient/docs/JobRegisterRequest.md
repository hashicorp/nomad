# JobRegisterRequest

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**EnforceIndex** | Pointer to **bool** |  | [optional] 
**Job** | Pointer to [**Job**](Job.md) |  | [optional] 
**JobModifyIndex** | Pointer to **int32** |  | [optional] 
**Namespace** | Pointer to **string** |  | [optional] 
**PolicyOverride** | Pointer to **bool** |  | [optional] 
**PreserveCounts** | Pointer to **bool** |  | [optional] 
**Region** | Pointer to **string** |  | [optional] 
**SecretID** | Pointer to **string** |  | [optional] 

## Methods

### NewJobRegisterRequest

`func NewJobRegisterRequest() *JobRegisterRequest`

NewJobRegisterRequest instantiates a new JobRegisterRequest object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewJobRegisterRequestWithDefaults

`func NewJobRegisterRequestWithDefaults() *JobRegisterRequest`

NewJobRegisterRequestWithDefaults instantiates a new JobRegisterRequest object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetEnforceIndex

`func (o *JobRegisterRequest) GetEnforceIndex() bool`

GetEnforceIndex returns the EnforceIndex field if non-nil, zero value otherwise.

### GetEnforceIndexOk

`func (o *JobRegisterRequest) GetEnforceIndexOk() (*bool, bool)`

GetEnforceIndexOk returns a tuple with the EnforceIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEnforceIndex

`func (o *JobRegisterRequest) SetEnforceIndex(v bool)`

SetEnforceIndex sets EnforceIndex field to given value.

### HasEnforceIndex

`func (o *JobRegisterRequest) HasEnforceIndex() bool`

HasEnforceIndex returns a boolean if a field has been set.

### GetJob

`func (o *JobRegisterRequest) GetJob() Job`

GetJob returns the Job field if non-nil, zero value otherwise.

### GetJobOk

`func (o *JobRegisterRequest) GetJobOk() (*Job, bool)`

GetJobOk returns a tuple with the Job field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetJob

`func (o *JobRegisterRequest) SetJob(v Job)`

SetJob sets Job field to given value.

### HasJob

`func (o *JobRegisterRequest) HasJob() bool`

HasJob returns a boolean if a field has been set.

### GetJobModifyIndex

`func (o *JobRegisterRequest) GetJobModifyIndex() int32`

GetJobModifyIndex returns the JobModifyIndex field if non-nil, zero value otherwise.

### GetJobModifyIndexOk

`func (o *JobRegisterRequest) GetJobModifyIndexOk() (*int32, bool)`

GetJobModifyIndexOk returns a tuple with the JobModifyIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetJobModifyIndex

`func (o *JobRegisterRequest) SetJobModifyIndex(v int32)`

SetJobModifyIndex sets JobModifyIndex field to given value.

### HasJobModifyIndex

`func (o *JobRegisterRequest) HasJobModifyIndex() bool`

HasJobModifyIndex returns a boolean if a field has been set.

### GetNamespace

`func (o *JobRegisterRequest) GetNamespace() string`

GetNamespace returns the Namespace field if non-nil, zero value otherwise.

### GetNamespaceOk

`func (o *JobRegisterRequest) GetNamespaceOk() (*string, bool)`

GetNamespaceOk returns a tuple with the Namespace field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNamespace

`func (o *JobRegisterRequest) SetNamespace(v string)`

SetNamespace sets Namespace field to given value.

### HasNamespace

`func (o *JobRegisterRequest) HasNamespace() bool`

HasNamespace returns a boolean if a field has been set.

### GetPolicyOverride

`func (o *JobRegisterRequest) GetPolicyOverride() bool`

GetPolicyOverride returns the PolicyOverride field if non-nil, zero value otherwise.

### GetPolicyOverrideOk

`func (o *JobRegisterRequest) GetPolicyOverrideOk() (*bool, bool)`

GetPolicyOverrideOk returns a tuple with the PolicyOverride field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPolicyOverride

`func (o *JobRegisterRequest) SetPolicyOverride(v bool)`

SetPolicyOverride sets PolicyOverride field to given value.

### HasPolicyOverride

`func (o *JobRegisterRequest) HasPolicyOverride() bool`

HasPolicyOverride returns a boolean if a field has been set.

### GetPreserveCounts

`func (o *JobRegisterRequest) GetPreserveCounts() bool`

GetPreserveCounts returns the PreserveCounts field if non-nil, zero value otherwise.

### GetPreserveCountsOk

`func (o *JobRegisterRequest) GetPreserveCountsOk() (*bool, bool)`

GetPreserveCountsOk returns a tuple with the PreserveCounts field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPreserveCounts

`func (o *JobRegisterRequest) SetPreserveCounts(v bool)`

SetPreserveCounts sets PreserveCounts field to given value.

### HasPreserveCounts

`func (o *JobRegisterRequest) HasPreserveCounts() bool`

HasPreserveCounts returns a boolean if a field has been set.

### GetRegion

`func (o *JobRegisterRequest) GetRegion() string`

GetRegion returns the Region field if non-nil, zero value otherwise.

### GetRegionOk

`func (o *JobRegisterRequest) GetRegionOk() (*string, bool)`

GetRegionOk returns a tuple with the Region field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetRegion

`func (o *JobRegisterRequest) SetRegion(v string)`

SetRegion sets Region field to given value.

### HasRegion

`func (o *JobRegisterRequest) HasRegion() bool`

HasRegion returns a boolean if a field has been set.

### GetSecretID

`func (o *JobRegisterRequest) GetSecretID() string`

GetSecretID returns the SecretID field if non-nil, zero value otherwise.

### GetSecretIDOk

`func (o *JobRegisterRequest) GetSecretIDOk() (*string, bool)`

GetSecretIDOk returns a tuple with the SecretID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSecretID

`func (o *JobRegisterRequest) SetSecretID(v string)`

SetSecretID sets SecretID field to given value.

### HasSecretID

`func (o *JobRegisterRequest) HasSecretID() bool`

HasSecretID returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


