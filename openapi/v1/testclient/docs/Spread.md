# Spread

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Attribute** | Pointer to **string** |  | [optional] 
**SpreadTarget** | Pointer to [**[]SpreadTarget**](SpreadTarget.md) |  | [optional] 
**Weight** | Pointer to **int32** |  | [optional] 

## Methods

### NewSpread

`func NewSpread() *Spread`

NewSpread instantiates a new Spread object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewSpreadWithDefaults

`func NewSpreadWithDefaults() *Spread`

NewSpreadWithDefaults instantiates a new Spread object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetAttribute

`func (o *Spread) GetAttribute() string`

GetAttribute returns the Attribute field if non-nil, zero value otherwise.

### GetAttributeOk

`func (o *Spread) GetAttributeOk() (*string, bool)`

GetAttributeOk returns a tuple with the Attribute field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAttribute

`func (o *Spread) SetAttribute(v string)`

SetAttribute sets Attribute field to given value.

### HasAttribute

`func (o *Spread) HasAttribute() bool`

HasAttribute returns a boolean if a field has been set.

### GetSpreadTarget

`func (o *Spread) GetSpreadTarget() []SpreadTarget`

GetSpreadTarget returns the SpreadTarget field if non-nil, zero value otherwise.

### GetSpreadTargetOk

`func (o *Spread) GetSpreadTargetOk() (*[]SpreadTarget, bool)`

GetSpreadTargetOk returns a tuple with the SpreadTarget field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSpreadTarget

`func (o *Spread) SetSpreadTarget(v []SpreadTarget)`

SetSpreadTarget sets SpreadTarget field to given value.

### HasSpreadTarget

`func (o *Spread) HasSpreadTarget() bool`

HasSpreadTarget returns a boolean if a field has been set.

### GetWeight

`func (o *Spread) GetWeight() int32`

GetWeight returns the Weight field if non-nil, zero value otherwise.

### GetWeightOk

`func (o *Spread) GetWeightOk() (*int32, bool)`

GetWeightOk returns a tuple with the Weight field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetWeight

`func (o *Spread) SetWeight(v int32)`

SetWeight sets Weight field to given value.

### HasWeight

`func (o *Spread) HasWeight() bool`

HasWeight returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


