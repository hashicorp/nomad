# ReschedulePolicy

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Attempts** | Pointer to **int32** |  | [optional] 
**Delay** | Pointer to **int64** |  | [optional] 
**DelayFunction** | Pointer to **string** |  | [optional] 
**Interval** | Pointer to **int64** |  | [optional] 
**MaxDelay** | Pointer to **int64** |  | [optional] 
**Unlimited** | Pointer to **bool** |  | [optional] 

## Methods

### NewReschedulePolicy

`func NewReschedulePolicy() *ReschedulePolicy`

NewReschedulePolicy instantiates a new ReschedulePolicy object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewReschedulePolicyWithDefaults

`func NewReschedulePolicyWithDefaults() *ReschedulePolicy`

NewReschedulePolicyWithDefaults instantiates a new ReschedulePolicy object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetAttempts

`func (o *ReschedulePolicy) GetAttempts() int32`

GetAttempts returns the Attempts field if non-nil, zero value otherwise.

### GetAttemptsOk

`func (o *ReschedulePolicy) GetAttemptsOk() (*int32, bool)`

GetAttemptsOk returns a tuple with the Attempts field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAttempts

`func (o *ReschedulePolicy) SetAttempts(v int32)`

SetAttempts sets Attempts field to given value.

### HasAttempts

`func (o *ReschedulePolicy) HasAttempts() bool`

HasAttempts returns a boolean if a field has been set.

### GetDelay

`func (o *ReschedulePolicy) GetDelay() int64`

GetDelay returns the Delay field if non-nil, zero value otherwise.

### GetDelayOk

`func (o *ReschedulePolicy) GetDelayOk() (*int64, bool)`

GetDelayOk returns a tuple with the Delay field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDelay

`func (o *ReschedulePolicy) SetDelay(v int64)`

SetDelay sets Delay field to given value.

### HasDelay

`func (o *ReschedulePolicy) HasDelay() bool`

HasDelay returns a boolean if a field has been set.

### GetDelayFunction

`func (o *ReschedulePolicy) GetDelayFunction() string`

GetDelayFunction returns the DelayFunction field if non-nil, zero value otherwise.

### GetDelayFunctionOk

`func (o *ReschedulePolicy) GetDelayFunctionOk() (*string, bool)`

GetDelayFunctionOk returns a tuple with the DelayFunction field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDelayFunction

`func (o *ReschedulePolicy) SetDelayFunction(v string)`

SetDelayFunction sets DelayFunction field to given value.

### HasDelayFunction

`func (o *ReschedulePolicy) HasDelayFunction() bool`

HasDelayFunction returns a boolean if a field has been set.

### GetInterval

`func (o *ReschedulePolicy) GetInterval() int64`

GetInterval returns the Interval field if non-nil, zero value otherwise.

### GetIntervalOk

`func (o *ReschedulePolicy) GetIntervalOk() (*int64, bool)`

GetIntervalOk returns a tuple with the Interval field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetInterval

`func (o *ReschedulePolicy) SetInterval(v int64)`

SetInterval sets Interval field to given value.

### HasInterval

`func (o *ReschedulePolicy) HasInterval() bool`

HasInterval returns a boolean if a field has been set.

### GetMaxDelay

`func (o *ReschedulePolicy) GetMaxDelay() int64`

GetMaxDelay returns the MaxDelay field if non-nil, zero value otherwise.

### GetMaxDelayOk

`func (o *ReschedulePolicy) GetMaxDelayOk() (*int64, bool)`

GetMaxDelayOk returns a tuple with the MaxDelay field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMaxDelay

`func (o *ReschedulePolicy) SetMaxDelay(v int64)`

SetMaxDelay sets MaxDelay field to given value.

### HasMaxDelay

`func (o *ReschedulePolicy) HasMaxDelay() bool`

HasMaxDelay returns a boolean if a field has been set.

### GetUnlimited

`func (o *ReschedulePolicy) GetUnlimited() bool`

GetUnlimited returns the Unlimited field if non-nil, zero value otherwise.

### GetUnlimitedOk

`func (o *ReschedulePolicy) GetUnlimitedOk() (*bool, bool)`

GetUnlimitedOk returns a tuple with the Unlimited field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetUnlimited

`func (o *ReschedulePolicy) SetUnlimited(v bool)`

SetUnlimited sets Unlimited field to given value.

### HasUnlimited

`func (o *ReschedulePolicy) HasUnlimited() bool`

HasUnlimited returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


