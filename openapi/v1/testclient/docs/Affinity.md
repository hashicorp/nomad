# Affinity

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**LTarget** | Pointer to **string** |  | [optional] 
**Operand** | Pointer to **string** |  | [optional] 
**RTarget** | Pointer to **string** |  | [optional] 
**Weight** | Pointer to **int32** |  | [optional] 

## Methods

### NewAffinity

`func NewAffinity() *Affinity`

NewAffinity instantiates a new Affinity object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewAffinityWithDefaults

`func NewAffinityWithDefaults() *Affinity`

NewAffinityWithDefaults instantiates a new Affinity object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetLTarget

`func (o *Affinity) GetLTarget() string`

GetLTarget returns the LTarget field if non-nil, zero value otherwise.

### GetLTargetOk

`func (o *Affinity) GetLTargetOk() (*string, bool)`

GetLTargetOk returns a tuple with the LTarget field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLTarget

`func (o *Affinity) SetLTarget(v string)`

SetLTarget sets LTarget field to given value.

### HasLTarget

`func (o *Affinity) HasLTarget() bool`

HasLTarget returns a boolean if a field has been set.

### GetOperand

`func (o *Affinity) GetOperand() string`

GetOperand returns the Operand field if non-nil, zero value otherwise.

### GetOperandOk

`func (o *Affinity) GetOperandOk() (*string, bool)`

GetOperandOk returns a tuple with the Operand field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetOperand

`func (o *Affinity) SetOperand(v string)`

SetOperand sets Operand field to given value.

### HasOperand

`func (o *Affinity) HasOperand() bool`

HasOperand returns a boolean if a field has been set.

### GetRTarget

`func (o *Affinity) GetRTarget() string`

GetRTarget returns the RTarget field if non-nil, zero value otherwise.

### GetRTargetOk

`func (o *Affinity) GetRTargetOk() (*string, bool)`

GetRTargetOk returns a tuple with the RTarget field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetRTarget

`func (o *Affinity) SetRTarget(v string)`

SetRTarget sets RTarget field to given value.

### HasRTarget

`func (o *Affinity) HasRTarget() bool`

HasRTarget returns a boolean if a field has been set.

### GetWeight

`func (o *Affinity) GetWeight() int32`

GetWeight returns the Weight field if non-nil, zero value otherwise.

### GetWeightOk

`func (o *Affinity) GetWeightOk() (*int32, bool)`

GetWeightOk returns a tuple with the Weight field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetWeight

`func (o *Affinity) SetWeight(v int32)`

SetWeight sets Weight field to given value.

### HasWeight

`func (o *Affinity) HasWeight() bool`

HasWeight returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


