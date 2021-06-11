# TaskLifecycleConfig

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Hook** | Pointer to **string** |  | [optional] 
**Sidecar** | Pointer to **bool** |  | [optional] 

## Methods

### NewTaskLifecycleConfig

`func NewTaskLifecycleConfig() *TaskLifecycleConfig`

NewTaskLifecycleConfig instantiates a new TaskLifecycleConfig object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewTaskLifecycleConfigWithDefaults

`func NewTaskLifecycleConfigWithDefaults() *TaskLifecycleConfig`

NewTaskLifecycleConfigWithDefaults instantiates a new TaskLifecycleConfig object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetHook

`func (o *TaskLifecycleConfig) GetHook() string`

GetHook returns the Hook field if non-nil, zero value otherwise.

### GetHookOk

`func (o *TaskLifecycleConfig) GetHookOk() (*string, bool)`

GetHookOk returns a tuple with the Hook field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetHook

`func (o *TaskLifecycleConfig) SetHook(v string)`

SetHook sets Hook field to given value.

### HasHook

`func (o *TaskLifecycleConfig) HasHook() bool`

HasHook returns a boolean if a field has been set.

### GetSidecar

`func (o *TaskLifecycleConfig) GetSidecar() bool`

GetSidecar returns the Sidecar field if non-nil, zero value otherwise.

### GetSidecarOk

`func (o *TaskLifecycleConfig) GetSidecarOk() (*bool, bool)`

GetSidecarOk returns a tuple with the Sidecar field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSidecar

`func (o *TaskLifecycleConfig) SetSidecar(v bool)`

SetSidecar sets Sidecar field to given value.

### HasSidecar

`func (o *TaskLifecycleConfig) HasSidecar() bool`

HasSidecar returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


