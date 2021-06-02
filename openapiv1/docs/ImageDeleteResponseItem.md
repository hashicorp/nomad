# ImageDeleteResponseItem

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Deleted** | Pointer to **string** | The image ID of an image that was deleted | [optional] 
**Untagged** | Pointer to **string** | The image ID of an image that was untagged | [optional] 

## Methods

### NewImageDeleteResponseItem

`func NewImageDeleteResponseItem() *ImageDeleteResponseItem`

NewImageDeleteResponseItem instantiates a new ImageDeleteResponseItem object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewImageDeleteResponseItemWithDefaults

`func NewImageDeleteResponseItemWithDefaults() *ImageDeleteResponseItem`

NewImageDeleteResponseItemWithDefaults instantiates a new ImageDeleteResponseItem object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetDeleted

`func (o *ImageDeleteResponseItem) GetDeleted() string`

GetDeleted returns the Deleted field if non-nil, zero value otherwise.

### GetDeletedOk

`func (o *ImageDeleteResponseItem) GetDeletedOk() (*string, bool)`

GetDeletedOk returns a tuple with the Deleted field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDeleted

`func (o *ImageDeleteResponseItem) SetDeleted(v string)`

SetDeleted sets Deleted field to given value.

### HasDeleted

`func (o *ImageDeleteResponseItem) HasDeleted() bool`

HasDeleted returns a boolean if a field has been set.

### GetUntagged

`func (o *ImageDeleteResponseItem) GetUntagged() string`

GetUntagged returns the Untagged field if non-nil, zero value otherwise.

### GetUntaggedOk

`func (o *ImageDeleteResponseItem) GetUntaggedOk() (*string, bool)`

GetUntaggedOk returns a tuple with the Untagged field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetUntagged

`func (o *ImageDeleteResponseItem) SetUntagged(v string)`

SetUntagged sets Untagged field to given value.

### HasUntagged

`func (o *ImageDeleteResponseItem) HasUntagged() bool`

HasUntagged returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


