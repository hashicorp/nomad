# AllocationMetric

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**AllocationTime** | Pointer to **int64** | A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years. | [optional] 
**ClassExhausted** | Pointer to **map[string]int64** |  | [optional] 
**ClassFiltered** | Pointer to **map[string]int64** |  | [optional] 
**CoalescedFailures** | Pointer to **int64** |  | [optional] 
**ConstraintFiltered** | Pointer to **map[string]int64** |  | [optional] 
**DimensionExhausted** | Pointer to **map[string]int64** |  | [optional] 
**NodesAvailable** | Pointer to **map[string]int64** |  | [optional] 
**NodesEvaluated** | Pointer to **int64** |  | [optional] 
**NodesExhausted** | Pointer to **int64** |  | [optional] 
**NodesFiltered** | Pointer to **int64** |  | [optional] 
**QuotaExhausted** | Pointer to **[]string** |  | [optional] 
**ResourcesExhausted** | Pointer to [**map[string]Resources**](Resources.md) |  | [optional] 
**ScoreMetaData** | Pointer to [**[]NodeScoreMeta**](NodeScoreMeta.md) |  | [optional] 
**Scores** | Pointer to **map[string]float64** | Deprecated, replaced with ScoreMetaData | [optional] 

## Methods

### NewAllocationMetric

`func NewAllocationMetric() *AllocationMetric`

NewAllocationMetric instantiates a new AllocationMetric object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewAllocationMetricWithDefaults

`func NewAllocationMetricWithDefaults() *AllocationMetric`

NewAllocationMetricWithDefaults instantiates a new AllocationMetric object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetAllocationTime

`func (o *AllocationMetric) GetAllocationTime() int64`

GetAllocationTime returns the AllocationTime field if non-nil, zero value otherwise.

### GetAllocationTimeOk

`func (o *AllocationMetric) GetAllocationTimeOk() (*int64, bool)`

GetAllocationTimeOk returns a tuple with the AllocationTime field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAllocationTime

`func (o *AllocationMetric) SetAllocationTime(v int64)`

SetAllocationTime sets AllocationTime field to given value.

### HasAllocationTime

`func (o *AllocationMetric) HasAllocationTime() bool`

HasAllocationTime returns a boolean if a field has been set.

### GetClassExhausted

`func (o *AllocationMetric) GetClassExhausted() map[string]int64`

GetClassExhausted returns the ClassExhausted field if non-nil, zero value otherwise.

### GetClassExhaustedOk

`func (o *AllocationMetric) GetClassExhaustedOk() (*map[string]int64, bool)`

GetClassExhaustedOk returns a tuple with the ClassExhausted field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetClassExhausted

`func (o *AllocationMetric) SetClassExhausted(v map[string]int64)`

SetClassExhausted sets ClassExhausted field to given value.

### HasClassExhausted

`func (o *AllocationMetric) HasClassExhausted() bool`

HasClassExhausted returns a boolean if a field has been set.

### GetClassFiltered

`func (o *AllocationMetric) GetClassFiltered() map[string]int64`

GetClassFiltered returns the ClassFiltered field if non-nil, zero value otherwise.

### GetClassFilteredOk

`func (o *AllocationMetric) GetClassFilteredOk() (*map[string]int64, bool)`

GetClassFilteredOk returns a tuple with the ClassFiltered field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetClassFiltered

`func (o *AllocationMetric) SetClassFiltered(v map[string]int64)`

SetClassFiltered sets ClassFiltered field to given value.

### HasClassFiltered

`func (o *AllocationMetric) HasClassFiltered() bool`

HasClassFiltered returns a boolean if a field has been set.

### GetCoalescedFailures

`func (o *AllocationMetric) GetCoalescedFailures() int64`

GetCoalescedFailures returns the CoalescedFailures field if non-nil, zero value otherwise.

### GetCoalescedFailuresOk

`func (o *AllocationMetric) GetCoalescedFailuresOk() (*int64, bool)`

GetCoalescedFailuresOk returns a tuple with the CoalescedFailures field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCoalescedFailures

`func (o *AllocationMetric) SetCoalescedFailures(v int64)`

SetCoalescedFailures sets CoalescedFailures field to given value.

### HasCoalescedFailures

`func (o *AllocationMetric) HasCoalescedFailures() bool`

HasCoalescedFailures returns a boolean if a field has been set.

### GetConstraintFiltered

`func (o *AllocationMetric) GetConstraintFiltered() map[string]int64`

GetConstraintFiltered returns the ConstraintFiltered field if non-nil, zero value otherwise.

### GetConstraintFilteredOk

`func (o *AllocationMetric) GetConstraintFilteredOk() (*map[string]int64, bool)`

GetConstraintFilteredOk returns a tuple with the ConstraintFiltered field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetConstraintFiltered

`func (o *AllocationMetric) SetConstraintFiltered(v map[string]int64)`

SetConstraintFiltered sets ConstraintFiltered field to given value.

### HasConstraintFiltered

`func (o *AllocationMetric) HasConstraintFiltered() bool`

HasConstraintFiltered returns a boolean if a field has been set.

### GetDimensionExhausted

`func (o *AllocationMetric) GetDimensionExhausted() map[string]int64`

GetDimensionExhausted returns the DimensionExhausted field if non-nil, zero value otherwise.

### GetDimensionExhaustedOk

`func (o *AllocationMetric) GetDimensionExhaustedOk() (*map[string]int64, bool)`

GetDimensionExhaustedOk returns a tuple with the DimensionExhausted field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDimensionExhausted

`func (o *AllocationMetric) SetDimensionExhausted(v map[string]int64)`

SetDimensionExhausted sets DimensionExhausted field to given value.

### HasDimensionExhausted

`func (o *AllocationMetric) HasDimensionExhausted() bool`

HasDimensionExhausted returns a boolean if a field has been set.

### GetNodesAvailable

`func (o *AllocationMetric) GetNodesAvailable() map[string]int64`

GetNodesAvailable returns the NodesAvailable field if non-nil, zero value otherwise.

### GetNodesAvailableOk

`func (o *AllocationMetric) GetNodesAvailableOk() (*map[string]int64, bool)`

GetNodesAvailableOk returns a tuple with the NodesAvailable field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNodesAvailable

`func (o *AllocationMetric) SetNodesAvailable(v map[string]int64)`

SetNodesAvailable sets NodesAvailable field to given value.

### HasNodesAvailable

`func (o *AllocationMetric) HasNodesAvailable() bool`

HasNodesAvailable returns a boolean if a field has been set.

### GetNodesEvaluated

`func (o *AllocationMetric) GetNodesEvaluated() int64`

GetNodesEvaluated returns the NodesEvaluated field if non-nil, zero value otherwise.

### GetNodesEvaluatedOk

`func (o *AllocationMetric) GetNodesEvaluatedOk() (*int64, bool)`

GetNodesEvaluatedOk returns a tuple with the NodesEvaluated field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNodesEvaluated

`func (o *AllocationMetric) SetNodesEvaluated(v int64)`

SetNodesEvaluated sets NodesEvaluated field to given value.

### HasNodesEvaluated

`func (o *AllocationMetric) HasNodesEvaluated() bool`

HasNodesEvaluated returns a boolean if a field has been set.

### GetNodesExhausted

`func (o *AllocationMetric) GetNodesExhausted() int64`

GetNodesExhausted returns the NodesExhausted field if non-nil, zero value otherwise.

### GetNodesExhaustedOk

`func (o *AllocationMetric) GetNodesExhaustedOk() (*int64, bool)`

GetNodesExhaustedOk returns a tuple with the NodesExhausted field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNodesExhausted

`func (o *AllocationMetric) SetNodesExhausted(v int64)`

SetNodesExhausted sets NodesExhausted field to given value.

### HasNodesExhausted

`func (o *AllocationMetric) HasNodesExhausted() bool`

HasNodesExhausted returns a boolean if a field has been set.

### GetNodesFiltered

`func (o *AllocationMetric) GetNodesFiltered() int64`

GetNodesFiltered returns the NodesFiltered field if non-nil, zero value otherwise.

### GetNodesFilteredOk

`func (o *AllocationMetric) GetNodesFilteredOk() (*int64, bool)`

GetNodesFilteredOk returns a tuple with the NodesFiltered field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNodesFiltered

`func (o *AllocationMetric) SetNodesFiltered(v int64)`

SetNodesFiltered sets NodesFiltered field to given value.

### HasNodesFiltered

`func (o *AllocationMetric) HasNodesFiltered() bool`

HasNodesFiltered returns a boolean if a field has been set.

### GetQuotaExhausted

`func (o *AllocationMetric) GetQuotaExhausted() []string`

GetQuotaExhausted returns the QuotaExhausted field if non-nil, zero value otherwise.

### GetQuotaExhaustedOk

`func (o *AllocationMetric) GetQuotaExhaustedOk() (*[]string, bool)`

GetQuotaExhaustedOk returns a tuple with the QuotaExhausted field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetQuotaExhausted

`func (o *AllocationMetric) SetQuotaExhausted(v []string)`

SetQuotaExhausted sets QuotaExhausted field to given value.

### HasQuotaExhausted

`func (o *AllocationMetric) HasQuotaExhausted() bool`

HasQuotaExhausted returns a boolean if a field has been set.

### GetResourcesExhausted

`func (o *AllocationMetric) GetResourcesExhausted() map[string]Resources`

GetResourcesExhausted returns the ResourcesExhausted field if non-nil, zero value otherwise.

### GetResourcesExhaustedOk

`func (o *AllocationMetric) GetResourcesExhaustedOk() (*map[string]Resources, bool)`

GetResourcesExhaustedOk returns a tuple with the ResourcesExhausted field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetResourcesExhausted

`func (o *AllocationMetric) SetResourcesExhausted(v map[string]Resources)`

SetResourcesExhausted sets ResourcesExhausted field to given value.

### HasResourcesExhausted

`func (o *AllocationMetric) HasResourcesExhausted() bool`

HasResourcesExhausted returns a boolean if a field has been set.

### GetScoreMetaData

`func (o *AllocationMetric) GetScoreMetaData() []NodeScoreMeta`

GetScoreMetaData returns the ScoreMetaData field if non-nil, zero value otherwise.

### GetScoreMetaDataOk

`func (o *AllocationMetric) GetScoreMetaDataOk() (*[]NodeScoreMeta, bool)`

GetScoreMetaDataOk returns a tuple with the ScoreMetaData field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetScoreMetaData

`func (o *AllocationMetric) SetScoreMetaData(v []NodeScoreMeta)`

SetScoreMetaData sets ScoreMetaData field to given value.

### HasScoreMetaData

`func (o *AllocationMetric) HasScoreMetaData() bool`

HasScoreMetaData returns a boolean if a field has been set.

### GetScores

`func (o *AllocationMetric) GetScores() map[string]float64`

GetScores returns the Scores field if non-nil, zero value otherwise.

### GetScoresOk

`func (o *AllocationMetric) GetScoresOk() (*map[string]float64, bool)`

GetScoresOk returns a tuple with the Scores field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetScores

`func (o *AllocationMetric) SetScores(v map[string]float64)`

SetScores sets Scores field to given value.

### HasScores

`func (o *AllocationMetric) HasScores() bool`

HasScores returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


