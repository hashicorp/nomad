# JobDiff

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Fields** | Pointer to [**[]FieldDiff**](FieldDiff.md) |  | [optional] 
**ID** | Pointer to **string** |  | [optional] 
**Objects** | Pointer to [**[]ObjectDiff**](ObjectDiff.md) |  | [optional] 
**TaskGroups** | Pointer to [**[]TaskGroupDiff**](TaskGroupDiff.md) |  | [optional] 
**Type** | Pointer to **string** |  | [optional] 

## Methods

### NewJobDiff

`func NewJobDiff() *JobDiff`

NewJobDiff instantiates a new JobDiff object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewJobDiffWithDefaults

`func NewJobDiffWithDefaults() *JobDiff`

NewJobDiffWithDefaults instantiates a new JobDiff object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetFields

`func (o *JobDiff) GetFields() []FieldDiff`

GetFields returns the Fields field if non-nil, zero value otherwise.

### GetFieldsOk

`func (o *JobDiff) GetFieldsOk() (*[]FieldDiff, bool)`

GetFieldsOk returns a tuple with the Fields field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetFields

`func (o *JobDiff) SetFields(v []FieldDiff)`

SetFields sets Fields field to given value.

### HasFields

`func (o *JobDiff) HasFields() bool`

HasFields returns a boolean if a field has been set.

### GetID

`func (o *JobDiff) GetID() string`

GetID returns the ID field if non-nil, zero value otherwise.

### GetIDOk

`func (o *JobDiff) GetIDOk() (*string, bool)`

GetIDOk returns a tuple with the ID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetID

`func (o *JobDiff) SetID(v string)`

SetID sets ID field to given value.

### HasID

`func (o *JobDiff) HasID() bool`

HasID returns a boolean if a field has been set.

### GetObjects

`func (o *JobDiff) GetObjects() []ObjectDiff`

GetObjects returns the Objects field if non-nil, zero value otherwise.

### GetObjectsOk

`func (o *JobDiff) GetObjectsOk() (*[]ObjectDiff, bool)`

GetObjectsOk returns a tuple with the Objects field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetObjects

`func (o *JobDiff) SetObjects(v []ObjectDiff)`

SetObjects sets Objects field to given value.

### HasObjects

`func (o *JobDiff) HasObjects() bool`

HasObjects returns a boolean if a field has been set.

### GetTaskGroups

`func (o *JobDiff) GetTaskGroups() []TaskGroupDiff`

GetTaskGroups returns the TaskGroups field if non-nil, zero value otherwise.

### GetTaskGroupsOk

`func (o *JobDiff) GetTaskGroupsOk() (*[]TaskGroupDiff, bool)`

GetTaskGroupsOk returns a tuple with the TaskGroups field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTaskGroups

`func (o *JobDiff) SetTaskGroups(v []TaskGroupDiff)`

SetTaskGroups sets TaskGroups field to given value.

### HasTaskGroups

`func (o *JobDiff) HasTaskGroups() bool`

HasTaskGroups returns a boolean if a field has been set.

### GetType

`func (o *JobDiff) GetType() string`

GetType returns the Type field if non-nil, zero value otherwise.

### GetTypeOk

`func (o *JobDiff) GetTypeOk() (*string, bool)`

GetTypeOk returns a tuple with the Type field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetType

`func (o *JobDiff) SetType(v string)`

SetType sets Type field to given value.

### HasType

`func (o *JobDiff) HasType() bool`

HasType returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


