# Vault

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**ChangeMode** | Pointer to **string** |  | [optional] 
**ChangeSignal** | Pointer to **string** |  | [optional] 
**Env** | Pointer to **bool** |  | [optional] 
**Namespace** | Pointer to **string** |  | [optional] 
**Policies** | Pointer to **[]string** |  | [optional] 

## Methods

### NewVault

`func NewVault() *Vault`

NewVault instantiates a new Vault object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewVaultWithDefaults

`func NewVaultWithDefaults() *Vault`

NewVaultWithDefaults instantiates a new Vault object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetChangeMode

`func (o *Vault) GetChangeMode() string`

GetChangeMode returns the ChangeMode field if non-nil, zero value otherwise.

### GetChangeModeOk

`func (o *Vault) GetChangeModeOk() (*string, bool)`

GetChangeModeOk returns a tuple with the ChangeMode field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetChangeMode

`func (o *Vault) SetChangeMode(v string)`

SetChangeMode sets ChangeMode field to given value.

### HasChangeMode

`func (o *Vault) HasChangeMode() bool`

HasChangeMode returns a boolean if a field has been set.

### GetChangeSignal

`func (o *Vault) GetChangeSignal() string`

GetChangeSignal returns the ChangeSignal field if non-nil, zero value otherwise.

### GetChangeSignalOk

`func (o *Vault) GetChangeSignalOk() (*string, bool)`

GetChangeSignalOk returns a tuple with the ChangeSignal field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetChangeSignal

`func (o *Vault) SetChangeSignal(v string)`

SetChangeSignal sets ChangeSignal field to given value.

### HasChangeSignal

`func (o *Vault) HasChangeSignal() bool`

HasChangeSignal returns a boolean if a field has been set.

### GetEnv

`func (o *Vault) GetEnv() bool`

GetEnv returns the Env field if non-nil, zero value otherwise.

### GetEnvOk

`func (o *Vault) GetEnvOk() (*bool, bool)`

GetEnvOk returns a tuple with the Env field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEnv

`func (o *Vault) SetEnv(v bool)`

SetEnv sets Env field to given value.

### HasEnv

`func (o *Vault) HasEnv() bool`

HasEnv returns a boolean if a field has been set.

### GetNamespace

`func (o *Vault) GetNamespace() string`

GetNamespace returns the Namespace field if non-nil, zero value otherwise.

### GetNamespaceOk

`func (o *Vault) GetNamespaceOk() (*string, bool)`

GetNamespaceOk returns a tuple with the Namespace field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNamespace

`func (o *Vault) SetNamespace(v string)`

SetNamespace sets Namespace field to given value.

### HasNamespace

`func (o *Vault) HasNamespace() bool`

HasNamespace returns a boolean if a field has been set.

### GetPolicies

`func (o *Vault) GetPolicies() []string`

GetPolicies returns the Policies field if non-nil, zero value otherwise.

### GetPoliciesOk

`func (o *Vault) GetPoliciesOk() (*[]string, bool)`

GetPoliciesOk returns a tuple with the Policies field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPolicies

`func (o *Vault) SetPolicies(v []string)`

SetPolicies sets Policies field to given value.

### HasPolicies

`func (o *Vault) HasPolicies() bool`

HasPolicies returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


