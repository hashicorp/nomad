# ConsulUpstream

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Datacenter** | Pointer to **string** |  | [optional] 
**DestinationName** | Pointer to **string** |  | [optional] 
**LocalBindAddress** | Pointer to **string** |  | [optional] 
**LocalBindPort** | Pointer to **int64** |  | [optional] 

## Methods

### NewConsulUpstream

`func NewConsulUpstream() *ConsulUpstream`

NewConsulUpstream instantiates a new ConsulUpstream object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewConsulUpstreamWithDefaults

`func NewConsulUpstreamWithDefaults() *ConsulUpstream`

NewConsulUpstreamWithDefaults instantiates a new ConsulUpstream object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetDatacenter

`func (o *ConsulUpstream) GetDatacenter() string`

GetDatacenter returns the Datacenter field if non-nil, zero value otherwise.

### GetDatacenterOk

`func (o *ConsulUpstream) GetDatacenterOk() (*string, bool)`

GetDatacenterOk returns a tuple with the Datacenter field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDatacenter

`func (o *ConsulUpstream) SetDatacenter(v string)`

SetDatacenter sets Datacenter field to given value.

### HasDatacenter

`func (o *ConsulUpstream) HasDatacenter() bool`

HasDatacenter returns a boolean if a field has been set.

### GetDestinationName

`func (o *ConsulUpstream) GetDestinationName() string`

GetDestinationName returns the DestinationName field if non-nil, zero value otherwise.

### GetDestinationNameOk

`func (o *ConsulUpstream) GetDestinationNameOk() (*string, bool)`

GetDestinationNameOk returns a tuple with the DestinationName field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDestinationName

`func (o *ConsulUpstream) SetDestinationName(v string)`

SetDestinationName sets DestinationName field to given value.

### HasDestinationName

`func (o *ConsulUpstream) HasDestinationName() bool`

HasDestinationName returns a boolean if a field has been set.

### GetLocalBindAddress

`func (o *ConsulUpstream) GetLocalBindAddress() string`

GetLocalBindAddress returns the LocalBindAddress field if non-nil, zero value otherwise.

### GetLocalBindAddressOk

`func (o *ConsulUpstream) GetLocalBindAddressOk() (*string, bool)`

GetLocalBindAddressOk returns a tuple with the LocalBindAddress field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLocalBindAddress

`func (o *ConsulUpstream) SetLocalBindAddress(v string)`

SetLocalBindAddress sets LocalBindAddress field to given value.

### HasLocalBindAddress

`func (o *ConsulUpstream) HasLocalBindAddress() bool`

HasLocalBindAddress returns a boolean if a field has been set.

### GetLocalBindPort

`func (o *ConsulUpstream) GetLocalBindPort() int64`

GetLocalBindPort returns the LocalBindPort field if non-nil, zero value otherwise.

### GetLocalBindPortOk

`func (o *ConsulUpstream) GetLocalBindPortOk() (*int64, bool)`

GetLocalBindPortOk returns a tuple with the LocalBindPort field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLocalBindPort

`func (o *ConsulUpstream) SetLocalBindPort(v int64)`

SetLocalBindPort sets LocalBindPort field to given value.

### HasLocalBindPort

`func (o *ConsulUpstream) HasLocalBindPort() bool`

HasLocalBindPort returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


