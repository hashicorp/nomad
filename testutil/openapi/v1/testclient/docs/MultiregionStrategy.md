# MultiregionStrategy

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**MaxParallel** | Pointer to **int32** |  | [optional] 
**OnFailure** | Pointer to **string** |  | [optional] 

## Methods

### NewMultiregionStrategy

`func NewMultiregionStrategy() *MultiregionStrategy`

NewMultiregionStrategy instantiates a new MultiregionStrategy object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewMultiregionStrategyWithDefaults

`func NewMultiregionStrategyWithDefaults() *MultiregionStrategy`

NewMultiregionStrategyWithDefaults instantiates a new MultiregionStrategy object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetMaxParallel

`func (o *MultiregionStrategy) GetMaxParallel() int32`

GetMaxParallel returns the MaxParallel field if non-nil, zero value otherwise.

### GetMaxParallelOk

`func (o *MultiregionStrategy) GetMaxParallelOk() (*int32, bool)`

GetMaxParallelOk returns a tuple with the MaxParallel field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMaxParallel

`func (o *MultiregionStrategy) SetMaxParallel(v int32)`

SetMaxParallel sets MaxParallel field to given value.

### HasMaxParallel

`func (o *MultiregionStrategy) HasMaxParallel() bool`

HasMaxParallel returns a boolean if a field has been set.

### GetOnFailure

`func (o *MultiregionStrategy) GetOnFailure() string`

GetOnFailure returns the OnFailure field if non-nil, zero value otherwise.

### GetOnFailureOk

`func (o *MultiregionStrategy) GetOnFailureOk() (*string, bool)`

GetOnFailureOk returns a tuple with the OnFailure field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetOnFailure

`func (o *MultiregionStrategy) SetOnFailure(v string)`

SetOnFailure sets OnFailure field to given value.

### HasOnFailure

`func (o *MultiregionStrategy) HasOnFailure() bool`

HasOnFailure returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


