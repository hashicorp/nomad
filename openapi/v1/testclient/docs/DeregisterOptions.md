# DeregisterOptions

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Global** | Pointer to **bool** | If Global is set to true, all regions of a multiregion job will be stopped. | [optional] 
**Purge** | Pointer to **bool** | If Purge is set to true, the job is deregistered and purged from the system versus still being queryable and eventually GC&#39;ed from the system. Most callers should not specify purge. | [optional] 

## Methods

### NewDeregisterOptions

`func NewDeregisterOptions() *DeregisterOptions`

NewDeregisterOptions instantiates a new DeregisterOptions object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewDeregisterOptionsWithDefaults

`func NewDeregisterOptionsWithDefaults() *DeregisterOptions`

NewDeregisterOptionsWithDefaults instantiates a new DeregisterOptions object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetGlobal

`func (o *DeregisterOptions) GetGlobal() bool`

GetGlobal returns the Global field if non-nil, zero value otherwise.

### GetGlobalOk

`func (o *DeregisterOptions) GetGlobalOk() (*bool, bool)`

GetGlobalOk returns a tuple with the Global field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetGlobal

`func (o *DeregisterOptions) SetGlobal(v bool)`

SetGlobal sets Global field to given value.

### HasGlobal

`func (o *DeregisterOptions) HasGlobal() bool`

HasGlobal returns a boolean if a field has been set.

### GetPurge

`func (o *DeregisterOptions) GetPurge() bool`

GetPurge returns the Purge field if non-nil, zero value otherwise.

### GetPurgeOk

`func (o *DeregisterOptions) GetPurgeOk() (*bool, bool)`

GetPurgeOk returns a tuple with the Purge field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPurge

`func (o *DeregisterOptions) SetPurge(v bool)`

SetPurge sets Purge field to given value.

### HasPurge

`func (o *DeregisterOptions) HasPurge() bool`

HasPurge returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


