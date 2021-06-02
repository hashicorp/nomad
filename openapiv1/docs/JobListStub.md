# JobListStub

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**CreateIndex** | Pointer to **int32** |  | [optional] 
**Datacenters** | Pointer to **[]string** |  | [optional] 
**ID** | Pointer to **string** |  | [optional] 
**JobModifyIndex** | Pointer to **int32** |  | [optional] 
**JobSummary** | Pointer to [**JobSummary**](JobSummary.md) |  | [optional] 
**ModifyIndex** | Pointer to **int32** |  | [optional] 
**Name** | Pointer to **string** |  | [optional] 
**Namespace** | Pointer to **string** |  | [optional] 
**ParameterizedJob** | Pointer to **bool** |  | [optional] 
**ParentID** | Pointer to **string** |  | [optional] 
**Periodic** | Pointer to **bool** |  | [optional] 
**Priority** | Pointer to **int64** |  | [optional] 
**Status** | Pointer to **string** |  | [optional] 
**StatusDescription** | Pointer to **string** |  | [optional] 
**Stop** | Pointer to **bool** |  | [optional] 
**SubmitTime** | Pointer to **int64** |  | [optional] 
**Type** | Pointer to **string** |  | [optional] 

## Methods

### NewJobListStub

`func NewJobListStub() *JobListStub`

NewJobListStub instantiates a new JobListStub object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewJobListStubWithDefaults

`func NewJobListStubWithDefaults() *JobListStub`

NewJobListStubWithDefaults instantiates a new JobListStub object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetCreateIndex

`func (o *JobListStub) GetCreateIndex() int32`

GetCreateIndex returns the CreateIndex field if non-nil, zero value otherwise.

### GetCreateIndexOk

`func (o *JobListStub) GetCreateIndexOk() (*int32, bool)`

GetCreateIndexOk returns a tuple with the CreateIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCreateIndex

`func (o *JobListStub) SetCreateIndex(v int32)`

SetCreateIndex sets CreateIndex field to given value.

### HasCreateIndex

`func (o *JobListStub) HasCreateIndex() bool`

HasCreateIndex returns a boolean if a field has been set.

### GetDatacenters

`func (o *JobListStub) GetDatacenters() []string`

GetDatacenters returns the Datacenters field if non-nil, zero value otherwise.

### GetDatacentersOk

`func (o *JobListStub) GetDatacentersOk() (*[]string, bool)`

GetDatacentersOk returns a tuple with the Datacenters field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDatacenters

`func (o *JobListStub) SetDatacenters(v []string)`

SetDatacenters sets Datacenters field to given value.

### HasDatacenters

`func (o *JobListStub) HasDatacenters() bool`

HasDatacenters returns a boolean if a field has been set.

### GetID

`func (o *JobListStub) GetID() string`

GetID returns the ID field if non-nil, zero value otherwise.

### GetIDOk

`func (o *JobListStub) GetIDOk() (*string, bool)`

GetIDOk returns a tuple with the ID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetID

`func (o *JobListStub) SetID(v string)`

SetID sets ID field to given value.

### HasID

`func (o *JobListStub) HasID() bool`

HasID returns a boolean if a field has been set.

### GetJobModifyIndex

`func (o *JobListStub) GetJobModifyIndex() int32`

GetJobModifyIndex returns the JobModifyIndex field if non-nil, zero value otherwise.

### GetJobModifyIndexOk

`func (o *JobListStub) GetJobModifyIndexOk() (*int32, bool)`

GetJobModifyIndexOk returns a tuple with the JobModifyIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetJobModifyIndex

`func (o *JobListStub) SetJobModifyIndex(v int32)`

SetJobModifyIndex sets JobModifyIndex field to given value.

### HasJobModifyIndex

`func (o *JobListStub) HasJobModifyIndex() bool`

HasJobModifyIndex returns a boolean if a field has been set.

### GetJobSummary

`func (o *JobListStub) GetJobSummary() JobSummary`

GetJobSummary returns the JobSummary field if non-nil, zero value otherwise.

### GetJobSummaryOk

`func (o *JobListStub) GetJobSummaryOk() (*JobSummary, bool)`

GetJobSummaryOk returns a tuple with the JobSummary field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetJobSummary

`func (o *JobListStub) SetJobSummary(v JobSummary)`

SetJobSummary sets JobSummary field to given value.

### HasJobSummary

`func (o *JobListStub) HasJobSummary() bool`

HasJobSummary returns a boolean if a field has been set.

### GetModifyIndex

`func (o *JobListStub) GetModifyIndex() int32`

GetModifyIndex returns the ModifyIndex field if non-nil, zero value otherwise.

### GetModifyIndexOk

`func (o *JobListStub) GetModifyIndexOk() (*int32, bool)`

GetModifyIndexOk returns a tuple with the ModifyIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetModifyIndex

`func (o *JobListStub) SetModifyIndex(v int32)`

SetModifyIndex sets ModifyIndex field to given value.

### HasModifyIndex

`func (o *JobListStub) HasModifyIndex() bool`

HasModifyIndex returns a boolean if a field has been set.

### GetName

`func (o *JobListStub) GetName() string`

GetName returns the Name field if non-nil, zero value otherwise.

### GetNameOk

`func (o *JobListStub) GetNameOk() (*string, bool)`

GetNameOk returns a tuple with the Name field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetName

`func (o *JobListStub) SetName(v string)`

SetName sets Name field to given value.

### HasName

`func (o *JobListStub) HasName() bool`

HasName returns a boolean if a field has been set.

### GetNamespace

`func (o *JobListStub) GetNamespace() string`

GetNamespace returns the Namespace field if non-nil, zero value otherwise.

### GetNamespaceOk

`func (o *JobListStub) GetNamespaceOk() (*string, bool)`

GetNamespaceOk returns a tuple with the Namespace field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNamespace

`func (o *JobListStub) SetNamespace(v string)`

SetNamespace sets Namespace field to given value.

### HasNamespace

`func (o *JobListStub) HasNamespace() bool`

HasNamespace returns a boolean if a field has been set.

### GetParameterizedJob

`func (o *JobListStub) GetParameterizedJob() bool`

GetParameterizedJob returns the ParameterizedJob field if non-nil, zero value otherwise.

### GetParameterizedJobOk

`func (o *JobListStub) GetParameterizedJobOk() (*bool, bool)`

GetParameterizedJobOk returns a tuple with the ParameterizedJob field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetParameterizedJob

`func (o *JobListStub) SetParameterizedJob(v bool)`

SetParameterizedJob sets ParameterizedJob field to given value.

### HasParameterizedJob

`func (o *JobListStub) HasParameterizedJob() bool`

HasParameterizedJob returns a boolean if a field has been set.

### GetParentID

`func (o *JobListStub) GetParentID() string`

GetParentID returns the ParentID field if non-nil, zero value otherwise.

### GetParentIDOk

`func (o *JobListStub) GetParentIDOk() (*string, bool)`

GetParentIDOk returns a tuple with the ParentID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetParentID

`func (o *JobListStub) SetParentID(v string)`

SetParentID sets ParentID field to given value.

### HasParentID

`func (o *JobListStub) HasParentID() bool`

HasParentID returns a boolean if a field has been set.

### GetPeriodic

`func (o *JobListStub) GetPeriodic() bool`

GetPeriodic returns the Periodic field if non-nil, zero value otherwise.

### GetPeriodicOk

`func (o *JobListStub) GetPeriodicOk() (*bool, bool)`

GetPeriodicOk returns a tuple with the Periodic field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPeriodic

`func (o *JobListStub) SetPeriodic(v bool)`

SetPeriodic sets Periodic field to given value.

### HasPeriodic

`func (o *JobListStub) HasPeriodic() bool`

HasPeriodic returns a boolean if a field has been set.

### GetPriority

`func (o *JobListStub) GetPriority() int64`

GetPriority returns the Priority field if non-nil, zero value otherwise.

### GetPriorityOk

`func (o *JobListStub) GetPriorityOk() (*int64, bool)`

GetPriorityOk returns a tuple with the Priority field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPriority

`func (o *JobListStub) SetPriority(v int64)`

SetPriority sets Priority field to given value.

### HasPriority

`func (o *JobListStub) HasPriority() bool`

HasPriority returns a boolean if a field has been set.

### GetStatus

`func (o *JobListStub) GetStatus() string`

GetStatus returns the Status field if non-nil, zero value otherwise.

### GetStatusOk

`func (o *JobListStub) GetStatusOk() (*string, bool)`

GetStatusOk returns a tuple with the Status field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetStatus

`func (o *JobListStub) SetStatus(v string)`

SetStatus sets Status field to given value.

### HasStatus

`func (o *JobListStub) HasStatus() bool`

HasStatus returns a boolean if a field has been set.

### GetStatusDescription

`func (o *JobListStub) GetStatusDescription() string`

GetStatusDescription returns the StatusDescription field if non-nil, zero value otherwise.

### GetStatusDescriptionOk

`func (o *JobListStub) GetStatusDescriptionOk() (*string, bool)`

GetStatusDescriptionOk returns a tuple with the StatusDescription field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetStatusDescription

`func (o *JobListStub) SetStatusDescription(v string)`

SetStatusDescription sets StatusDescription field to given value.

### HasStatusDescription

`func (o *JobListStub) HasStatusDescription() bool`

HasStatusDescription returns a boolean if a field has been set.

### GetStop

`func (o *JobListStub) GetStop() bool`

GetStop returns the Stop field if non-nil, zero value otherwise.

### GetStopOk

`func (o *JobListStub) GetStopOk() (*bool, bool)`

GetStopOk returns a tuple with the Stop field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetStop

`func (o *JobListStub) SetStop(v bool)`

SetStop sets Stop field to given value.

### HasStop

`func (o *JobListStub) HasStop() bool`

HasStop returns a boolean if a field has been set.

### GetSubmitTime

`func (o *JobListStub) GetSubmitTime() int64`

GetSubmitTime returns the SubmitTime field if non-nil, zero value otherwise.

### GetSubmitTimeOk

`func (o *JobListStub) GetSubmitTimeOk() (*int64, bool)`

GetSubmitTimeOk returns a tuple with the SubmitTime field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSubmitTime

`func (o *JobListStub) SetSubmitTime(v int64)`

SetSubmitTime sets SubmitTime field to given value.

### HasSubmitTime

`func (o *JobListStub) HasSubmitTime() bool`

HasSubmitTime returns a boolean if a field has been set.

### GetType

`func (o *JobListStub) GetType() string`

GetType returns the Type field if non-nil, zero value otherwise.

### GetTypeOk

`func (o *JobListStub) GetTypeOk() (*string, bool)`

GetTypeOk returns a tuple with the Type field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetType

`func (o *JobListStub) SetType(v string)`

SetType sets Type field to given value.

### HasType

`func (o *JobListStub) HasType() bool`

HasType returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


