# ContainerChangeResponseItem

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Kind** | **int32** | Kind of change | 
**Path** | **string** | Path to file that has changed | 

## Methods

### NewContainerChangeResponseItem

`func NewContainerChangeResponseItem(kind int32, path string, ) *ContainerChangeResponseItem`

NewContainerChangeResponseItem instantiates a new ContainerChangeResponseItem object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewContainerChangeResponseItemWithDefaults

`func NewContainerChangeResponseItemWithDefaults() *ContainerChangeResponseItem`

NewContainerChangeResponseItemWithDefaults instantiates a new ContainerChangeResponseItem object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetKind

`func (o *ContainerChangeResponseItem) GetKind() int32`

GetKind returns the Kind field if non-nil, zero value otherwise.

### GetKindOk

`func (o *ContainerChangeResponseItem) GetKindOk() (*int32, bool)`

GetKindOk returns a tuple with the Kind field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetKind

`func (o *ContainerChangeResponseItem) SetKind(v int32)`

SetKind sets Kind field to given value.


### GetPath

`func (o *ContainerChangeResponseItem) GetPath() string`

GetPath returns the Path field if non-nil, zero value otherwise.

### GetPathOk

`func (o *ContainerChangeResponseItem) GetPathOk() (*string, bool)`

GetPathOk returns a tuple with the Path field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPath

`func (o *ContainerChangeResponseItem) SetPath(v string)`

SetPath sets Path field to given value.



[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


