# WriteRequest

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Namespace** | Pointer to **string** | Namespace is the target namespace for this write | [optional] 
**Region** | Pointer to **string** | The target region for this write | [optional] 
**SecretID** | Pointer to **string** | SecretID is the secret ID of an ACL token | [optional] 

## Methods

### NewWriteRequest

`func NewWriteRequest() *WriteRequest`

NewWriteRequest instantiates a new WriteRequest object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewWriteRequestWithDefaults

`func NewWriteRequestWithDefaults() *WriteRequest`

NewWriteRequestWithDefaults instantiates a new WriteRequest object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetNamespace

`func (o *WriteRequest) GetNamespace() string`

GetNamespace returns the Namespace field if non-nil, zero value otherwise.

### GetNamespaceOk

`func (o *WriteRequest) GetNamespaceOk() (*string, bool)`

GetNamespaceOk returns a tuple with the Namespace field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNamespace

`func (o *WriteRequest) SetNamespace(v string)`

SetNamespace sets Namespace field to given value.

### HasNamespace

`func (o *WriteRequest) HasNamespace() bool`

HasNamespace returns a boolean if a field has been set.

### GetRegion

`func (o *WriteRequest) GetRegion() string`

GetRegion returns the Region field if non-nil, zero value otherwise.

### GetRegionOk

`func (o *WriteRequest) GetRegionOk() (*string, bool)`

GetRegionOk returns a tuple with the Region field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetRegion

`func (o *WriteRequest) SetRegion(v string)`

SetRegion sets Region field to given value.

### HasRegion

`func (o *WriteRequest) HasRegion() bool`

HasRegion returns a boolean if a field has been set.

### GetSecretID

`func (o *WriteRequest) GetSecretID() string`

GetSecretID returns the SecretID field if non-nil, zero value otherwise.

### GetSecretIDOk

`func (o *WriteRequest) GetSecretIDOk() (*string, bool)`

GetSecretIDOk returns a tuple with the SecretID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSecretID

`func (o *WriteRequest) SetSecretID(v string)`

SetSecretID sets SecretID field to given value.

### HasSecretID

`func (o *WriteRequest) HasSecretID() bool`

HasSecretID returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


