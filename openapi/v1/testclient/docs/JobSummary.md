# JobSummary

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Children** | Pointer to [**JobChildrenSummary**](JobChildrenSummary.md) |  | [optional] 
**CreateIndex** | Pointer to **int32** | Raft Indexes | [optional] 
**JobID** | Pointer to **string** |  | [optional] 
**ModifyIndex** | Pointer to **int32** |  | [optional] 
**Namespace** | Pointer to **string** |  | [optional] 
**Summary** | Pointer to [**map[string]TaskGroupSummary**](TaskGroupSummary.md) |  | [optional] 

## Methods

### NewJobSummary

`func NewJobSummary() *JobSummary`

NewJobSummary instantiates a new JobSummary object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewJobSummaryWithDefaults

`func NewJobSummaryWithDefaults() *JobSummary`

NewJobSummaryWithDefaults instantiates a new JobSummary object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetChildren

`func (o *JobSummary) GetChildren() JobChildrenSummary`

GetChildren returns the Children field if non-nil, zero value otherwise.

### GetChildrenOk

`func (o *JobSummary) GetChildrenOk() (*JobChildrenSummary, bool)`

GetChildrenOk returns a tuple with the Children field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetChildren

`func (o *JobSummary) SetChildren(v JobChildrenSummary)`

SetChildren sets Children field to given value.

### HasChildren

`func (o *JobSummary) HasChildren() bool`

HasChildren returns a boolean if a field has been set.

### GetCreateIndex

`func (o *JobSummary) GetCreateIndex() int32`

GetCreateIndex returns the CreateIndex field if non-nil, zero value otherwise.

### GetCreateIndexOk

`func (o *JobSummary) GetCreateIndexOk() (*int32, bool)`

GetCreateIndexOk returns a tuple with the CreateIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCreateIndex

`func (o *JobSummary) SetCreateIndex(v int32)`

SetCreateIndex sets CreateIndex field to given value.

### HasCreateIndex

`func (o *JobSummary) HasCreateIndex() bool`

HasCreateIndex returns a boolean if a field has been set.

### GetJobID

`func (o *JobSummary) GetJobID() string`

GetJobID returns the JobID field if non-nil, zero value otherwise.

### GetJobIDOk

`func (o *JobSummary) GetJobIDOk() (*string, bool)`

GetJobIDOk returns a tuple with the JobID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetJobID

`func (o *JobSummary) SetJobID(v string)`

SetJobID sets JobID field to given value.

### HasJobID

`func (o *JobSummary) HasJobID() bool`

HasJobID returns a boolean if a field has been set.

### GetModifyIndex

`func (o *JobSummary) GetModifyIndex() int32`

GetModifyIndex returns the ModifyIndex field if non-nil, zero value otherwise.

### GetModifyIndexOk

`func (o *JobSummary) GetModifyIndexOk() (*int32, bool)`

GetModifyIndexOk returns a tuple with the ModifyIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetModifyIndex

`func (o *JobSummary) SetModifyIndex(v int32)`

SetModifyIndex sets ModifyIndex field to given value.

### HasModifyIndex

`func (o *JobSummary) HasModifyIndex() bool`

HasModifyIndex returns a boolean if a field has been set.

### GetNamespace

`func (o *JobSummary) GetNamespace() string`

GetNamespace returns the Namespace field if non-nil, zero value otherwise.

### GetNamespaceOk

`func (o *JobSummary) GetNamespaceOk() (*string, bool)`

GetNamespaceOk returns a tuple with the Namespace field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNamespace

`func (o *JobSummary) SetNamespace(v string)`

SetNamespace sets Namespace field to given value.

### HasNamespace

`func (o *JobSummary) HasNamespace() bool`

HasNamespace returns a boolean if a field has been set.

### GetSummary

`func (o *JobSummary) GetSummary() map[string]TaskGroupSummary`

GetSummary returns the Summary field if non-nil, zero value otherwise.

### GetSummaryOk

`func (o *JobSummary) GetSummaryOk() (*map[string]TaskGroupSummary, bool)`

GetSummaryOk returns a tuple with the Summary field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSummary

`func (o *JobSummary) SetSummary(v map[string]TaskGroupSummary)`

SetSummary sets Summary field to given value.

### HasSummary

`func (o *JobSummary) HasSummary() bool`

HasSummary returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


