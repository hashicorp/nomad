# TaskGroupDiff

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Fields** | Pointer to [**[]FieldDiff**](FieldDiff.md) |  | [optional] 
**Name** | Pointer to **string** |  | [optional] 
**Objects** | Pointer to [**[]ObjectDiff**](ObjectDiff.md) |  | [optional] 
**Tasks** | Pointer to [**[]TaskDiff**](TaskDiff.md) |  | [optional] 
**Type** | Pointer to **string** |  | [optional] 
**Updates** | Pointer to **map[string]int32** |  | [optional] 

## Methods

### NewTaskGroupDiff

`func NewTaskGroupDiff() *TaskGroupDiff`

NewTaskGroupDiff instantiates a new TaskGroupDiff object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewTaskGroupDiffWithDefaults

`func NewTaskGroupDiffWithDefaults() *TaskGroupDiff`

NewTaskGroupDiffWithDefaults instantiates a new TaskGroupDiff object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetFields

`func (o *TaskGroupDiff) GetFields() []FieldDiff`

GetFields returns the Fields field if non-nil, zero value otherwise.

### GetFieldsOk

`func (o *TaskGroupDiff) GetFieldsOk() (*[]FieldDiff, bool)`

GetFieldsOk returns a tuple with the Fields field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetFields

`func (o *TaskGroupDiff) SetFields(v []FieldDiff)`

SetFields sets Fields field to given value.

### HasFields

`func (o *TaskGroupDiff) HasFields() bool`

HasFields returns a boolean if a field has been set.

### GetName

`func (o *TaskGroupDiff) GetName() string`

GetName returns the Name field if non-nil, zero value otherwise.

### GetNameOk

`func (o *TaskGroupDiff) GetNameOk() (*string, bool)`

GetNameOk returns a tuple with the Name field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetName

`func (o *TaskGroupDiff) SetName(v string)`

SetName sets Name field to given value.

### HasName

`func (o *TaskGroupDiff) HasName() bool`

HasName returns a boolean if a field has been set.

### GetObjects

`func (o *TaskGroupDiff) GetObjects() []ObjectDiff`

GetObjects returns the Objects field if non-nil, zero value otherwise.

### GetObjectsOk

`func (o *TaskGroupDiff) GetObjectsOk() (*[]ObjectDiff, bool)`

GetObjectsOk returns a tuple with the Objects field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetObjects

`func (o *TaskGroupDiff) SetObjects(v []ObjectDiff)`

SetObjects sets Objects field to given value.

### HasObjects

`func (o *TaskGroupDiff) HasObjects() bool`

HasObjects returns a boolean if a field has been set.

### GetTasks

`func (o *TaskGroupDiff) GetTasks() []TaskDiff`

GetTasks returns the Tasks field if non-nil, zero value otherwise.

### GetTasksOk

`func (o *TaskGroupDiff) GetTasksOk() (*[]TaskDiff, bool)`

GetTasksOk returns a tuple with the Tasks field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTasks

`func (o *TaskGroupDiff) SetTasks(v []TaskDiff)`

SetTasks sets Tasks field to given value.

### HasTasks

`func (o *TaskGroupDiff) HasTasks() bool`

HasTasks returns a boolean if a field has been set.

### GetType

`func (o *TaskGroupDiff) GetType() string`

GetType returns the Type field if non-nil, zero value otherwise.

### GetTypeOk

`func (o *TaskGroupDiff) GetTypeOk() (*string, bool)`

GetTypeOk returns a tuple with the Type field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetType

`func (o *TaskGroupDiff) SetType(v string)`

SetType sets Type field to given value.

### HasType

`func (o *TaskGroupDiff) HasType() bool`

HasType returns a boolean if a field has been set.

### GetUpdates

`func (o *TaskGroupDiff) GetUpdates() map[string]int32`

GetUpdates returns the Updates field if non-nil, zero value otherwise.

### GetUpdatesOk

`func (o *TaskGroupDiff) GetUpdatesOk() (*map[string]int32, bool)`

GetUpdatesOk returns a tuple with the Updates field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetUpdates

`func (o *TaskGroupDiff) SetUpdates(v map[string]int32)`

SetUpdates sets Updates field to given value.

### HasUpdates

`func (o *TaskGroupDiff) HasUpdates() bool`

HasUpdates returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


