# Port

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**HostNetwork** | Pointer to **string** |  | [optional] 
**Label** | Pointer to **string** |  | [optional] 
**To** | Pointer to **int32** |  | [optional] 
**Value** | Pointer to **int32** |  | [optional] 

## Methods

### NewPort

`func NewPort() *Port`

NewPort instantiates a new Port object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewPortWithDefaults

`func NewPortWithDefaults() *Port`

NewPortWithDefaults instantiates a new Port object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetHostNetwork

`func (o *Port) GetHostNetwork() string`

GetHostNetwork returns the HostNetwork field if non-nil, zero value otherwise.

### GetHostNetworkOk

`func (o *Port) GetHostNetworkOk() (*string, bool)`

GetHostNetworkOk returns a tuple with the HostNetwork field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetHostNetwork

`func (o *Port) SetHostNetwork(v string)`

SetHostNetwork sets HostNetwork field to given value.

### HasHostNetwork

`func (o *Port) HasHostNetwork() bool`

HasHostNetwork returns a boolean if a field has been set.

### GetLabel

`func (o *Port) GetLabel() string`

GetLabel returns the Label field if non-nil, zero value otherwise.

### GetLabelOk

`func (o *Port) GetLabelOk() (*string, bool)`

GetLabelOk returns a tuple with the Label field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLabel

`func (o *Port) SetLabel(v string)`

SetLabel sets Label field to given value.

### HasLabel

`func (o *Port) HasLabel() bool`

HasLabel returns a boolean if a field has been set.

### GetTo

`func (o *Port) GetTo() int32`

GetTo returns the To field if non-nil, zero value otherwise.

### GetToOk

`func (o *Port) GetToOk() (*int32, bool)`

GetToOk returns a tuple with the To field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTo

`func (o *Port) SetTo(v int32)`

SetTo sets To field to given value.

### HasTo

`func (o *Port) HasTo() bool`

HasTo returns a boolean if a field has been set.

### GetValue

`func (o *Port) GetValue() int32`

GetValue returns the Value field if non-nil, zero value otherwise.

### GetValueOk

`func (o *Port) GetValueOk() (*int32, bool)`

GetValueOk returns a tuple with the Value field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetValue

`func (o *Port) SetValue(v int32)`

SetValue sets Value field to given value.

### HasValue

`func (o *Port) HasValue() bool`

HasValue returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


