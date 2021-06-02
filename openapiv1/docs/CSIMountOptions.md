# CSIMountOptions

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**FSType** | Pointer to **string** | FSType is an optional field that allows an operator to specify the type of the filesystem. | [optional] 
**MountFlags** | Pointer to **[]string** | MountFlags contains additional options that may be used when mounting the volume by the plugin. This may contain sensitive data and should not be leaked. | [optional] 

## Methods

### NewCSIMountOptions

`func NewCSIMountOptions() *CSIMountOptions`

NewCSIMountOptions instantiates a new CSIMountOptions object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewCSIMountOptionsWithDefaults

`func NewCSIMountOptionsWithDefaults() *CSIMountOptions`

NewCSIMountOptionsWithDefaults instantiates a new CSIMountOptions object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetFSType

`func (o *CSIMountOptions) GetFSType() string`

GetFSType returns the FSType field if non-nil, zero value otherwise.

### GetFSTypeOk

`func (o *CSIMountOptions) GetFSTypeOk() (*string, bool)`

GetFSTypeOk returns a tuple with the FSType field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetFSType

`func (o *CSIMountOptions) SetFSType(v string)`

SetFSType sets FSType field to given value.

### HasFSType

`func (o *CSIMountOptions) HasFSType() bool`

HasFSType returns a boolean if a field has been set.

### GetMountFlags

`func (o *CSIMountOptions) GetMountFlags() []string`

GetMountFlags returns the MountFlags field if non-nil, zero value otherwise.

### GetMountFlagsOk

`func (o *CSIMountOptions) GetMountFlagsOk() (*[]string, bool)`

GetMountFlagsOk returns a tuple with the MountFlags field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMountFlags

`func (o *CSIMountOptions) SetMountFlags(v []string)`

SetMountFlags sets MountFlags field to given value.

### HasMountFlags

`func (o *CSIMountOptions) HasMountFlags() bool`

HasMountFlags returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


