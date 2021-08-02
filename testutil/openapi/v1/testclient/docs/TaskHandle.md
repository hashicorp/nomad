# TaskHandle

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**DriverState** | Pointer to **string** |  | [optional] 
**Version** | Pointer to **int32** |  | [optional] 

## Methods

### NewTaskHandle

`func NewTaskHandle() *TaskHandle`

NewTaskHandle instantiates a new TaskHandle object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewTaskHandleWithDefaults

`func NewTaskHandleWithDefaults() *TaskHandle`

NewTaskHandleWithDefaults instantiates a new TaskHandle object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetDriverState

`func (o *TaskHandle) GetDriverState() string`

GetDriverState returns the DriverState field if non-nil, zero value otherwise.

### GetDriverStateOk

`func (o *TaskHandle) GetDriverStateOk() (*string, bool)`

GetDriverStateOk returns a tuple with the DriverState field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDriverState

`func (o *TaskHandle) SetDriverState(v string)`

SetDriverState sets DriverState field to given value.

### HasDriverState

`func (o *TaskHandle) HasDriverState() bool`

HasDriverState returns a boolean if a field has been set.

### GetVersion

`func (o *TaskHandle) GetVersion() int32`

GetVersion returns the Version field if non-nil, zero value otherwise.

### GetVersionOk

`func (o *TaskHandle) GetVersionOk() (*int32, bool)`

GetVersionOk returns a tuple with the Version field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetVersion

`func (o *TaskHandle) SetVersion(v int32)`

SetVersion sets Version field to given value.

### HasVersion

`func (o *TaskHandle) HasVersion() bool`

HasVersion returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


