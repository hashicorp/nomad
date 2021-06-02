# JobEvaluateRequest

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**EvalOptions** | Pointer to [**EvalOptions**](EvalOptions.md) |  | [optional] 
**JobID** | Pointer to **string** |  | [optional] 
**Namespace** | Pointer to **string** | Namespace is the target namespace for this write | [optional] 
**Region** | Pointer to **string** | The target region for this write | [optional] 
**SecretID** | Pointer to **string** | SecretID is the secret ID of an ACL token | [optional] 

## Methods

### NewJobEvaluateRequest

`func NewJobEvaluateRequest() *JobEvaluateRequest`

NewJobEvaluateRequest instantiates a new JobEvaluateRequest object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewJobEvaluateRequestWithDefaults

`func NewJobEvaluateRequestWithDefaults() *JobEvaluateRequest`

NewJobEvaluateRequestWithDefaults instantiates a new JobEvaluateRequest object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetEvalOptions

`func (o *JobEvaluateRequest) GetEvalOptions() EvalOptions`

GetEvalOptions returns the EvalOptions field if non-nil, zero value otherwise.

### GetEvalOptionsOk

`func (o *JobEvaluateRequest) GetEvalOptionsOk() (*EvalOptions, bool)`

GetEvalOptionsOk returns a tuple with the EvalOptions field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEvalOptions

`func (o *JobEvaluateRequest) SetEvalOptions(v EvalOptions)`

SetEvalOptions sets EvalOptions field to given value.

### HasEvalOptions

`func (o *JobEvaluateRequest) HasEvalOptions() bool`

HasEvalOptions returns a boolean if a field has been set.

### GetJobID

`func (o *JobEvaluateRequest) GetJobID() string`

GetJobID returns the JobID field if non-nil, zero value otherwise.

### GetJobIDOk

`func (o *JobEvaluateRequest) GetJobIDOk() (*string, bool)`

GetJobIDOk returns a tuple with the JobID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetJobID

`func (o *JobEvaluateRequest) SetJobID(v string)`

SetJobID sets JobID field to given value.

### HasJobID

`func (o *JobEvaluateRequest) HasJobID() bool`

HasJobID returns a boolean if a field has been set.

### GetNamespace

`func (o *JobEvaluateRequest) GetNamespace() string`

GetNamespace returns the Namespace field if non-nil, zero value otherwise.

### GetNamespaceOk

`func (o *JobEvaluateRequest) GetNamespaceOk() (*string, bool)`

GetNamespaceOk returns a tuple with the Namespace field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNamespace

`func (o *JobEvaluateRequest) SetNamespace(v string)`

SetNamespace sets Namespace field to given value.

### HasNamespace

`func (o *JobEvaluateRequest) HasNamespace() bool`

HasNamespace returns a boolean if a field has been set.

### GetRegion

`func (o *JobEvaluateRequest) GetRegion() string`

GetRegion returns the Region field if non-nil, zero value otherwise.

### GetRegionOk

`func (o *JobEvaluateRequest) GetRegionOk() (*string, bool)`

GetRegionOk returns a tuple with the Region field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetRegion

`func (o *JobEvaluateRequest) SetRegion(v string)`

SetRegion sets Region field to given value.

### HasRegion

`func (o *JobEvaluateRequest) HasRegion() bool`

HasRegion returns a boolean if a field has been set.

### GetSecretID

`func (o *JobEvaluateRequest) GetSecretID() string`

GetSecretID returns the SecretID field if non-nil, zero value otherwise.

### GetSecretIDOk

`func (o *JobEvaluateRequest) GetSecretIDOk() (*string, bool)`

GetSecretIDOk returns a tuple with the SecretID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSecretID

`func (o *JobEvaluateRequest) SetSecretID(v string)`

SetSecretID sets SecretID field to given value.

### HasSecretID

`func (o *JobEvaluateRequest) HasSecretID() bool`

HasSecretID returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


