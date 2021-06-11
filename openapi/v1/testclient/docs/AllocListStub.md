# AllocListStub

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**AllocatedResources** | Pointer to [**AllocatedResources**](AllocatedResources.md) |  | [optional] 
**ClientDescription** | Pointer to **string** |  | [optional] 
**ClientStatus** | Pointer to **string** |  | [optional] 
**CreateIndex** | Pointer to **int32** |  | [optional] 
**CreateTime** | Pointer to **int64** |  | [optional] 
**DeploymentStatus** | Pointer to [**AllocDeploymentStatus**](AllocDeploymentStatus.md) |  | [optional] 
**DesiredDescription** | Pointer to **string** |  | [optional] 
**DesiredStatus** | Pointer to **string** |  | [optional] 
**DesiredTransition** | Pointer to [**DesiredTransition**](DesiredTransition.md) |  | [optional] 
**EvalID** | Pointer to **string** |  | [optional] 
**FollowupEvalID** | Pointer to **string** |  | [optional] 
**ID** | Pointer to **string** |  | [optional] 
**JobID** | Pointer to **string** |  | [optional] 
**JobType** | Pointer to **string** |  | [optional] 
**JobVersion** | Pointer to **int32** |  | [optional] 
**ModifyIndex** | Pointer to **int32** |  | [optional] 
**ModifyTime** | Pointer to **int64** |  | [optional] 
**Name** | Pointer to **string** |  | [optional] 
**Namespace** | Pointer to **string** |  | [optional] 
**NodeID** | Pointer to **string** |  | [optional] 
**NodeName** | Pointer to **string** |  | [optional] 
**PreemptedAllocations** | Pointer to **[]string** |  | [optional] 
**PreemptedByAllocation** | Pointer to **string** |  | [optional] 
**RescheduleTracker** | Pointer to [**RescheduleTracker**](RescheduleTracker.md) |  | [optional] 
**TaskGroup** | Pointer to **string** |  | [optional] 
**TaskStates** | Pointer to [**map[string]TaskState**](TaskState.md) |  | [optional] 

## Methods

### NewAllocListStub

`func NewAllocListStub() *AllocListStub`

NewAllocListStub instantiates a new AllocListStub object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewAllocListStubWithDefaults

`func NewAllocListStubWithDefaults() *AllocListStub`

NewAllocListStubWithDefaults instantiates a new AllocListStub object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetAllocatedResources

`func (o *AllocListStub) GetAllocatedResources() AllocatedResources`

GetAllocatedResources returns the AllocatedResources field if non-nil, zero value otherwise.

### GetAllocatedResourcesOk

`func (o *AllocListStub) GetAllocatedResourcesOk() (*AllocatedResources, bool)`

GetAllocatedResourcesOk returns a tuple with the AllocatedResources field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAllocatedResources

`func (o *AllocListStub) SetAllocatedResources(v AllocatedResources)`

SetAllocatedResources sets AllocatedResources field to given value.

### HasAllocatedResources

`func (o *AllocListStub) HasAllocatedResources() bool`

HasAllocatedResources returns a boolean if a field has been set.

### GetClientDescription

`func (o *AllocListStub) GetClientDescription() string`

GetClientDescription returns the ClientDescription field if non-nil, zero value otherwise.

### GetClientDescriptionOk

`func (o *AllocListStub) GetClientDescriptionOk() (*string, bool)`

GetClientDescriptionOk returns a tuple with the ClientDescription field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetClientDescription

`func (o *AllocListStub) SetClientDescription(v string)`

SetClientDescription sets ClientDescription field to given value.

### HasClientDescription

`func (o *AllocListStub) HasClientDescription() bool`

HasClientDescription returns a boolean if a field has been set.

### GetClientStatus

`func (o *AllocListStub) GetClientStatus() string`

GetClientStatus returns the ClientStatus field if non-nil, zero value otherwise.

### GetClientStatusOk

`func (o *AllocListStub) GetClientStatusOk() (*string, bool)`

GetClientStatusOk returns a tuple with the ClientStatus field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetClientStatus

`func (o *AllocListStub) SetClientStatus(v string)`

SetClientStatus sets ClientStatus field to given value.

### HasClientStatus

`func (o *AllocListStub) HasClientStatus() bool`

HasClientStatus returns a boolean if a field has been set.

### GetCreateIndex

`func (o *AllocListStub) GetCreateIndex() int32`

GetCreateIndex returns the CreateIndex field if non-nil, zero value otherwise.

### GetCreateIndexOk

`func (o *AllocListStub) GetCreateIndexOk() (*int32, bool)`

GetCreateIndexOk returns a tuple with the CreateIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCreateIndex

`func (o *AllocListStub) SetCreateIndex(v int32)`

SetCreateIndex sets CreateIndex field to given value.

### HasCreateIndex

`func (o *AllocListStub) HasCreateIndex() bool`

HasCreateIndex returns a boolean if a field has been set.

### GetCreateTime

`func (o *AllocListStub) GetCreateTime() int64`

GetCreateTime returns the CreateTime field if non-nil, zero value otherwise.

### GetCreateTimeOk

`func (o *AllocListStub) GetCreateTimeOk() (*int64, bool)`

GetCreateTimeOk returns a tuple with the CreateTime field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCreateTime

`func (o *AllocListStub) SetCreateTime(v int64)`

SetCreateTime sets CreateTime field to given value.

### HasCreateTime

`func (o *AllocListStub) HasCreateTime() bool`

HasCreateTime returns a boolean if a field has been set.

### GetDeploymentStatus

`func (o *AllocListStub) GetDeploymentStatus() AllocDeploymentStatus`

GetDeploymentStatus returns the DeploymentStatus field if non-nil, zero value otherwise.

### GetDeploymentStatusOk

`func (o *AllocListStub) GetDeploymentStatusOk() (*AllocDeploymentStatus, bool)`

GetDeploymentStatusOk returns a tuple with the DeploymentStatus field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDeploymentStatus

`func (o *AllocListStub) SetDeploymentStatus(v AllocDeploymentStatus)`

SetDeploymentStatus sets DeploymentStatus field to given value.

### HasDeploymentStatus

`func (o *AllocListStub) HasDeploymentStatus() bool`

HasDeploymentStatus returns a boolean if a field has been set.

### GetDesiredDescription

`func (o *AllocListStub) GetDesiredDescription() string`

GetDesiredDescription returns the DesiredDescription field if non-nil, zero value otherwise.

### GetDesiredDescriptionOk

`func (o *AllocListStub) GetDesiredDescriptionOk() (*string, bool)`

GetDesiredDescriptionOk returns a tuple with the DesiredDescription field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDesiredDescription

`func (o *AllocListStub) SetDesiredDescription(v string)`

SetDesiredDescription sets DesiredDescription field to given value.

### HasDesiredDescription

`func (o *AllocListStub) HasDesiredDescription() bool`

HasDesiredDescription returns a boolean if a field has been set.

### GetDesiredStatus

`func (o *AllocListStub) GetDesiredStatus() string`

GetDesiredStatus returns the DesiredStatus field if non-nil, zero value otherwise.

### GetDesiredStatusOk

`func (o *AllocListStub) GetDesiredStatusOk() (*string, bool)`

GetDesiredStatusOk returns a tuple with the DesiredStatus field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDesiredStatus

`func (o *AllocListStub) SetDesiredStatus(v string)`

SetDesiredStatus sets DesiredStatus field to given value.

### HasDesiredStatus

`func (o *AllocListStub) HasDesiredStatus() bool`

HasDesiredStatus returns a boolean if a field has been set.

### GetDesiredTransition

`func (o *AllocListStub) GetDesiredTransition() DesiredTransition`

GetDesiredTransition returns the DesiredTransition field if non-nil, zero value otherwise.

### GetDesiredTransitionOk

`func (o *AllocListStub) GetDesiredTransitionOk() (*DesiredTransition, bool)`

GetDesiredTransitionOk returns a tuple with the DesiredTransition field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDesiredTransition

`func (o *AllocListStub) SetDesiredTransition(v DesiredTransition)`

SetDesiredTransition sets DesiredTransition field to given value.

### HasDesiredTransition

`func (o *AllocListStub) HasDesiredTransition() bool`

HasDesiredTransition returns a boolean if a field has been set.

### GetEvalID

`func (o *AllocListStub) GetEvalID() string`

GetEvalID returns the EvalID field if non-nil, zero value otherwise.

### GetEvalIDOk

`func (o *AllocListStub) GetEvalIDOk() (*string, bool)`

GetEvalIDOk returns a tuple with the EvalID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEvalID

`func (o *AllocListStub) SetEvalID(v string)`

SetEvalID sets EvalID field to given value.

### HasEvalID

`func (o *AllocListStub) HasEvalID() bool`

HasEvalID returns a boolean if a field has been set.

### GetFollowupEvalID

`func (o *AllocListStub) GetFollowupEvalID() string`

GetFollowupEvalID returns the FollowupEvalID field if non-nil, zero value otherwise.

### GetFollowupEvalIDOk

`func (o *AllocListStub) GetFollowupEvalIDOk() (*string, bool)`

GetFollowupEvalIDOk returns a tuple with the FollowupEvalID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetFollowupEvalID

`func (o *AllocListStub) SetFollowupEvalID(v string)`

SetFollowupEvalID sets FollowupEvalID field to given value.

### HasFollowupEvalID

`func (o *AllocListStub) HasFollowupEvalID() bool`

HasFollowupEvalID returns a boolean if a field has been set.

### GetID

`func (o *AllocListStub) GetID() string`

GetID returns the ID field if non-nil, zero value otherwise.

### GetIDOk

`func (o *AllocListStub) GetIDOk() (*string, bool)`

GetIDOk returns a tuple with the ID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetID

`func (o *AllocListStub) SetID(v string)`

SetID sets ID field to given value.

### HasID

`func (o *AllocListStub) HasID() bool`

HasID returns a boolean if a field has been set.

### GetJobID

`func (o *AllocListStub) GetJobID() string`

GetJobID returns the JobID field if non-nil, zero value otherwise.

### GetJobIDOk

`func (o *AllocListStub) GetJobIDOk() (*string, bool)`

GetJobIDOk returns a tuple with the JobID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetJobID

`func (o *AllocListStub) SetJobID(v string)`

SetJobID sets JobID field to given value.

### HasJobID

`func (o *AllocListStub) HasJobID() bool`

HasJobID returns a boolean if a field has been set.

### GetJobType

`func (o *AllocListStub) GetJobType() string`

GetJobType returns the JobType field if non-nil, zero value otherwise.

### GetJobTypeOk

`func (o *AllocListStub) GetJobTypeOk() (*string, bool)`

GetJobTypeOk returns a tuple with the JobType field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetJobType

`func (o *AllocListStub) SetJobType(v string)`

SetJobType sets JobType field to given value.

### HasJobType

`func (o *AllocListStub) HasJobType() bool`

HasJobType returns a boolean if a field has been set.

### GetJobVersion

`func (o *AllocListStub) GetJobVersion() int32`

GetJobVersion returns the JobVersion field if non-nil, zero value otherwise.

### GetJobVersionOk

`func (o *AllocListStub) GetJobVersionOk() (*int32, bool)`

GetJobVersionOk returns a tuple with the JobVersion field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetJobVersion

`func (o *AllocListStub) SetJobVersion(v int32)`

SetJobVersion sets JobVersion field to given value.

### HasJobVersion

`func (o *AllocListStub) HasJobVersion() bool`

HasJobVersion returns a boolean if a field has been set.

### GetModifyIndex

`func (o *AllocListStub) GetModifyIndex() int32`

GetModifyIndex returns the ModifyIndex field if non-nil, zero value otherwise.

### GetModifyIndexOk

`func (o *AllocListStub) GetModifyIndexOk() (*int32, bool)`

GetModifyIndexOk returns a tuple with the ModifyIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetModifyIndex

`func (o *AllocListStub) SetModifyIndex(v int32)`

SetModifyIndex sets ModifyIndex field to given value.

### HasModifyIndex

`func (o *AllocListStub) HasModifyIndex() bool`

HasModifyIndex returns a boolean if a field has been set.

### GetModifyTime

`func (o *AllocListStub) GetModifyTime() int64`

GetModifyTime returns the ModifyTime field if non-nil, zero value otherwise.

### GetModifyTimeOk

`func (o *AllocListStub) GetModifyTimeOk() (*int64, bool)`

GetModifyTimeOk returns a tuple with the ModifyTime field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetModifyTime

`func (o *AllocListStub) SetModifyTime(v int64)`

SetModifyTime sets ModifyTime field to given value.

### HasModifyTime

`func (o *AllocListStub) HasModifyTime() bool`

HasModifyTime returns a boolean if a field has been set.

### GetName

`func (o *AllocListStub) GetName() string`

GetName returns the Name field if non-nil, zero value otherwise.

### GetNameOk

`func (o *AllocListStub) GetNameOk() (*string, bool)`

GetNameOk returns a tuple with the Name field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetName

`func (o *AllocListStub) SetName(v string)`

SetName sets Name field to given value.

### HasName

`func (o *AllocListStub) HasName() bool`

HasName returns a boolean if a field has been set.

### GetNamespace

`func (o *AllocListStub) GetNamespace() string`

GetNamespace returns the Namespace field if non-nil, zero value otherwise.

### GetNamespaceOk

`func (o *AllocListStub) GetNamespaceOk() (*string, bool)`

GetNamespaceOk returns a tuple with the Namespace field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNamespace

`func (o *AllocListStub) SetNamespace(v string)`

SetNamespace sets Namespace field to given value.

### HasNamespace

`func (o *AllocListStub) HasNamespace() bool`

HasNamespace returns a boolean if a field has been set.

### GetNodeID

`func (o *AllocListStub) GetNodeID() string`

GetNodeID returns the NodeID field if non-nil, zero value otherwise.

### GetNodeIDOk

`func (o *AllocListStub) GetNodeIDOk() (*string, bool)`

GetNodeIDOk returns a tuple with the NodeID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNodeID

`func (o *AllocListStub) SetNodeID(v string)`

SetNodeID sets NodeID field to given value.

### HasNodeID

`func (o *AllocListStub) HasNodeID() bool`

HasNodeID returns a boolean if a field has been set.

### GetNodeName

`func (o *AllocListStub) GetNodeName() string`

GetNodeName returns the NodeName field if non-nil, zero value otherwise.

### GetNodeNameOk

`func (o *AllocListStub) GetNodeNameOk() (*string, bool)`

GetNodeNameOk returns a tuple with the NodeName field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNodeName

`func (o *AllocListStub) SetNodeName(v string)`

SetNodeName sets NodeName field to given value.

### HasNodeName

`func (o *AllocListStub) HasNodeName() bool`

HasNodeName returns a boolean if a field has been set.

### GetPreemptedAllocations

`func (o *AllocListStub) GetPreemptedAllocations() []string`

GetPreemptedAllocations returns the PreemptedAllocations field if non-nil, zero value otherwise.

### GetPreemptedAllocationsOk

`func (o *AllocListStub) GetPreemptedAllocationsOk() (*[]string, bool)`

GetPreemptedAllocationsOk returns a tuple with the PreemptedAllocations field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPreemptedAllocations

`func (o *AllocListStub) SetPreemptedAllocations(v []string)`

SetPreemptedAllocations sets PreemptedAllocations field to given value.

### HasPreemptedAllocations

`func (o *AllocListStub) HasPreemptedAllocations() bool`

HasPreemptedAllocations returns a boolean if a field has been set.

### GetPreemptedByAllocation

`func (o *AllocListStub) GetPreemptedByAllocation() string`

GetPreemptedByAllocation returns the PreemptedByAllocation field if non-nil, zero value otherwise.

### GetPreemptedByAllocationOk

`func (o *AllocListStub) GetPreemptedByAllocationOk() (*string, bool)`

GetPreemptedByAllocationOk returns a tuple with the PreemptedByAllocation field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPreemptedByAllocation

`func (o *AllocListStub) SetPreemptedByAllocation(v string)`

SetPreemptedByAllocation sets PreemptedByAllocation field to given value.

### HasPreemptedByAllocation

`func (o *AllocListStub) HasPreemptedByAllocation() bool`

HasPreemptedByAllocation returns a boolean if a field has been set.

### GetRescheduleTracker

`func (o *AllocListStub) GetRescheduleTracker() RescheduleTracker`

GetRescheduleTracker returns the RescheduleTracker field if non-nil, zero value otherwise.

### GetRescheduleTrackerOk

`func (o *AllocListStub) GetRescheduleTrackerOk() (*RescheduleTracker, bool)`

GetRescheduleTrackerOk returns a tuple with the RescheduleTracker field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetRescheduleTracker

`func (o *AllocListStub) SetRescheduleTracker(v RescheduleTracker)`

SetRescheduleTracker sets RescheduleTracker field to given value.

### HasRescheduleTracker

`func (o *AllocListStub) HasRescheduleTracker() bool`

HasRescheduleTracker returns a boolean if a field has been set.

### GetTaskGroup

`func (o *AllocListStub) GetTaskGroup() string`

GetTaskGroup returns the TaskGroup field if non-nil, zero value otherwise.

### GetTaskGroupOk

`func (o *AllocListStub) GetTaskGroupOk() (*string, bool)`

GetTaskGroupOk returns a tuple with the TaskGroup field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTaskGroup

`func (o *AllocListStub) SetTaskGroup(v string)`

SetTaskGroup sets TaskGroup field to given value.

### HasTaskGroup

`func (o *AllocListStub) HasTaskGroup() bool`

HasTaskGroup returns a boolean if a field has been set.

### GetTaskStates

`func (o *AllocListStub) GetTaskStates() map[string]TaskState`

GetTaskStates returns the TaskStates field if non-nil, zero value otherwise.

### GetTaskStatesOk

`func (o *AllocListStub) GetTaskStatesOk() (*map[string]TaskState, bool)`

GetTaskStatesOk returns a tuple with the TaskStates field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTaskStates

`func (o *AllocListStub) SetTaskStates(v map[string]TaskState)`

SetTaskStates sets TaskStates field to given value.

### HasTaskStates

`func (o *AllocListStub) HasTaskStates() bool`

HasTaskStates returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


