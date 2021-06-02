# UpdateStrategy

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**AutoPromote** | Pointer to **bool** |  | [optional] 
**AutoRevert** | Pointer to **bool** |  | [optional] 
**Canary** | Pointer to **int64** |  | [optional] 
**HealthCheck** | Pointer to **string** |  | [optional] 
**HealthyDeadline** | Pointer to **int64** | A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years. | [optional] 
**MaxParallel** | Pointer to **int64** |  | [optional] 
**MinHealthyTime** | Pointer to **int64** | A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years. | [optional] 
**ProgressDeadline** | Pointer to **int64** | A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years. | [optional] 
**Stagger** | Pointer to **int64** | A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years. | [optional] 

## Methods

### NewUpdateStrategy

`func NewUpdateStrategy() *UpdateStrategy`

NewUpdateStrategy instantiates a new UpdateStrategy object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewUpdateStrategyWithDefaults

`func NewUpdateStrategyWithDefaults() *UpdateStrategy`

NewUpdateStrategyWithDefaults instantiates a new UpdateStrategy object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetAutoPromote

`func (o *UpdateStrategy) GetAutoPromote() bool`

GetAutoPromote returns the AutoPromote field if non-nil, zero value otherwise.

### GetAutoPromoteOk

`func (o *UpdateStrategy) GetAutoPromoteOk() (*bool, bool)`

GetAutoPromoteOk returns a tuple with the AutoPromote field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAutoPromote

`func (o *UpdateStrategy) SetAutoPromote(v bool)`

SetAutoPromote sets AutoPromote field to given value.

### HasAutoPromote

`func (o *UpdateStrategy) HasAutoPromote() bool`

HasAutoPromote returns a boolean if a field has been set.

### GetAutoRevert

`func (o *UpdateStrategy) GetAutoRevert() bool`

GetAutoRevert returns the AutoRevert field if non-nil, zero value otherwise.

### GetAutoRevertOk

`func (o *UpdateStrategy) GetAutoRevertOk() (*bool, bool)`

GetAutoRevertOk returns a tuple with the AutoRevert field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAutoRevert

`func (o *UpdateStrategy) SetAutoRevert(v bool)`

SetAutoRevert sets AutoRevert field to given value.

### HasAutoRevert

`func (o *UpdateStrategy) HasAutoRevert() bool`

HasAutoRevert returns a boolean if a field has been set.

### GetCanary

`func (o *UpdateStrategy) GetCanary() int64`

GetCanary returns the Canary field if non-nil, zero value otherwise.

### GetCanaryOk

`func (o *UpdateStrategy) GetCanaryOk() (*int64, bool)`

GetCanaryOk returns a tuple with the Canary field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCanary

`func (o *UpdateStrategy) SetCanary(v int64)`

SetCanary sets Canary field to given value.

### HasCanary

`func (o *UpdateStrategy) HasCanary() bool`

HasCanary returns a boolean if a field has been set.

### GetHealthCheck

`func (o *UpdateStrategy) GetHealthCheck() string`

GetHealthCheck returns the HealthCheck field if non-nil, zero value otherwise.

### GetHealthCheckOk

`func (o *UpdateStrategy) GetHealthCheckOk() (*string, bool)`

GetHealthCheckOk returns a tuple with the HealthCheck field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetHealthCheck

`func (o *UpdateStrategy) SetHealthCheck(v string)`

SetHealthCheck sets HealthCheck field to given value.

### HasHealthCheck

`func (o *UpdateStrategy) HasHealthCheck() bool`

HasHealthCheck returns a boolean if a field has been set.

### GetHealthyDeadline

`func (o *UpdateStrategy) GetHealthyDeadline() int64`

GetHealthyDeadline returns the HealthyDeadline field if non-nil, zero value otherwise.

### GetHealthyDeadlineOk

`func (o *UpdateStrategy) GetHealthyDeadlineOk() (*int64, bool)`

GetHealthyDeadlineOk returns a tuple with the HealthyDeadline field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetHealthyDeadline

`func (o *UpdateStrategy) SetHealthyDeadline(v int64)`

SetHealthyDeadline sets HealthyDeadline field to given value.

### HasHealthyDeadline

`func (o *UpdateStrategy) HasHealthyDeadline() bool`

HasHealthyDeadline returns a boolean if a field has been set.

### GetMaxParallel

`func (o *UpdateStrategy) GetMaxParallel() int64`

GetMaxParallel returns the MaxParallel field if non-nil, zero value otherwise.

### GetMaxParallelOk

`func (o *UpdateStrategy) GetMaxParallelOk() (*int64, bool)`

GetMaxParallelOk returns a tuple with the MaxParallel field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMaxParallel

`func (o *UpdateStrategy) SetMaxParallel(v int64)`

SetMaxParallel sets MaxParallel field to given value.

### HasMaxParallel

`func (o *UpdateStrategy) HasMaxParallel() bool`

HasMaxParallel returns a boolean if a field has been set.

### GetMinHealthyTime

`func (o *UpdateStrategy) GetMinHealthyTime() int64`

GetMinHealthyTime returns the MinHealthyTime field if non-nil, zero value otherwise.

### GetMinHealthyTimeOk

`func (o *UpdateStrategy) GetMinHealthyTimeOk() (*int64, bool)`

GetMinHealthyTimeOk returns a tuple with the MinHealthyTime field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMinHealthyTime

`func (o *UpdateStrategy) SetMinHealthyTime(v int64)`

SetMinHealthyTime sets MinHealthyTime field to given value.

### HasMinHealthyTime

`func (o *UpdateStrategy) HasMinHealthyTime() bool`

HasMinHealthyTime returns a boolean if a field has been set.

### GetProgressDeadline

`func (o *UpdateStrategy) GetProgressDeadline() int64`

GetProgressDeadline returns the ProgressDeadline field if non-nil, zero value otherwise.

### GetProgressDeadlineOk

`func (o *UpdateStrategy) GetProgressDeadlineOk() (*int64, bool)`

GetProgressDeadlineOk returns a tuple with the ProgressDeadline field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetProgressDeadline

`func (o *UpdateStrategy) SetProgressDeadline(v int64)`

SetProgressDeadline sets ProgressDeadline field to given value.

### HasProgressDeadline

`func (o *UpdateStrategy) HasProgressDeadline() bool`

HasProgressDeadline returns a boolean if a field has been set.

### GetStagger

`func (o *UpdateStrategy) GetStagger() int64`

GetStagger returns the Stagger field if non-nil, zero value otherwise.

### GetStaggerOk

`func (o *UpdateStrategy) GetStaggerOk() (*int64, bool)`

GetStaggerOk returns a tuple with the Stagger field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetStagger

`func (o *UpdateStrategy) SetStagger(v int64)`

SetStagger sets Stagger field to given value.

### HasStagger

`func (o *UpdateStrategy) HasStagger() bool`

HasStagger returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


