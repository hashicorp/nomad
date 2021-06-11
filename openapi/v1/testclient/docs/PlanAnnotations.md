# PlanAnnotations

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**DesiredTGUpdates** | Pointer to [**map[string]DesiredUpdates**](DesiredUpdates.md) |  | [optional] 
**PreemptedAllocs** | Pointer to [**[]AllocationListStub**](AllocationListStub.md) |  | [optional] 

## Methods

### NewPlanAnnotations

`func NewPlanAnnotations() *PlanAnnotations`

NewPlanAnnotations instantiates a new PlanAnnotations object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewPlanAnnotationsWithDefaults

`func NewPlanAnnotationsWithDefaults() *PlanAnnotations`

NewPlanAnnotationsWithDefaults instantiates a new PlanAnnotations object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetDesiredTGUpdates

`func (o *PlanAnnotations) GetDesiredTGUpdates() map[string]DesiredUpdates`

GetDesiredTGUpdates returns the DesiredTGUpdates field if non-nil, zero value otherwise.

### GetDesiredTGUpdatesOk

`func (o *PlanAnnotations) GetDesiredTGUpdatesOk() (*map[string]DesiredUpdates, bool)`

GetDesiredTGUpdatesOk returns a tuple with the DesiredTGUpdates field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDesiredTGUpdates

`func (o *PlanAnnotations) SetDesiredTGUpdates(v map[string]DesiredUpdates)`

SetDesiredTGUpdates sets DesiredTGUpdates field to given value.

### HasDesiredTGUpdates

`func (o *PlanAnnotations) HasDesiredTGUpdates() bool`

HasDesiredTGUpdates returns a boolean if a field has been set.

### GetPreemptedAllocs

`func (o *PlanAnnotations) GetPreemptedAllocs() []AllocationListStub`

GetPreemptedAllocs returns the PreemptedAllocs field if non-nil, zero value otherwise.

### GetPreemptedAllocsOk

`func (o *PlanAnnotations) GetPreemptedAllocsOk() (*[]AllocationListStub, bool)`

GetPreemptedAllocsOk returns a tuple with the PreemptedAllocs field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPreemptedAllocs

`func (o *PlanAnnotations) SetPreemptedAllocs(v []AllocationListStub)`

SetPreemptedAllocs sets PreemptedAllocs field to given value.

### HasPreemptedAllocs

`func (o *PlanAnnotations) HasPreemptedAllocs() bool`

HasPreemptedAllocs returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


