# MigrateStrategy

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**HealthCheck** | Pointer to **string** |  | [optional] 
**HealthyDeadline** | Pointer to **int64** | A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years. | [optional] 
**MaxParallel** | Pointer to **int64** |  | [optional] 
**MinHealthyTime** | Pointer to **int64** | A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years. | [optional] 

## Methods

### NewMigrateStrategy

`func NewMigrateStrategy() *MigrateStrategy`

NewMigrateStrategy instantiates a new MigrateStrategy object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewMigrateStrategyWithDefaults

`func NewMigrateStrategyWithDefaults() *MigrateStrategy`

NewMigrateStrategyWithDefaults instantiates a new MigrateStrategy object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetHealthCheck

`func (o *MigrateStrategy) GetHealthCheck() string`

GetHealthCheck returns the HealthCheck field if non-nil, zero value otherwise.

### GetHealthCheckOk

`func (o *MigrateStrategy) GetHealthCheckOk() (*string, bool)`

GetHealthCheckOk returns a tuple with the HealthCheck field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetHealthCheck

`func (o *MigrateStrategy) SetHealthCheck(v string)`

SetHealthCheck sets HealthCheck field to given value.

### HasHealthCheck

`func (o *MigrateStrategy) HasHealthCheck() bool`

HasHealthCheck returns a boolean if a field has been set.

### GetHealthyDeadline

`func (o *MigrateStrategy) GetHealthyDeadline() int64`

GetHealthyDeadline returns the HealthyDeadline field if non-nil, zero value otherwise.

### GetHealthyDeadlineOk

`func (o *MigrateStrategy) GetHealthyDeadlineOk() (*int64, bool)`

GetHealthyDeadlineOk returns a tuple with the HealthyDeadline field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetHealthyDeadline

`func (o *MigrateStrategy) SetHealthyDeadline(v int64)`

SetHealthyDeadline sets HealthyDeadline field to given value.

### HasHealthyDeadline

`func (o *MigrateStrategy) HasHealthyDeadline() bool`

HasHealthyDeadline returns a boolean if a field has been set.

### GetMaxParallel

`func (o *MigrateStrategy) GetMaxParallel() int64`

GetMaxParallel returns the MaxParallel field if non-nil, zero value otherwise.

### GetMaxParallelOk

`func (o *MigrateStrategy) GetMaxParallelOk() (*int64, bool)`

GetMaxParallelOk returns a tuple with the MaxParallel field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMaxParallel

`func (o *MigrateStrategy) SetMaxParallel(v int64)`

SetMaxParallel sets MaxParallel field to given value.

### HasMaxParallel

`func (o *MigrateStrategy) HasMaxParallel() bool`

HasMaxParallel returns a boolean if a field has been set.

### GetMinHealthyTime

`func (o *MigrateStrategy) GetMinHealthyTime() int64`

GetMinHealthyTime returns the MinHealthyTime field if non-nil, zero value otherwise.

### GetMinHealthyTimeOk

`func (o *MigrateStrategy) GetMinHealthyTimeOk() (*int64, bool)`

GetMinHealthyTimeOk returns a tuple with the MinHealthyTime field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMinHealthyTime

`func (o *MigrateStrategy) SetMinHealthyTime(v int64)`

SetMinHealthyTime sets MinHealthyTime field to given value.

### HasMinHealthyTime

`func (o *MigrateStrategy) HasMinHealthyTime() bool`

HasMinHealthyTime returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


