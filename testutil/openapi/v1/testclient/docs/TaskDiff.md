# TaskDiff

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Annotations** | Pointer to **[]string** |  | [optional] 
**Fields** | Pointer to [**[]FieldDiff**](FieldDiff.md) |  | [optional] 
**Name** | Pointer to **string** |  | [optional] 
**Objects** | Pointer to [**[]ObjectDiff**](ObjectDiff.md) |  | [optional] 
**Type** | Pointer to **string** |  | [optional] 

## Methods

### NewTaskDiff

`func NewTaskDiff() *TaskDiff`

NewTaskDiff instantiates a new TaskDiff object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewTaskDiffWithDefaults

`func NewTaskDiffWithDefaults() *TaskDiff`

NewTaskDiffWithDefaults instantiates a new TaskDiff object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetAnnotations

`func (o *TaskDiff) GetAnnotations() []string`

GetAnnotations returns the Annotations field if non-nil, zero value otherwise.

### GetAnnotationsOk

`func (o *TaskDiff) GetAnnotationsOk() (*[]string, bool)`

GetAnnotationsOk returns a tuple with the Annotations field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAnnotations

`func (o *TaskDiff) SetAnnotations(v []string)`

SetAnnotations sets Annotations field to given value.

### HasAnnotations

`func (o *TaskDiff) HasAnnotations() bool`

HasAnnotations returns a boolean if a field has been set.

### GetFields

`func (o *TaskDiff) GetFields() []FieldDiff`

GetFields returns the Fields field if non-nil, zero value otherwise.

### GetFieldsOk

`func (o *TaskDiff) GetFieldsOk() (*[]FieldDiff, bool)`

GetFieldsOk returns a tuple with the Fields field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetFields

`func (o *TaskDiff) SetFields(v []FieldDiff)`

SetFields sets Fields field to given value.

### HasFields

`func (o *TaskDiff) HasFields() bool`

HasFields returns a boolean if a field has been set.

### GetName

`func (o *TaskDiff) GetName() string`

GetName returns the Name field if non-nil, zero value otherwise.

### GetNameOk

`func (o *TaskDiff) GetNameOk() (*string, bool)`

GetNameOk returns a tuple with the Name field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetName

`func (o *TaskDiff) SetName(v string)`

SetName sets Name field to given value.

### HasName

`func (o *TaskDiff) HasName() bool`

HasName returns a boolean if a field has been set.

### GetObjects

`func (o *TaskDiff) GetObjects() []ObjectDiff`

GetObjects returns the Objects field if non-nil, zero value otherwise.

### GetObjectsOk

`func (o *TaskDiff) GetObjectsOk() (*[]ObjectDiff, bool)`

GetObjectsOk returns a tuple with the Objects field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetObjects

`func (o *TaskDiff) SetObjects(v []ObjectDiff)`

SetObjects sets Objects field to given value.

### HasObjects

`func (o *TaskDiff) HasObjects() bool`

HasObjects returns a boolean if a field has been set.

### GetType

`func (o *TaskDiff) GetType() string`

GetType returns the Type field if non-nil, zero value otherwise.

### GetTypeOk

`func (o *TaskDiff) GetTypeOk() (*string, bool)`

GetTypeOk returns a tuple with the Type field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetType

`func (o *TaskDiff) SetType(v string)`

SetType sets Type field to given value.

### HasType

`func (o *TaskDiff) HasType() bool`

HasType returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


