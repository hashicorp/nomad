# VolumeMount

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Destination** | Pointer to **string** |  | [optional] 
**PropagationMode** | Pointer to **string** |  | [optional] 
**ReadOnly** | Pointer to **bool** |  | [optional] 
**Volume** | Pointer to **string** |  | [optional] 

## Methods

### NewVolumeMount

`func NewVolumeMount() *VolumeMount`

NewVolumeMount instantiates a new VolumeMount object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewVolumeMountWithDefaults

`func NewVolumeMountWithDefaults() *VolumeMount`

NewVolumeMountWithDefaults instantiates a new VolumeMount object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetDestination

`func (o *VolumeMount) GetDestination() string`

GetDestination returns the Destination field if non-nil, zero value otherwise.

### GetDestinationOk

`func (o *VolumeMount) GetDestinationOk() (*string, bool)`

GetDestinationOk returns a tuple with the Destination field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDestination

`func (o *VolumeMount) SetDestination(v string)`

SetDestination sets Destination field to given value.

### HasDestination

`func (o *VolumeMount) HasDestination() bool`

HasDestination returns a boolean if a field has been set.

### GetPropagationMode

`func (o *VolumeMount) GetPropagationMode() string`

GetPropagationMode returns the PropagationMode field if non-nil, zero value otherwise.

### GetPropagationModeOk

`func (o *VolumeMount) GetPropagationModeOk() (*string, bool)`

GetPropagationModeOk returns a tuple with the PropagationMode field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPropagationMode

`func (o *VolumeMount) SetPropagationMode(v string)`

SetPropagationMode sets PropagationMode field to given value.

### HasPropagationMode

`func (o *VolumeMount) HasPropagationMode() bool`

HasPropagationMode returns a boolean if a field has been set.

### GetReadOnly

`func (o *VolumeMount) GetReadOnly() bool`

GetReadOnly returns the ReadOnly field if non-nil, zero value otherwise.

### GetReadOnlyOk

`func (o *VolumeMount) GetReadOnlyOk() (*bool, bool)`

GetReadOnlyOk returns a tuple with the ReadOnly field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetReadOnly

`func (o *VolumeMount) SetReadOnly(v bool)`

SetReadOnly sets ReadOnly field to given value.

### HasReadOnly

`func (o *VolumeMount) HasReadOnly() bool`

HasReadOnly returns a boolean if a field has been set.

### GetVolume

`func (o *VolumeMount) GetVolume() string`

GetVolume returns the Volume field if non-nil, zero value otherwise.

### GetVolumeOk

`func (o *VolumeMount) GetVolumeOk() (*string, bool)`

GetVolumeOk returns a tuple with the Volume field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetVolume

`func (o *VolumeMount) SetVolume(v string)`

SetVolume sets Volume field to given value.

### HasVolume

`func (o *VolumeMount) HasVolume() bool`

HasVolume returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


