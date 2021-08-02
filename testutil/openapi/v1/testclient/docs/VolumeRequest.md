# VolumeRequest

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**AccessMode** | Pointer to **string** |  | [optional] 
**AttachmentMode** | Pointer to **string** |  | [optional] 
**MountOptions** | Pointer to [**CSIMountOptions**](CSIMountOptions.md) |  | [optional] 
**Name** | Pointer to **string** |  | [optional] 
**PerAlloc** | Pointer to **bool** |  | [optional] 
**ReadOnly** | Pointer to **bool** |  | [optional] 
**Source** | Pointer to **string** |  | [optional] 
**Type** | Pointer to **string** |  | [optional] 

## Methods

### NewVolumeRequest

`func NewVolumeRequest() *VolumeRequest`

NewVolumeRequest instantiates a new VolumeRequest object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewVolumeRequestWithDefaults

`func NewVolumeRequestWithDefaults() *VolumeRequest`

NewVolumeRequestWithDefaults instantiates a new VolumeRequest object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetAccessMode

`func (o *VolumeRequest) GetAccessMode() string`

GetAccessMode returns the AccessMode field if non-nil, zero value otherwise.

### GetAccessModeOk

`func (o *VolumeRequest) GetAccessModeOk() (*string, bool)`

GetAccessModeOk returns a tuple with the AccessMode field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAccessMode

`func (o *VolumeRequest) SetAccessMode(v string)`

SetAccessMode sets AccessMode field to given value.

### HasAccessMode

`func (o *VolumeRequest) HasAccessMode() bool`

HasAccessMode returns a boolean if a field has been set.

### GetAttachmentMode

`func (o *VolumeRequest) GetAttachmentMode() string`

GetAttachmentMode returns the AttachmentMode field if non-nil, zero value otherwise.

### GetAttachmentModeOk

`func (o *VolumeRequest) GetAttachmentModeOk() (*string, bool)`

GetAttachmentModeOk returns a tuple with the AttachmentMode field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAttachmentMode

`func (o *VolumeRequest) SetAttachmentMode(v string)`

SetAttachmentMode sets AttachmentMode field to given value.

### HasAttachmentMode

`func (o *VolumeRequest) HasAttachmentMode() bool`

HasAttachmentMode returns a boolean if a field has been set.

### GetMountOptions

`func (o *VolumeRequest) GetMountOptions() CSIMountOptions`

GetMountOptions returns the MountOptions field if non-nil, zero value otherwise.

### GetMountOptionsOk

`func (o *VolumeRequest) GetMountOptionsOk() (*CSIMountOptions, bool)`

GetMountOptionsOk returns a tuple with the MountOptions field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMountOptions

`func (o *VolumeRequest) SetMountOptions(v CSIMountOptions)`

SetMountOptions sets MountOptions field to given value.

### HasMountOptions

`func (o *VolumeRequest) HasMountOptions() bool`

HasMountOptions returns a boolean if a field has been set.

### GetName

`func (o *VolumeRequest) GetName() string`

GetName returns the Name field if non-nil, zero value otherwise.

### GetNameOk

`func (o *VolumeRequest) GetNameOk() (*string, bool)`

GetNameOk returns a tuple with the Name field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetName

`func (o *VolumeRequest) SetName(v string)`

SetName sets Name field to given value.

### HasName

`func (o *VolumeRequest) HasName() bool`

HasName returns a boolean if a field has been set.

### GetPerAlloc

`func (o *VolumeRequest) GetPerAlloc() bool`

GetPerAlloc returns the PerAlloc field if non-nil, zero value otherwise.

### GetPerAllocOk

`func (o *VolumeRequest) GetPerAllocOk() (*bool, bool)`

GetPerAllocOk returns a tuple with the PerAlloc field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPerAlloc

`func (o *VolumeRequest) SetPerAlloc(v bool)`

SetPerAlloc sets PerAlloc field to given value.

### HasPerAlloc

`func (o *VolumeRequest) HasPerAlloc() bool`

HasPerAlloc returns a boolean if a field has been set.

### GetReadOnly

`func (o *VolumeRequest) GetReadOnly() bool`

GetReadOnly returns the ReadOnly field if non-nil, zero value otherwise.

### GetReadOnlyOk

`func (o *VolumeRequest) GetReadOnlyOk() (*bool, bool)`

GetReadOnlyOk returns a tuple with the ReadOnly field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetReadOnly

`func (o *VolumeRequest) SetReadOnly(v bool)`

SetReadOnly sets ReadOnly field to given value.

### HasReadOnly

`func (o *VolumeRequest) HasReadOnly() bool`

HasReadOnly returns a boolean if a field has been set.

### GetSource

`func (o *VolumeRequest) GetSource() string`

GetSource returns the Source field if non-nil, zero value otherwise.

### GetSourceOk

`func (o *VolumeRequest) GetSourceOk() (*string, bool)`

GetSourceOk returns a tuple with the Source field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSource

`func (o *VolumeRequest) SetSource(v string)`

SetSource sets Source field to given value.

### HasSource

`func (o *VolumeRequest) HasSource() bool`

HasSource returns a boolean if a field has been set.

### GetType

`func (o *VolumeRequest) GetType() string`

GetType returns the Type field if non-nil, zero value otherwise.

### GetTypeOk

`func (o *VolumeRequest) GetTypeOk() (*string, bool)`

GetTypeOk returns a tuple with the Type field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetType

`func (o *VolumeRequest) SetType(v string)`

SetType sets Type field to given value.

### HasType

`func (o *VolumeRequest) HasType() bool`

HasType returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


