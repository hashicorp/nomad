# Port

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**HostNetwork** | Pointer to **string** | HostNetwork is the name of the network this port should be assigned to. Jobs with a HostNetwork set can only be placed on nodes with that host network available. | [optional] 
**Label** | Pointer to **string** | Label is the key for HCL port stanzas: port \&quot;foo\&quot; {} | [optional] 
**To** | Pointer to **int64** | To is the port inside a network namespace where this port is forwarded. -1 is an internal sentinel value used by Consul Connect to mean \&quot;same as the host port.\&quot; | [optional] 
**Value** | Pointer to **int64** | Value is the static or dynamic port value. For dynamic ports this will be 0 in the jobspec and set by the scheduler. | [optional] 

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

`func (o *Port) GetTo() int64`

GetTo returns the To field if non-nil, zero value otherwise.

### GetToOk

`func (o *Port) GetToOk() (*int64, bool)`

GetToOk returns a tuple with the To field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTo

`func (o *Port) SetTo(v int64)`

SetTo sets To field to given value.

### HasTo

`func (o *Port) HasTo() bool`

HasTo returns a boolean if a field has been set.

### GetValue

`func (o *Port) GetValue() int64`

GetValue returns the Value field if non-nil, zero value otherwise.

### GetValueOk

`func (o *Port) GetValueOk() (*int64, bool)`

GetValueOk returns a tuple with the Value field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetValue

`func (o *Port) SetValue(v int64)`

SetValue sets Value field to given value.

### HasValue

`func (o *Port) HasValue() bool`

HasValue returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


