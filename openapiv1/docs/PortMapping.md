# PortMapping

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**HostIP** | Pointer to **string** |  | [optional] 
**Label** | Pointer to **string** |  | [optional] 
**To** | Pointer to **int64** |  | [optional] 
**Value** | Pointer to **int64** |  | [optional] 

## Methods

### NewPortMapping

`func NewPortMapping() *PortMapping`

NewPortMapping instantiates a new PortMapping object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewPortMappingWithDefaults

`func NewPortMappingWithDefaults() *PortMapping`

NewPortMappingWithDefaults instantiates a new PortMapping object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetHostIP

`func (o *PortMapping) GetHostIP() string`

GetHostIP returns the HostIP field if non-nil, zero value otherwise.

### GetHostIPOk

`func (o *PortMapping) GetHostIPOk() (*string, bool)`

GetHostIPOk returns a tuple with the HostIP field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetHostIP

`func (o *PortMapping) SetHostIP(v string)`

SetHostIP sets HostIP field to given value.

### HasHostIP

`func (o *PortMapping) HasHostIP() bool`

HasHostIP returns a boolean if a field has been set.

### GetLabel

`func (o *PortMapping) GetLabel() string`

GetLabel returns the Label field if non-nil, zero value otherwise.

### GetLabelOk

`func (o *PortMapping) GetLabelOk() (*string, bool)`

GetLabelOk returns a tuple with the Label field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLabel

`func (o *PortMapping) SetLabel(v string)`

SetLabel sets Label field to given value.

### HasLabel

`func (o *PortMapping) HasLabel() bool`

HasLabel returns a boolean if a field has been set.

### GetTo

`func (o *PortMapping) GetTo() int64`

GetTo returns the To field if non-nil, zero value otherwise.

### GetToOk

`func (o *PortMapping) GetToOk() (*int64, bool)`

GetToOk returns a tuple with the To field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTo

`func (o *PortMapping) SetTo(v int64)`

SetTo sets To field to given value.

### HasTo

`func (o *PortMapping) HasTo() bool`

HasTo returns a boolean if a field has been set.

### GetValue

`func (o *PortMapping) GetValue() int64`

GetValue returns the Value field if non-nil, zero value otherwise.

### GetValueOk

`func (o *PortMapping) GetValueOk() (*int64, bool)`

GetValueOk returns a tuple with the Value field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetValue

`func (o *PortMapping) SetValue(v int64)`

SetValue sets Value field to given value.

### HasValue

`func (o *PortMapping) HasValue() bool`

HasValue returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


