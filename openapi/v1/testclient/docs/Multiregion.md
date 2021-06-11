# Multiregion

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Regions** | Pointer to [**[]MultiregionRegion**](MultiregionRegion.md) |  | [optional] 
**Strategy** | Pointer to [**MultiregionStrategy**](MultiregionStrategy.md) |  | [optional] 

## Methods

### NewMultiregion

`func NewMultiregion() *Multiregion`

NewMultiregion instantiates a new Multiregion object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewMultiregionWithDefaults

`func NewMultiregionWithDefaults() *Multiregion`

NewMultiregionWithDefaults instantiates a new Multiregion object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetRegions

`func (o *Multiregion) GetRegions() []MultiregionRegion`

GetRegions returns the Regions field if non-nil, zero value otherwise.

### GetRegionsOk

`func (o *Multiregion) GetRegionsOk() (*[]MultiregionRegion, bool)`

GetRegionsOk returns a tuple with the Regions field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetRegions

`func (o *Multiregion) SetRegions(v []MultiregionRegion)`

SetRegions sets Regions field to given value.

### HasRegions

`func (o *Multiregion) HasRegions() bool`

HasRegions returns a boolean if a field has been set.

### GetStrategy

`func (o *Multiregion) GetStrategy() MultiregionStrategy`

GetStrategy returns the Strategy field if non-nil, zero value otherwise.

### GetStrategyOk

`func (o *Multiregion) GetStrategyOk() (*MultiregionStrategy, bool)`

GetStrategyOk returns a tuple with the Strategy field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetStrategy

`func (o *Multiregion) SetStrategy(v MultiregionStrategy)`

SetStrategy sets Strategy field to given value.

### HasStrategy

`func (o *Multiregion) HasStrategy() bool`

HasStrategy returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


