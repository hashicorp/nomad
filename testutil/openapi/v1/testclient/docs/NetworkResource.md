# NetworkResource

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**CIDR** | Pointer to **string** |  | [optional] 
**DNS** | Pointer to [**DNSConfig**](DNSConfig.md) |  | [optional] 
**Device** | Pointer to **string** |  | [optional] 
**DynamicPorts** | Pointer to [**[]Port**](Port.md) |  | [optional] 
**IP** | Pointer to **string** |  | [optional] 
**MBits** | Pointer to **int32** |  | [optional] 
**Mode** | Pointer to **string** |  | [optional] 
**ReservedPorts** | Pointer to [**[]Port**](Port.md) |  | [optional] 

## Methods

### NewNetworkResource

`func NewNetworkResource() *NetworkResource`

NewNetworkResource instantiates a new NetworkResource object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewNetworkResourceWithDefaults

`func NewNetworkResourceWithDefaults() *NetworkResource`

NewNetworkResourceWithDefaults instantiates a new NetworkResource object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetCIDR

`func (o *NetworkResource) GetCIDR() string`

GetCIDR returns the CIDR field if non-nil, zero value otherwise.

### GetCIDROk

`func (o *NetworkResource) GetCIDROk() (*string, bool)`

GetCIDROk returns a tuple with the CIDR field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCIDR

`func (o *NetworkResource) SetCIDR(v string)`

SetCIDR sets CIDR field to given value.

### HasCIDR

`func (o *NetworkResource) HasCIDR() bool`

HasCIDR returns a boolean if a field has been set.

### GetDNS

`func (o *NetworkResource) GetDNS() DNSConfig`

GetDNS returns the DNS field if non-nil, zero value otherwise.

### GetDNSOk

`func (o *NetworkResource) GetDNSOk() (*DNSConfig, bool)`

GetDNSOk returns a tuple with the DNS field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDNS

`func (o *NetworkResource) SetDNS(v DNSConfig)`

SetDNS sets DNS field to given value.

### HasDNS

`func (o *NetworkResource) HasDNS() bool`

HasDNS returns a boolean if a field has been set.

### GetDevice

`func (o *NetworkResource) GetDevice() string`

GetDevice returns the Device field if non-nil, zero value otherwise.

### GetDeviceOk

`func (o *NetworkResource) GetDeviceOk() (*string, bool)`

GetDeviceOk returns a tuple with the Device field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDevice

`func (o *NetworkResource) SetDevice(v string)`

SetDevice sets Device field to given value.

### HasDevice

`func (o *NetworkResource) HasDevice() bool`

HasDevice returns a boolean if a field has been set.

### GetDynamicPorts

`func (o *NetworkResource) GetDynamicPorts() []Port`

GetDynamicPorts returns the DynamicPorts field if non-nil, zero value otherwise.

### GetDynamicPortsOk

`func (o *NetworkResource) GetDynamicPortsOk() (*[]Port, bool)`

GetDynamicPortsOk returns a tuple with the DynamicPorts field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDynamicPorts

`func (o *NetworkResource) SetDynamicPorts(v []Port)`

SetDynamicPorts sets DynamicPorts field to given value.

### HasDynamicPorts

`func (o *NetworkResource) HasDynamicPorts() bool`

HasDynamicPorts returns a boolean if a field has been set.

### GetIP

`func (o *NetworkResource) GetIP() string`

GetIP returns the IP field if non-nil, zero value otherwise.

### GetIPOk

`func (o *NetworkResource) GetIPOk() (*string, bool)`

GetIPOk returns a tuple with the IP field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetIP

`func (o *NetworkResource) SetIP(v string)`

SetIP sets IP field to given value.

### HasIP

`func (o *NetworkResource) HasIP() bool`

HasIP returns a boolean if a field has been set.

### GetMBits

`func (o *NetworkResource) GetMBits() int32`

GetMBits returns the MBits field if non-nil, zero value otherwise.

### GetMBitsOk

`func (o *NetworkResource) GetMBitsOk() (*int32, bool)`

GetMBitsOk returns a tuple with the MBits field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMBits

`func (o *NetworkResource) SetMBits(v int32)`

SetMBits sets MBits field to given value.

### HasMBits

`func (o *NetworkResource) HasMBits() bool`

HasMBits returns a boolean if a field has been set.

### GetMode

`func (o *NetworkResource) GetMode() string`

GetMode returns the Mode field if non-nil, zero value otherwise.

### GetModeOk

`func (o *NetworkResource) GetModeOk() (*string, bool)`

GetModeOk returns a tuple with the Mode field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMode

`func (o *NetworkResource) SetMode(v string)`

SetMode sets Mode field to given value.

### HasMode

`func (o *NetworkResource) HasMode() bool`

HasMode returns a boolean if a field has been set.

### GetReservedPorts

`func (o *NetworkResource) GetReservedPorts() []Port`

GetReservedPorts returns the ReservedPorts field if non-nil, zero value otherwise.

### GetReservedPortsOk

`func (o *NetworkResource) GetReservedPortsOk() (*[]Port, bool)`

GetReservedPortsOk returns a tuple with the ReservedPorts field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetReservedPorts

`func (o *NetworkResource) SetReservedPorts(v []Port)`

SetReservedPorts sets ReservedPorts field to given value.

### HasReservedPorts

`func (o *NetworkResource) HasReservedPorts() bool`

HasReservedPorts returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


