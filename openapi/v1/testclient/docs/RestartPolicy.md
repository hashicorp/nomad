# RestartPolicy

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Attempts** | Pointer to **int64** |  | [optional] 
**Delay** | Pointer to **int64** | A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years. | [optional] 
**Interval** | Pointer to **int64** | A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years. | [optional] 
**Mode** | Pointer to **string** |  | [optional] 

## Methods

### NewRestartPolicy

`func NewRestartPolicy() *RestartPolicy`

NewRestartPolicy instantiates a new RestartPolicy object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewRestartPolicyWithDefaults

`func NewRestartPolicyWithDefaults() *RestartPolicy`

NewRestartPolicyWithDefaults instantiates a new RestartPolicy object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetAttempts

`func (o *RestartPolicy) GetAttempts() int64`

GetAttempts returns the Attempts field if non-nil, zero value otherwise.

### GetAttemptsOk

`func (o *RestartPolicy) GetAttemptsOk() (*int64, bool)`

GetAttemptsOk returns a tuple with the Attempts field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAttempts

`func (o *RestartPolicy) SetAttempts(v int64)`

SetAttempts sets Attempts field to given value.

### HasAttempts

`func (o *RestartPolicy) HasAttempts() bool`

HasAttempts returns a boolean if a field has been set.

### GetDelay

`func (o *RestartPolicy) GetDelay() int64`

GetDelay returns the Delay field if non-nil, zero value otherwise.

### GetDelayOk

`func (o *RestartPolicy) GetDelayOk() (*int64, bool)`

GetDelayOk returns a tuple with the Delay field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDelay

`func (o *RestartPolicy) SetDelay(v int64)`

SetDelay sets Delay field to given value.

### HasDelay

`func (o *RestartPolicy) HasDelay() bool`

HasDelay returns a boolean if a field has been set.

### GetInterval

`func (o *RestartPolicy) GetInterval() int64`

GetInterval returns the Interval field if non-nil, zero value otherwise.

### GetIntervalOk

`func (o *RestartPolicy) GetIntervalOk() (*int64, bool)`

GetIntervalOk returns a tuple with the Interval field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetInterval

`func (o *RestartPolicy) SetInterval(v int64)`

SetInterval sets Interval field to given value.

### HasInterval

`func (o *RestartPolicy) HasInterval() bool`

HasInterval returns a boolean if a field has been set.

### GetMode

`func (o *RestartPolicy) GetMode() string`

GetMode returns the Mode field if non-nil, zero value otherwise.

### GetModeOk

`func (o *RestartPolicy) GetModeOk() (*string, bool)`

GetModeOk returns a tuple with the Mode field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMode

`func (o *RestartPolicy) SetMode(v string)`

SetMode sets Mode field to given value.

### HasMode

`func (o *RestartPolicy) HasMode() bool`

HasMode returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


