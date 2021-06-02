# JobStabilityRequest

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**JobID** | Pointer to **string** | Job to set the stability on | [optional] 
**JobVersion** | Pointer to **int32** |  | [optional] 
**Namespace** | Pointer to **string** | Namespace is the target namespace for this write | [optional] 
**Region** | Pointer to **string** | The target region for this write | [optional] 
**SecretID** | Pointer to **string** | SecretID is the secret ID of an ACL token | [optional] 
**Stable** | Pointer to **bool** | Set the stability | [optional] 

## Methods

### NewJobStabilityRequest

`func NewJobStabilityRequest() *JobStabilityRequest`

NewJobStabilityRequest instantiates a new JobStabilityRequest object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewJobStabilityRequestWithDefaults

`func NewJobStabilityRequestWithDefaults() *JobStabilityRequest`

NewJobStabilityRequestWithDefaults instantiates a new JobStabilityRequest object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetJobID

`func (o *JobStabilityRequest) GetJobID() string`

GetJobID returns the JobID field if non-nil, zero value otherwise.

### GetJobIDOk

`func (o *JobStabilityRequest) GetJobIDOk() (*string, bool)`

GetJobIDOk returns a tuple with the JobID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetJobID

`func (o *JobStabilityRequest) SetJobID(v string)`

SetJobID sets JobID field to given value.

### HasJobID

`func (o *JobStabilityRequest) HasJobID() bool`

HasJobID returns a boolean if a field has been set.

### GetJobVersion

`func (o *JobStabilityRequest) GetJobVersion() int32`

GetJobVersion returns the JobVersion field if non-nil, zero value otherwise.

### GetJobVersionOk

`func (o *JobStabilityRequest) GetJobVersionOk() (*int32, bool)`

GetJobVersionOk returns a tuple with the JobVersion field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetJobVersion

`func (o *JobStabilityRequest) SetJobVersion(v int32)`

SetJobVersion sets JobVersion field to given value.

### HasJobVersion

`func (o *JobStabilityRequest) HasJobVersion() bool`

HasJobVersion returns a boolean if a field has been set.

### GetNamespace

`func (o *JobStabilityRequest) GetNamespace() string`

GetNamespace returns the Namespace field if non-nil, zero value otherwise.

### GetNamespaceOk

`func (o *JobStabilityRequest) GetNamespaceOk() (*string, bool)`

GetNamespaceOk returns a tuple with the Namespace field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNamespace

`func (o *JobStabilityRequest) SetNamespace(v string)`

SetNamespace sets Namespace field to given value.

### HasNamespace

`func (o *JobStabilityRequest) HasNamespace() bool`

HasNamespace returns a boolean if a field has been set.

### GetRegion

`func (o *JobStabilityRequest) GetRegion() string`

GetRegion returns the Region field if non-nil, zero value otherwise.

### GetRegionOk

`func (o *JobStabilityRequest) GetRegionOk() (*string, bool)`

GetRegionOk returns a tuple with the Region field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetRegion

`func (o *JobStabilityRequest) SetRegion(v string)`

SetRegion sets Region field to given value.

### HasRegion

`func (o *JobStabilityRequest) HasRegion() bool`

HasRegion returns a boolean if a field has been set.

### GetSecretID

`func (o *JobStabilityRequest) GetSecretID() string`

GetSecretID returns the SecretID field if non-nil, zero value otherwise.

### GetSecretIDOk

`func (o *JobStabilityRequest) GetSecretIDOk() (*string, bool)`

GetSecretIDOk returns a tuple with the SecretID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSecretID

`func (o *JobStabilityRequest) SetSecretID(v string)`

SetSecretID sets SecretID field to given value.

### HasSecretID

`func (o *JobStabilityRequest) HasSecretID() bool`

HasSecretID returns a boolean if a field has been set.

### GetStable

`func (o *JobStabilityRequest) GetStable() bool`

GetStable returns the Stable field if non-nil, zero value otherwise.

### GetStableOk

`func (o *JobStabilityRequest) GetStableOk() (*bool, bool)`

GetStableOk returns a tuple with the Stable field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetStable

`func (o *JobStabilityRequest) SetStable(v bool)`

SetStable sets Stable field to given value.

### HasStable

`func (o *JobStabilityRequest) HasStable() bool`

HasStable returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


