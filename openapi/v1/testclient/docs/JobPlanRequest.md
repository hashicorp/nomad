# JobPlanRequest

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Diff** | Pointer to **bool** |  | [optional] 
**Job** | Pointer to [**Job**](Job.md) |  | [optional] 
**Namespace** | Pointer to **string** | Namespace is the target namespace for this write | [optional] 
**PolicyOverride** | Pointer to **bool** |  | [optional] 
**Region** | Pointer to **string** | The target region for this write | [optional] 
**SecretID** | Pointer to **string** | SecretID is the secret ID of an ACL token | [optional] 

## Methods

### NewJobPlanRequest

`func NewJobPlanRequest() *JobPlanRequest`

NewJobPlanRequest instantiates a new JobPlanRequest object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewJobPlanRequestWithDefaults

`func NewJobPlanRequestWithDefaults() *JobPlanRequest`

NewJobPlanRequestWithDefaults instantiates a new JobPlanRequest object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetDiff

`func (o *JobPlanRequest) GetDiff() bool`

GetDiff returns the Diff field if non-nil, zero value otherwise.

### GetDiffOk

`func (o *JobPlanRequest) GetDiffOk() (*bool, bool)`

GetDiffOk returns a tuple with the Diff field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDiff

`func (o *JobPlanRequest) SetDiff(v bool)`

SetDiff sets Diff field to given value.

### HasDiff

`func (o *JobPlanRequest) HasDiff() bool`

HasDiff returns a boolean if a field has been set.

### GetJob

`func (o *JobPlanRequest) GetJob() Job`

GetJob returns the Job field if non-nil, zero value otherwise.

### GetJobOk

`func (o *JobPlanRequest) GetJobOk() (*Job, bool)`

GetJobOk returns a tuple with the Job field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetJob

`func (o *JobPlanRequest) SetJob(v Job)`

SetJob sets Job field to given value.

### HasJob

`func (o *JobPlanRequest) HasJob() bool`

HasJob returns a boolean if a field has been set.

### GetNamespace

`func (o *JobPlanRequest) GetNamespace() string`

GetNamespace returns the Namespace field if non-nil, zero value otherwise.

### GetNamespaceOk

`func (o *JobPlanRequest) GetNamespaceOk() (*string, bool)`

GetNamespaceOk returns a tuple with the Namespace field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNamespace

`func (o *JobPlanRequest) SetNamespace(v string)`

SetNamespace sets Namespace field to given value.

### HasNamespace

`func (o *JobPlanRequest) HasNamespace() bool`

HasNamespace returns a boolean if a field has been set.

### GetPolicyOverride

`func (o *JobPlanRequest) GetPolicyOverride() bool`

GetPolicyOverride returns the PolicyOverride field if non-nil, zero value otherwise.

### GetPolicyOverrideOk

`func (o *JobPlanRequest) GetPolicyOverrideOk() (*bool, bool)`

GetPolicyOverrideOk returns a tuple with the PolicyOverride field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPolicyOverride

`func (o *JobPlanRequest) SetPolicyOverride(v bool)`

SetPolicyOverride sets PolicyOverride field to given value.

### HasPolicyOverride

`func (o *JobPlanRequest) HasPolicyOverride() bool`

HasPolicyOverride returns a boolean if a field has been set.

### GetRegion

`func (o *JobPlanRequest) GetRegion() string`

GetRegion returns the Region field if non-nil, zero value otherwise.

### GetRegionOk

`func (o *JobPlanRequest) GetRegionOk() (*string, bool)`

GetRegionOk returns a tuple with the Region field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetRegion

`func (o *JobPlanRequest) SetRegion(v string)`

SetRegion sets Region field to given value.

### HasRegion

`func (o *JobPlanRequest) HasRegion() bool`

HasRegion returns a boolean if a field has been set.

### GetSecretID

`func (o *JobPlanRequest) GetSecretID() string`

GetSecretID returns the SecretID field if non-nil, zero value otherwise.

### GetSecretIDOk

`func (o *JobPlanRequest) GetSecretIDOk() (*string, bool)`

GetSecretIDOk returns a tuple with the SecretID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSecretID

`func (o *JobPlanRequest) SetSecretID(v string)`

SetSecretID sets SecretID field to given value.

### HasSecretID

`func (o *JobPlanRequest) HasSecretID() bool`

HasSecretID returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


