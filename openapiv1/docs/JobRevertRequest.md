# JobRevertRequest

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**ConsulToken** | Pointer to **string** | ConsulToken is the Consul token that proves the submitter of the job revert has access to the Service Identity policies associated with the job&#39;s Consul Connect enabled services. This field is only used to transfer the token and is not stored after the Job revert. | [optional] 
**EnforcePriorVersion** | Pointer to **int32** | EnforcePriorVersion if set will enforce that the job is at the given version before reverting. | [optional] 
**JobID** | Pointer to **string** | JobID is the ID of the job  being reverted | [optional] 
**JobVersion** | Pointer to **int32** | JobVersion the version to revert to. | [optional] 
**Namespace** | Pointer to **string** | Namespace is the target namespace for this write | [optional] 
**Region** | Pointer to **string** | The target region for this write | [optional] 
**SecretID** | Pointer to **string** | SecretID is the secret ID of an ACL token | [optional] 
**VaultToken** | Pointer to **string** | VaultToken is the Vault token that proves the submitter of the job revert has access to any Vault policies specified in the targeted job version. This field is only used to authorize the revert and is not stored after the Job revert. | [optional] 

## Methods

### NewJobRevertRequest

`func NewJobRevertRequest() *JobRevertRequest`

NewJobRevertRequest instantiates a new JobRevertRequest object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewJobRevertRequestWithDefaults

`func NewJobRevertRequestWithDefaults() *JobRevertRequest`

NewJobRevertRequestWithDefaults instantiates a new JobRevertRequest object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetConsulToken

`func (o *JobRevertRequest) GetConsulToken() string`

GetConsulToken returns the ConsulToken field if non-nil, zero value otherwise.

### GetConsulTokenOk

`func (o *JobRevertRequest) GetConsulTokenOk() (*string, bool)`

GetConsulTokenOk returns a tuple with the ConsulToken field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetConsulToken

`func (o *JobRevertRequest) SetConsulToken(v string)`

SetConsulToken sets ConsulToken field to given value.

### HasConsulToken

`func (o *JobRevertRequest) HasConsulToken() bool`

HasConsulToken returns a boolean if a field has been set.

### GetEnforcePriorVersion

`func (o *JobRevertRequest) GetEnforcePriorVersion() int32`

GetEnforcePriorVersion returns the EnforcePriorVersion field if non-nil, zero value otherwise.

### GetEnforcePriorVersionOk

`func (o *JobRevertRequest) GetEnforcePriorVersionOk() (*int32, bool)`

GetEnforcePriorVersionOk returns a tuple with the EnforcePriorVersion field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEnforcePriorVersion

`func (o *JobRevertRequest) SetEnforcePriorVersion(v int32)`

SetEnforcePriorVersion sets EnforcePriorVersion field to given value.

### HasEnforcePriorVersion

`func (o *JobRevertRequest) HasEnforcePriorVersion() bool`

HasEnforcePriorVersion returns a boolean if a field has been set.

### GetJobID

`func (o *JobRevertRequest) GetJobID() string`

GetJobID returns the JobID field if non-nil, zero value otherwise.

### GetJobIDOk

`func (o *JobRevertRequest) GetJobIDOk() (*string, bool)`

GetJobIDOk returns a tuple with the JobID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetJobID

`func (o *JobRevertRequest) SetJobID(v string)`

SetJobID sets JobID field to given value.

### HasJobID

`func (o *JobRevertRequest) HasJobID() bool`

HasJobID returns a boolean if a field has been set.

### GetJobVersion

`func (o *JobRevertRequest) GetJobVersion() int32`

GetJobVersion returns the JobVersion field if non-nil, zero value otherwise.

### GetJobVersionOk

`func (o *JobRevertRequest) GetJobVersionOk() (*int32, bool)`

GetJobVersionOk returns a tuple with the JobVersion field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetJobVersion

`func (o *JobRevertRequest) SetJobVersion(v int32)`

SetJobVersion sets JobVersion field to given value.

### HasJobVersion

`func (o *JobRevertRequest) HasJobVersion() bool`

HasJobVersion returns a boolean if a field has been set.

### GetNamespace

`func (o *JobRevertRequest) GetNamespace() string`

GetNamespace returns the Namespace field if non-nil, zero value otherwise.

### GetNamespaceOk

`func (o *JobRevertRequest) GetNamespaceOk() (*string, bool)`

GetNamespaceOk returns a tuple with the Namespace field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNamespace

`func (o *JobRevertRequest) SetNamespace(v string)`

SetNamespace sets Namespace field to given value.

### HasNamespace

`func (o *JobRevertRequest) HasNamespace() bool`

HasNamespace returns a boolean if a field has been set.

### GetRegion

`func (o *JobRevertRequest) GetRegion() string`

GetRegion returns the Region field if non-nil, zero value otherwise.

### GetRegionOk

`func (o *JobRevertRequest) GetRegionOk() (*string, bool)`

GetRegionOk returns a tuple with the Region field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetRegion

`func (o *JobRevertRequest) SetRegion(v string)`

SetRegion sets Region field to given value.

### HasRegion

`func (o *JobRevertRequest) HasRegion() bool`

HasRegion returns a boolean if a field has been set.

### GetSecretID

`func (o *JobRevertRequest) GetSecretID() string`

GetSecretID returns the SecretID field if non-nil, zero value otherwise.

### GetSecretIDOk

`func (o *JobRevertRequest) GetSecretIDOk() (*string, bool)`

GetSecretIDOk returns a tuple with the SecretID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSecretID

`func (o *JobRevertRequest) SetSecretID(v string)`

SetSecretID sets SecretID field to given value.

### HasSecretID

`func (o *JobRevertRequest) HasSecretID() bool`

HasSecretID returns a boolean if a field has been set.

### GetVaultToken

`func (o *JobRevertRequest) GetVaultToken() string`

GetVaultToken returns the VaultToken field if non-nil, zero value otherwise.

### GetVaultTokenOk

`func (o *JobRevertRequest) GetVaultTokenOk() (*string, bool)`

GetVaultTokenOk returns a tuple with the VaultToken field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetVaultToken

`func (o *JobRevertRequest) SetVaultToken(v string)`

SetVaultToken sets VaultToken field to given value.

### HasVaultToken

`func (o *JobRevertRequest) HasVaultToken() bool`

HasVaultToken returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


