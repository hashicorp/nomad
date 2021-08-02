# ScalingPolicy

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**CreateIndex** | Pointer to **int32** |  | [optional] 
**Enabled** | Pointer to **bool** |  | [optional] 
**ID** | Pointer to **string** |  | [optional] 
**Max** | Pointer to **int64** |  | [optional] 
**Min** | Pointer to **int64** |  | [optional] 
**ModifyIndex** | Pointer to **int32** |  | [optional] 
**Namespace** | Pointer to **string** |  | [optional] 
**Policy** | Pointer to **map[string]interface{}** |  | [optional] 
**Target** | Pointer to **map[string]string** |  | [optional] 
**Type** | Pointer to **string** |  | [optional] 

## Methods

### NewScalingPolicy

`func NewScalingPolicy() *ScalingPolicy`

NewScalingPolicy instantiates a new ScalingPolicy object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewScalingPolicyWithDefaults

`func NewScalingPolicyWithDefaults() *ScalingPolicy`

NewScalingPolicyWithDefaults instantiates a new ScalingPolicy object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetCreateIndex

`func (o *ScalingPolicy) GetCreateIndex() int32`

GetCreateIndex returns the CreateIndex field if non-nil, zero value otherwise.

### GetCreateIndexOk

`func (o *ScalingPolicy) GetCreateIndexOk() (*int32, bool)`

GetCreateIndexOk returns a tuple with the CreateIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCreateIndex

`func (o *ScalingPolicy) SetCreateIndex(v int32)`

SetCreateIndex sets CreateIndex field to given value.

### HasCreateIndex

`func (o *ScalingPolicy) HasCreateIndex() bool`

HasCreateIndex returns a boolean if a field has been set.

### GetEnabled

`func (o *ScalingPolicy) GetEnabled() bool`

GetEnabled returns the Enabled field if non-nil, zero value otherwise.

### GetEnabledOk

`func (o *ScalingPolicy) GetEnabledOk() (*bool, bool)`

GetEnabledOk returns a tuple with the Enabled field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEnabled

`func (o *ScalingPolicy) SetEnabled(v bool)`

SetEnabled sets Enabled field to given value.

### HasEnabled

`func (o *ScalingPolicy) HasEnabled() bool`

HasEnabled returns a boolean if a field has been set.

### GetID

`func (o *ScalingPolicy) GetID() string`

GetID returns the ID field if non-nil, zero value otherwise.

### GetIDOk

`func (o *ScalingPolicy) GetIDOk() (*string, bool)`

GetIDOk returns a tuple with the ID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetID

`func (o *ScalingPolicy) SetID(v string)`

SetID sets ID field to given value.

### HasID

`func (o *ScalingPolicy) HasID() bool`

HasID returns a boolean if a field has been set.

### GetMax

`func (o *ScalingPolicy) GetMax() int64`

GetMax returns the Max field if non-nil, zero value otherwise.

### GetMaxOk

`func (o *ScalingPolicy) GetMaxOk() (*int64, bool)`

GetMaxOk returns a tuple with the Max field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMax

`func (o *ScalingPolicy) SetMax(v int64)`

SetMax sets Max field to given value.

### HasMax

`func (o *ScalingPolicy) HasMax() bool`

HasMax returns a boolean if a field has been set.

### GetMin

`func (o *ScalingPolicy) GetMin() int64`

GetMin returns the Min field if non-nil, zero value otherwise.

### GetMinOk

`func (o *ScalingPolicy) GetMinOk() (*int64, bool)`

GetMinOk returns a tuple with the Min field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMin

`func (o *ScalingPolicy) SetMin(v int64)`

SetMin sets Min field to given value.

### HasMin

`func (o *ScalingPolicy) HasMin() bool`

HasMin returns a boolean if a field has been set.

### GetModifyIndex

`func (o *ScalingPolicy) GetModifyIndex() int32`

GetModifyIndex returns the ModifyIndex field if non-nil, zero value otherwise.

### GetModifyIndexOk

`func (o *ScalingPolicy) GetModifyIndexOk() (*int32, bool)`

GetModifyIndexOk returns a tuple with the ModifyIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetModifyIndex

`func (o *ScalingPolicy) SetModifyIndex(v int32)`

SetModifyIndex sets ModifyIndex field to given value.

### HasModifyIndex

`func (o *ScalingPolicy) HasModifyIndex() bool`

HasModifyIndex returns a boolean if a field has been set.

### GetNamespace

`func (o *ScalingPolicy) GetNamespace() string`

GetNamespace returns the Namespace field if non-nil, zero value otherwise.

### GetNamespaceOk

`func (o *ScalingPolicy) GetNamespaceOk() (*string, bool)`

GetNamespaceOk returns a tuple with the Namespace field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNamespace

`func (o *ScalingPolicy) SetNamespace(v string)`

SetNamespace sets Namespace field to given value.

### HasNamespace

`func (o *ScalingPolicy) HasNamespace() bool`

HasNamespace returns a boolean if a field has been set.

### GetPolicy

`func (o *ScalingPolicy) GetPolicy() map[string]interface{}`

GetPolicy returns the Policy field if non-nil, zero value otherwise.

### GetPolicyOk

`func (o *ScalingPolicy) GetPolicyOk() (*map[string]interface{}, bool)`

GetPolicyOk returns a tuple with the Policy field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPolicy

`func (o *ScalingPolicy) SetPolicy(v map[string]interface{})`

SetPolicy sets Policy field to given value.

### HasPolicy

`func (o *ScalingPolicy) HasPolicy() bool`

HasPolicy returns a boolean if a field has been set.

### GetTarget

`func (o *ScalingPolicy) GetTarget() map[string]string`

GetTarget returns the Target field if non-nil, zero value otherwise.

### GetTargetOk

`func (o *ScalingPolicy) GetTargetOk() (*map[string]string, bool)`

GetTargetOk returns a tuple with the Target field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTarget

`func (o *ScalingPolicy) SetTarget(v map[string]string)`

SetTarget sets Target field to given value.

### HasTarget

`func (o *ScalingPolicy) HasTarget() bool`

HasTarget returns a boolean if a field has been set.

### GetType

`func (o *ScalingPolicy) GetType() string`

GetType returns the Type field if non-nil, zero value otherwise.

### GetTypeOk

`func (o *ScalingPolicy) GetTypeOk() (*string, bool)`

GetTypeOk returns a tuple with the Type field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetType

`func (o *ScalingPolicy) SetType(v string)`

SetType sets Type field to given value.

### HasType

`func (o *ScalingPolicy) HasType() bool`

HasType returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


