# JobPlanResponse

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Annotations** | Pointer to [**PlanAnnotations**](PlanAnnotations.md) |  | [optional] 
**CreatedEvals** | Pointer to [**[]Evaluation**](Evaluation.md) |  | [optional] 
**Diff** | Pointer to [**JobDiff**](JobDiff.md) |  | [optional] 
**FailedTGAllocs** | Pointer to [**map[string]AllocationMetric**](AllocationMetric.md) |  | [optional] 
**JobModifyIndex** | Pointer to **int32** |  | [optional] 
**NextPeriodicLaunch** | Pointer to **time.Time** |  | [optional] 
**Warnings** | Pointer to **string** |  | [optional] 

## Methods

### NewJobPlanResponse

`func NewJobPlanResponse() *JobPlanResponse`

NewJobPlanResponse instantiates a new JobPlanResponse object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewJobPlanResponseWithDefaults

`func NewJobPlanResponseWithDefaults() *JobPlanResponse`

NewJobPlanResponseWithDefaults instantiates a new JobPlanResponse object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetAnnotations

`func (o *JobPlanResponse) GetAnnotations() PlanAnnotations`

GetAnnotations returns the Annotations field if non-nil, zero value otherwise.

### GetAnnotationsOk

`func (o *JobPlanResponse) GetAnnotationsOk() (*PlanAnnotations, bool)`

GetAnnotationsOk returns a tuple with the Annotations field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAnnotations

`func (o *JobPlanResponse) SetAnnotations(v PlanAnnotations)`

SetAnnotations sets Annotations field to given value.

### HasAnnotations

`func (o *JobPlanResponse) HasAnnotations() bool`

HasAnnotations returns a boolean if a field has been set.

### GetCreatedEvals

`func (o *JobPlanResponse) GetCreatedEvals() []Evaluation`

GetCreatedEvals returns the CreatedEvals field if non-nil, zero value otherwise.

### GetCreatedEvalsOk

`func (o *JobPlanResponse) GetCreatedEvalsOk() (*[]Evaluation, bool)`

GetCreatedEvalsOk returns a tuple with the CreatedEvals field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCreatedEvals

`func (o *JobPlanResponse) SetCreatedEvals(v []Evaluation)`

SetCreatedEvals sets CreatedEvals field to given value.

### HasCreatedEvals

`func (o *JobPlanResponse) HasCreatedEvals() bool`

HasCreatedEvals returns a boolean if a field has been set.

### GetDiff

`func (o *JobPlanResponse) GetDiff() JobDiff`

GetDiff returns the Diff field if non-nil, zero value otherwise.

### GetDiffOk

`func (o *JobPlanResponse) GetDiffOk() (*JobDiff, bool)`

GetDiffOk returns a tuple with the Diff field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDiff

`func (o *JobPlanResponse) SetDiff(v JobDiff)`

SetDiff sets Diff field to given value.

### HasDiff

`func (o *JobPlanResponse) HasDiff() bool`

HasDiff returns a boolean if a field has been set.

### GetFailedTGAllocs

`func (o *JobPlanResponse) GetFailedTGAllocs() map[string]AllocationMetric`

GetFailedTGAllocs returns the FailedTGAllocs field if non-nil, zero value otherwise.

### GetFailedTGAllocsOk

`func (o *JobPlanResponse) GetFailedTGAllocsOk() (*map[string]AllocationMetric, bool)`

GetFailedTGAllocsOk returns a tuple with the FailedTGAllocs field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetFailedTGAllocs

`func (o *JobPlanResponse) SetFailedTGAllocs(v map[string]AllocationMetric)`

SetFailedTGAllocs sets FailedTGAllocs field to given value.

### HasFailedTGAllocs

`func (o *JobPlanResponse) HasFailedTGAllocs() bool`

HasFailedTGAllocs returns a boolean if a field has been set.

### GetJobModifyIndex

`func (o *JobPlanResponse) GetJobModifyIndex() int32`

GetJobModifyIndex returns the JobModifyIndex field if non-nil, zero value otherwise.

### GetJobModifyIndexOk

`func (o *JobPlanResponse) GetJobModifyIndexOk() (*int32, bool)`

GetJobModifyIndexOk returns a tuple with the JobModifyIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetJobModifyIndex

`func (o *JobPlanResponse) SetJobModifyIndex(v int32)`

SetJobModifyIndex sets JobModifyIndex field to given value.

### HasJobModifyIndex

`func (o *JobPlanResponse) HasJobModifyIndex() bool`

HasJobModifyIndex returns a boolean if a field has been set.

### GetNextPeriodicLaunch

`func (o *JobPlanResponse) GetNextPeriodicLaunch() time.Time`

GetNextPeriodicLaunch returns the NextPeriodicLaunch field if non-nil, zero value otherwise.

### GetNextPeriodicLaunchOk

`func (o *JobPlanResponse) GetNextPeriodicLaunchOk() (*time.Time, bool)`

GetNextPeriodicLaunchOk returns a tuple with the NextPeriodicLaunch field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNextPeriodicLaunch

`func (o *JobPlanResponse) SetNextPeriodicLaunch(v time.Time)`

SetNextPeriodicLaunch sets NextPeriodicLaunch field to given value.

### HasNextPeriodicLaunch

`func (o *JobPlanResponse) HasNextPeriodicLaunch() bool`

HasNextPeriodicLaunch returns a boolean if a field has been set.

### GetWarnings

`func (o *JobPlanResponse) GetWarnings() string`

GetWarnings returns the Warnings field if non-nil, zero value otherwise.

### GetWarningsOk

`func (o *JobPlanResponse) GetWarningsOk() (*string, bool)`

GetWarningsOk returns a tuple with the Warnings field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetWarnings

`func (o *JobPlanResponse) SetWarnings(v string)`

SetWarnings sets Warnings field to given value.

### HasWarnings

`func (o *JobPlanResponse) HasWarnings() bool`

HasWarnings returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


