# AllocatedResources

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Shared** | Pointer to [**AllocatedSharedResources**](AllocatedSharedResources.md) |  | [optional] 
**Tasks** | Pointer to [**map[string]AllocatedTaskResources**](AllocatedTaskResources.md) |  | [optional] 

## Methods

### NewAllocatedResources

`func NewAllocatedResources() *AllocatedResources`

NewAllocatedResources instantiates a new AllocatedResources object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewAllocatedResourcesWithDefaults

`func NewAllocatedResourcesWithDefaults() *AllocatedResources`

NewAllocatedResourcesWithDefaults instantiates a new AllocatedResources object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetShared

`func (o *AllocatedResources) GetShared() AllocatedSharedResources`

GetShared returns the Shared field if non-nil, zero value otherwise.

### GetSharedOk

`func (o *AllocatedResources) GetSharedOk() (*AllocatedSharedResources, bool)`

GetSharedOk returns a tuple with the Shared field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetShared

`func (o *AllocatedResources) SetShared(v AllocatedSharedResources)`

SetShared sets Shared field to given value.

### HasShared

`func (o *AllocatedResources) HasShared() bool`

HasShared returns a boolean if a field has been set.

### GetTasks

`func (o *AllocatedResources) GetTasks() map[string]AllocatedTaskResources`

GetTasks returns the Tasks field if non-nil, zero value otherwise.

### GetTasksOk

`func (o *AllocatedResources) GetTasksOk() (*map[string]AllocatedTaskResources, bool)`

GetTasksOk returns a tuple with the Tasks field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTasks

`func (o *AllocatedResources) SetTasks(v map[string]AllocatedTaskResources)`

SetTasks sets Tasks field to given value.

### HasTasks

`func (o *AllocatedResources) HasTasks() bool`

HasTasks returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


