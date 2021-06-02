# ObjectDiff

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Fields** | Pointer to [**[]FieldDiff**](FieldDiff.md) |  | [optional] 
**Name** | Pointer to **string** |  | [optional] 
**Objects** | Pointer to [**[]ObjectDiff**](ObjectDiff.md) |  | [optional] 
**Type** | Pointer to **string** |  | [optional] 

## Methods

### NewObjectDiff

`func NewObjectDiff() *ObjectDiff`

NewObjectDiff instantiates a new ObjectDiff object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewObjectDiffWithDefaults

`func NewObjectDiffWithDefaults() *ObjectDiff`

NewObjectDiffWithDefaults instantiates a new ObjectDiff object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetFields

`func (o *ObjectDiff) GetFields() []FieldDiff`

GetFields returns the Fields field if non-nil, zero value otherwise.

### GetFieldsOk

`func (o *ObjectDiff) GetFieldsOk() (*[]FieldDiff, bool)`

GetFieldsOk returns a tuple with the Fields field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetFields

`func (o *ObjectDiff) SetFields(v []FieldDiff)`

SetFields sets Fields field to given value.

### HasFields

`func (o *ObjectDiff) HasFields() bool`

HasFields returns a boolean if a field has been set.

### GetName

`func (o *ObjectDiff) GetName() string`

GetName returns the Name field if non-nil, zero value otherwise.

### GetNameOk

`func (o *ObjectDiff) GetNameOk() (*string, bool)`

GetNameOk returns a tuple with the Name field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetName

`func (o *ObjectDiff) SetName(v string)`

SetName sets Name field to given value.

### HasName

`func (o *ObjectDiff) HasName() bool`

HasName returns a boolean if a field has been set.

### GetObjects

`func (o *ObjectDiff) GetObjects() []ObjectDiff`

GetObjects returns the Objects field if non-nil, zero value otherwise.

### GetObjectsOk

`func (o *ObjectDiff) GetObjectsOk() (*[]ObjectDiff, bool)`

GetObjectsOk returns a tuple with the Objects field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetObjects

`func (o *ObjectDiff) SetObjects(v []ObjectDiff)`

SetObjects sets Objects field to given value.

### HasObjects

`func (o *ObjectDiff) HasObjects() bool`

HasObjects returns a boolean if a field has been set.

### GetType

`func (o *ObjectDiff) GetType() string`

GetType returns the Type field if non-nil, zero value otherwise.

### GetTypeOk

`func (o *ObjectDiff) GetTypeOk() (*string, bool)`

GetTypeOk returns a tuple with the Type field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetType

`func (o *ObjectDiff) SetType(v string)`

SetType sets Type field to given value.

### HasType

`func (o *ObjectDiff) HasType() bool`

HasType returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


