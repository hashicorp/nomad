# TaskLifecycle

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Hook** | Pointer to **string** |  | [optional] 
**Sidecar** | Pointer to **bool** |  | [optional] 

## Methods

### NewTaskLifecycle

`func NewTaskLifecycle() *TaskLifecycle`

NewTaskLifecycle instantiates a new TaskLifecycle object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewTaskLifecycleWithDefaults

`func NewTaskLifecycleWithDefaults() *TaskLifecycle`

NewTaskLifecycleWithDefaults instantiates a new TaskLifecycle object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetHook

`func (o *TaskLifecycle) GetHook() string`

GetHook returns the Hook field if non-nil, zero value otherwise.

### GetHookOk

`func (o *TaskLifecycle) GetHookOk() (*string, bool)`

GetHookOk returns a tuple with the Hook field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetHook

`func (o *TaskLifecycle) SetHook(v string)`

SetHook sets Hook field to given value.

### HasHook

`func (o *TaskLifecycle) HasHook() bool`

HasHook returns a boolean if a field has been set.

### GetSidecar

`func (o *TaskLifecycle) GetSidecar() bool`

GetSidecar returns the Sidecar field if non-nil, zero value otherwise.

### GetSidecarOk

`func (o *TaskLifecycle) GetSidecarOk() (*bool, bool)`

GetSidecarOk returns a tuple with the Sidecar field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSidecar

`func (o *TaskLifecycle) SetSidecar(v bool)`

SetSidecar sets Sidecar field to given value.

### HasSidecar

`func (o *TaskLifecycle) HasSidecar() bool`

HasSidecar returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


