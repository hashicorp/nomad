# Evaluation

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**AnnotatePlan** | Pointer to **bool** |  | [optional] 
**BlockedEval** | Pointer to **string** |  | [optional] 
**ClassEligibility** | Pointer to **map[string]bool** |  | [optional] 
**CreateIndex** | Pointer to **int32** |  | [optional] 
**CreateTime** | Pointer to **int64** |  | [optional] 
**DeploymentID** | Pointer to **string** |  | [optional] 
**EscapedComputedClass** | Pointer to **bool** |  | [optional] 
**FailedTGAllocs** | Pointer to [**map[string]AllocationMetric**](AllocationMetric.md) |  | [optional] 
**ID** | Pointer to **string** |  | [optional] 
**JobID** | Pointer to **string** |  | [optional] 
**JobModifyIndex** | Pointer to **int32** |  | [optional] 
**ModifyIndex** | Pointer to **int32** |  | [optional] 
**ModifyTime** | Pointer to **int64** |  | [optional] 
**Namespace** | Pointer to **string** |  | [optional] 
**NextEval** | Pointer to **string** |  | [optional] 
**NodeID** | Pointer to **string** |  | [optional] 
**NodeModifyIndex** | Pointer to **int32** |  | [optional] 
**PreviousEval** | Pointer to **string** |  | [optional] 
**Priority** | Pointer to **int64** |  | [optional] 
**QueuedAllocations** | Pointer to **map[string]int64** |  | [optional] 
**QuotaLimitReached** | Pointer to **string** |  | [optional] 
**SnapshotIndex** | Pointer to **int32** |  | [optional] 
**Status** | Pointer to **string** |  | [optional] 
**StatusDescription** | Pointer to **string** |  | [optional] 
**TriggeredBy** | Pointer to **string** |  | [optional] 
**Type** | Pointer to **string** |  | [optional] 
**Wait** | Pointer to **int64** | A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years. | [optional] 
**WaitUntil** | Pointer to **time.Time** |  | [optional] 

## Methods

### NewEvaluation

`func NewEvaluation() *Evaluation`

NewEvaluation instantiates a new Evaluation object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewEvaluationWithDefaults

`func NewEvaluationWithDefaults() *Evaluation`

NewEvaluationWithDefaults instantiates a new Evaluation object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetAnnotatePlan

`func (o *Evaluation) GetAnnotatePlan() bool`

GetAnnotatePlan returns the AnnotatePlan field if non-nil, zero value otherwise.

### GetAnnotatePlanOk

`func (o *Evaluation) GetAnnotatePlanOk() (*bool, bool)`

GetAnnotatePlanOk returns a tuple with the AnnotatePlan field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAnnotatePlan

`func (o *Evaluation) SetAnnotatePlan(v bool)`

SetAnnotatePlan sets AnnotatePlan field to given value.

### HasAnnotatePlan

`func (o *Evaluation) HasAnnotatePlan() bool`

HasAnnotatePlan returns a boolean if a field has been set.

### GetBlockedEval

`func (o *Evaluation) GetBlockedEval() string`

GetBlockedEval returns the BlockedEval field if non-nil, zero value otherwise.

### GetBlockedEvalOk

`func (o *Evaluation) GetBlockedEvalOk() (*string, bool)`

GetBlockedEvalOk returns a tuple with the BlockedEval field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetBlockedEval

`func (o *Evaluation) SetBlockedEval(v string)`

SetBlockedEval sets BlockedEval field to given value.

### HasBlockedEval

`func (o *Evaluation) HasBlockedEval() bool`

HasBlockedEval returns a boolean if a field has been set.

### GetClassEligibility

`func (o *Evaluation) GetClassEligibility() map[string]bool`

GetClassEligibility returns the ClassEligibility field if non-nil, zero value otherwise.

### GetClassEligibilityOk

`func (o *Evaluation) GetClassEligibilityOk() (*map[string]bool, bool)`

GetClassEligibilityOk returns a tuple with the ClassEligibility field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetClassEligibility

`func (o *Evaluation) SetClassEligibility(v map[string]bool)`

SetClassEligibility sets ClassEligibility field to given value.

### HasClassEligibility

`func (o *Evaluation) HasClassEligibility() bool`

HasClassEligibility returns a boolean if a field has been set.

### GetCreateIndex

`func (o *Evaluation) GetCreateIndex() int32`

GetCreateIndex returns the CreateIndex field if non-nil, zero value otherwise.

### GetCreateIndexOk

`func (o *Evaluation) GetCreateIndexOk() (*int32, bool)`

GetCreateIndexOk returns a tuple with the CreateIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCreateIndex

`func (o *Evaluation) SetCreateIndex(v int32)`

SetCreateIndex sets CreateIndex field to given value.

### HasCreateIndex

`func (o *Evaluation) HasCreateIndex() bool`

HasCreateIndex returns a boolean if a field has been set.

### GetCreateTime

`func (o *Evaluation) GetCreateTime() int64`

GetCreateTime returns the CreateTime field if non-nil, zero value otherwise.

### GetCreateTimeOk

`func (o *Evaluation) GetCreateTimeOk() (*int64, bool)`

GetCreateTimeOk returns a tuple with the CreateTime field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCreateTime

`func (o *Evaluation) SetCreateTime(v int64)`

SetCreateTime sets CreateTime field to given value.

### HasCreateTime

`func (o *Evaluation) HasCreateTime() bool`

HasCreateTime returns a boolean if a field has been set.

### GetDeploymentID

`func (o *Evaluation) GetDeploymentID() string`

GetDeploymentID returns the DeploymentID field if non-nil, zero value otherwise.

### GetDeploymentIDOk

`func (o *Evaluation) GetDeploymentIDOk() (*string, bool)`

GetDeploymentIDOk returns a tuple with the DeploymentID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDeploymentID

`func (o *Evaluation) SetDeploymentID(v string)`

SetDeploymentID sets DeploymentID field to given value.

### HasDeploymentID

`func (o *Evaluation) HasDeploymentID() bool`

HasDeploymentID returns a boolean if a field has been set.

### GetEscapedComputedClass

`func (o *Evaluation) GetEscapedComputedClass() bool`

GetEscapedComputedClass returns the EscapedComputedClass field if non-nil, zero value otherwise.

### GetEscapedComputedClassOk

`func (o *Evaluation) GetEscapedComputedClassOk() (*bool, bool)`

GetEscapedComputedClassOk returns a tuple with the EscapedComputedClass field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEscapedComputedClass

`func (o *Evaluation) SetEscapedComputedClass(v bool)`

SetEscapedComputedClass sets EscapedComputedClass field to given value.

### HasEscapedComputedClass

`func (o *Evaluation) HasEscapedComputedClass() bool`

HasEscapedComputedClass returns a boolean if a field has been set.

### GetFailedTGAllocs

`func (o *Evaluation) GetFailedTGAllocs() map[string]AllocationMetric`

GetFailedTGAllocs returns the FailedTGAllocs field if non-nil, zero value otherwise.

### GetFailedTGAllocsOk

`func (o *Evaluation) GetFailedTGAllocsOk() (*map[string]AllocationMetric, bool)`

GetFailedTGAllocsOk returns a tuple with the FailedTGAllocs field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetFailedTGAllocs

`func (o *Evaluation) SetFailedTGAllocs(v map[string]AllocationMetric)`

SetFailedTGAllocs sets FailedTGAllocs field to given value.

### HasFailedTGAllocs

`func (o *Evaluation) HasFailedTGAllocs() bool`

HasFailedTGAllocs returns a boolean if a field has been set.

### GetID

`func (o *Evaluation) GetID() string`

GetID returns the ID field if non-nil, zero value otherwise.

### GetIDOk

`func (o *Evaluation) GetIDOk() (*string, bool)`

GetIDOk returns a tuple with the ID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetID

`func (o *Evaluation) SetID(v string)`

SetID sets ID field to given value.

### HasID

`func (o *Evaluation) HasID() bool`

HasID returns a boolean if a field has been set.

### GetJobID

`func (o *Evaluation) GetJobID() string`

GetJobID returns the JobID field if non-nil, zero value otherwise.

### GetJobIDOk

`func (o *Evaluation) GetJobIDOk() (*string, bool)`

GetJobIDOk returns a tuple with the JobID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetJobID

`func (o *Evaluation) SetJobID(v string)`

SetJobID sets JobID field to given value.

### HasJobID

`func (o *Evaluation) HasJobID() bool`

HasJobID returns a boolean if a field has been set.

### GetJobModifyIndex

`func (o *Evaluation) GetJobModifyIndex() int32`

GetJobModifyIndex returns the JobModifyIndex field if non-nil, zero value otherwise.

### GetJobModifyIndexOk

`func (o *Evaluation) GetJobModifyIndexOk() (*int32, bool)`

GetJobModifyIndexOk returns a tuple with the JobModifyIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetJobModifyIndex

`func (o *Evaluation) SetJobModifyIndex(v int32)`

SetJobModifyIndex sets JobModifyIndex field to given value.

### HasJobModifyIndex

`func (o *Evaluation) HasJobModifyIndex() bool`

HasJobModifyIndex returns a boolean if a field has been set.

### GetModifyIndex

`func (o *Evaluation) GetModifyIndex() int32`

GetModifyIndex returns the ModifyIndex field if non-nil, zero value otherwise.

### GetModifyIndexOk

`func (o *Evaluation) GetModifyIndexOk() (*int32, bool)`

GetModifyIndexOk returns a tuple with the ModifyIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetModifyIndex

`func (o *Evaluation) SetModifyIndex(v int32)`

SetModifyIndex sets ModifyIndex field to given value.

### HasModifyIndex

`func (o *Evaluation) HasModifyIndex() bool`

HasModifyIndex returns a boolean if a field has been set.

### GetModifyTime

`func (o *Evaluation) GetModifyTime() int64`

GetModifyTime returns the ModifyTime field if non-nil, zero value otherwise.

### GetModifyTimeOk

`func (o *Evaluation) GetModifyTimeOk() (*int64, bool)`

GetModifyTimeOk returns a tuple with the ModifyTime field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetModifyTime

`func (o *Evaluation) SetModifyTime(v int64)`

SetModifyTime sets ModifyTime field to given value.

### HasModifyTime

`func (o *Evaluation) HasModifyTime() bool`

HasModifyTime returns a boolean if a field has been set.

### GetNamespace

`func (o *Evaluation) GetNamespace() string`

GetNamespace returns the Namespace field if non-nil, zero value otherwise.

### GetNamespaceOk

`func (o *Evaluation) GetNamespaceOk() (*string, bool)`

GetNamespaceOk returns a tuple with the Namespace field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNamespace

`func (o *Evaluation) SetNamespace(v string)`

SetNamespace sets Namespace field to given value.

### HasNamespace

`func (o *Evaluation) HasNamespace() bool`

HasNamespace returns a boolean if a field has been set.

### GetNextEval

`func (o *Evaluation) GetNextEval() string`

GetNextEval returns the NextEval field if non-nil, zero value otherwise.

### GetNextEvalOk

`func (o *Evaluation) GetNextEvalOk() (*string, bool)`

GetNextEvalOk returns a tuple with the NextEval field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNextEval

`func (o *Evaluation) SetNextEval(v string)`

SetNextEval sets NextEval field to given value.

### HasNextEval

`func (o *Evaluation) HasNextEval() bool`

HasNextEval returns a boolean if a field has been set.

### GetNodeID

`func (o *Evaluation) GetNodeID() string`

GetNodeID returns the NodeID field if non-nil, zero value otherwise.

### GetNodeIDOk

`func (o *Evaluation) GetNodeIDOk() (*string, bool)`

GetNodeIDOk returns a tuple with the NodeID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNodeID

`func (o *Evaluation) SetNodeID(v string)`

SetNodeID sets NodeID field to given value.

### HasNodeID

`func (o *Evaluation) HasNodeID() bool`

HasNodeID returns a boolean if a field has been set.

### GetNodeModifyIndex

`func (o *Evaluation) GetNodeModifyIndex() int32`

GetNodeModifyIndex returns the NodeModifyIndex field if non-nil, zero value otherwise.

### GetNodeModifyIndexOk

`func (o *Evaluation) GetNodeModifyIndexOk() (*int32, bool)`

GetNodeModifyIndexOk returns a tuple with the NodeModifyIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNodeModifyIndex

`func (o *Evaluation) SetNodeModifyIndex(v int32)`

SetNodeModifyIndex sets NodeModifyIndex field to given value.

### HasNodeModifyIndex

`func (o *Evaluation) HasNodeModifyIndex() bool`

HasNodeModifyIndex returns a boolean if a field has been set.

### GetPreviousEval

`func (o *Evaluation) GetPreviousEval() string`

GetPreviousEval returns the PreviousEval field if non-nil, zero value otherwise.

### GetPreviousEvalOk

`func (o *Evaluation) GetPreviousEvalOk() (*string, bool)`

GetPreviousEvalOk returns a tuple with the PreviousEval field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPreviousEval

`func (o *Evaluation) SetPreviousEval(v string)`

SetPreviousEval sets PreviousEval field to given value.

### HasPreviousEval

`func (o *Evaluation) HasPreviousEval() bool`

HasPreviousEval returns a boolean if a field has been set.

### GetPriority

`func (o *Evaluation) GetPriority() int64`

GetPriority returns the Priority field if non-nil, zero value otherwise.

### GetPriorityOk

`func (o *Evaluation) GetPriorityOk() (*int64, bool)`

GetPriorityOk returns a tuple with the Priority field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPriority

`func (o *Evaluation) SetPriority(v int64)`

SetPriority sets Priority field to given value.

### HasPriority

`func (o *Evaluation) HasPriority() bool`

HasPriority returns a boolean if a field has been set.

### GetQueuedAllocations

`func (o *Evaluation) GetQueuedAllocations() map[string]int64`

GetQueuedAllocations returns the QueuedAllocations field if non-nil, zero value otherwise.

### GetQueuedAllocationsOk

`func (o *Evaluation) GetQueuedAllocationsOk() (*map[string]int64, bool)`

GetQueuedAllocationsOk returns a tuple with the QueuedAllocations field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetQueuedAllocations

`func (o *Evaluation) SetQueuedAllocations(v map[string]int64)`

SetQueuedAllocations sets QueuedAllocations field to given value.

### HasQueuedAllocations

`func (o *Evaluation) HasQueuedAllocations() bool`

HasQueuedAllocations returns a boolean if a field has been set.

### GetQuotaLimitReached

`func (o *Evaluation) GetQuotaLimitReached() string`

GetQuotaLimitReached returns the QuotaLimitReached field if non-nil, zero value otherwise.

### GetQuotaLimitReachedOk

`func (o *Evaluation) GetQuotaLimitReachedOk() (*string, bool)`

GetQuotaLimitReachedOk returns a tuple with the QuotaLimitReached field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetQuotaLimitReached

`func (o *Evaluation) SetQuotaLimitReached(v string)`

SetQuotaLimitReached sets QuotaLimitReached field to given value.

### HasQuotaLimitReached

`func (o *Evaluation) HasQuotaLimitReached() bool`

HasQuotaLimitReached returns a boolean if a field has been set.

### GetSnapshotIndex

`func (o *Evaluation) GetSnapshotIndex() int32`

GetSnapshotIndex returns the SnapshotIndex field if non-nil, zero value otherwise.

### GetSnapshotIndexOk

`func (o *Evaluation) GetSnapshotIndexOk() (*int32, bool)`

GetSnapshotIndexOk returns a tuple with the SnapshotIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSnapshotIndex

`func (o *Evaluation) SetSnapshotIndex(v int32)`

SetSnapshotIndex sets SnapshotIndex field to given value.

### HasSnapshotIndex

`func (o *Evaluation) HasSnapshotIndex() bool`

HasSnapshotIndex returns a boolean if a field has been set.

### GetStatus

`func (o *Evaluation) GetStatus() string`

GetStatus returns the Status field if non-nil, zero value otherwise.

### GetStatusOk

`func (o *Evaluation) GetStatusOk() (*string, bool)`

GetStatusOk returns a tuple with the Status field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetStatus

`func (o *Evaluation) SetStatus(v string)`

SetStatus sets Status field to given value.

### HasStatus

`func (o *Evaluation) HasStatus() bool`

HasStatus returns a boolean if a field has been set.

### GetStatusDescription

`func (o *Evaluation) GetStatusDescription() string`

GetStatusDescription returns the StatusDescription field if non-nil, zero value otherwise.

### GetStatusDescriptionOk

`func (o *Evaluation) GetStatusDescriptionOk() (*string, bool)`

GetStatusDescriptionOk returns a tuple with the StatusDescription field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetStatusDescription

`func (o *Evaluation) SetStatusDescription(v string)`

SetStatusDescription sets StatusDescription field to given value.

### HasStatusDescription

`func (o *Evaluation) HasStatusDescription() bool`

HasStatusDescription returns a boolean if a field has been set.

### GetTriggeredBy

`func (o *Evaluation) GetTriggeredBy() string`

GetTriggeredBy returns the TriggeredBy field if non-nil, zero value otherwise.

### GetTriggeredByOk

`func (o *Evaluation) GetTriggeredByOk() (*string, bool)`

GetTriggeredByOk returns a tuple with the TriggeredBy field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTriggeredBy

`func (o *Evaluation) SetTriggeredBy(v string)`

SetTriggeredBy sets TriggeredBy field to given value.

### HasTriggeredBy

`func (o *Evaluation) HasTriggeredBy() bool`

HasTriggeredBy returns a boolean if a field has been set.

### GetType

`func (o *Evaluation) GetType() string`

GetType returns the Type field if non-nil, zero value otherwise.

### GetTypeOk

`func (o *Evaluation) GetTypeOk() (*string, bool)`

GetTypeOk returns a tuple with the Type field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetType

`func (o *Evaluation) SetType(v string)`

SetType sets Type field to given value.

### HasType

`func (o *Evaluation) HasType() bool`

HasType returns a boolean if a field has been set.

### GetWait

`func (o *Evaluation) GetWait() int64`

GetWait returns the Wait field if non-nil, zero value otherwise.

### GetWaitOk

`func (o *Evaluation) GetWaitOk() (*int64, bool)`

GetWaitOk returns a tuple with the Wait field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetWait

`func (o *Evaluation) SetWait(v int64)`

SetWait sets Wait field to given value.

### HasWait

`func (o *Evaluation) HasWait() bool`

HasWait returns a boolean if a field has been set.

### GetWaitUntil

`func (o *Evaluation) GetWaitUntil() time.Time`

GetWaitUntil returns the WaitUntil field if non-nil, zero value otherwise.

### GetWaitUntilOk

`func (o *Evaluation) GetWaitUntilOk() (*time.Time, bool)`

GetWaitUntilOk returns a tuple with the WaitUntil field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetWaitUntil

`func (o *Evaluation) SetWaitUntil(v time.Time)`

SetWaitUntil sets WaitUntil field to given value.

### HasWaitUntil

`func (o *Evaluation) HasWaitUntil() bool`

HasWaitUntil returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


