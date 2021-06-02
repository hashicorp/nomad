# CheckRestart

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Grace** | Pointer to **int64** | A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years. | [optional] 
**IgnoreWarnings** | Pointer to **bool** |  | [optional] 
**Limit** | Pointer to **int64** |  | [optional] 

## Methods

### NewCheckRestart

`func NewCheckRestart() *CheckRestart`

NewCheckRestart instantiates a new CheckRestart object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewCheckRestartWithDefaults

`func NewCheckRestartWithDefaults() *CheckRestart`

NewCheckRestartWithDefaults instantiates a new CheckRestart object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetGrace

`func (o *CheckRestart) GetGrace() int64`

GetGrace returns the Grace field if non-nil, zero value otherwise.

### GetGraceOk

`func (o *CheckRestart) GetGraceOk() (*int64, bool)`

GetGraceOk returns a tuple with the Grace field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetGrace

`func (o *CheckRestart) SetGrace(v int64)`

SetGrace sets Grace field to given value.

### HasGrace

`func (o *CheckRestart) HasGrace() bool`

HasGrace returns a boolean if a field has been set.

### GetIgnoreWarnings

`func (o *CheckRestart) GetIgnoreWarnings() bool`

GetIgnoreWarnings returns the IgnoreWarnings field if non-nil, zero value otherwise.

### GetIgnoreWarningsOk

`func (o *CheckRestart) GetIgnoreWarningsOk() (*bool, bool)`

GetIgnoreWarningsOk returns a tuple with the IgnoreWarnings field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetIgnoreWarnings

`func (o *CheckRestart) SetIgnoreWarnings(v bool)`

SetIgnoreWarnings sets IgnoreWarnings field to given value.

### HasIgnoreWarnings

`func (o *CheckRestart) HasIgnoreWarnings() bool`

HasIgnoreWarnings returns a boolean if a field has been set.

### GetLimit

`func (o *CheckRestart) GetLimit() int64`

GetLimit returns the Limit field if non-nil, zero value otherwise.

### GetLimitOk

`func (o *CheckRestart) GetLimitOk() (*int64, bool)`

GetLimitOk returns a tuple with the Limit field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLimit

`func (o *CheckRestart) SetLimit(v int64)`

SetLimit sets Limit field to given value.

### HasLimit

`func (o *CheckRestart) HasLimit() bool`

HasLimit returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


