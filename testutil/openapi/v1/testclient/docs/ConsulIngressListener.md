# ConsulIngressListener

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Port** | Pointer to **int32** |  | [optional] 
**Protocol** | Pointer to **string** |  | [optional] 
**Services** | Pointer to [**[]ConsulIngressService**](ConsulIngressService.md) |  | [optional] 

## Methods

### NewConsulIngressListener

`func NewConsulIngressListener() *ConsulIngressListener`

NewConsulIngressListener instantiates a new ConsulIngressListener object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewConsulIngressListenerWithDefaults

`func NewConsulIngressListenerWithDefaults() *ConsulIngressListener`

NewConsulIngressListenerWithDefaults instantiates a new ConsulIngressListener object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetPort

`func (o *ConsulIngressListener) GetPort() int32`

GetPort returns the Port field if non-nil, zero value otherwise.

### GetPortOk

`func (o *ConsulIngressListener) GetPortOk() (*int32, bool)`

GetPortOk returns a tuple with the Port field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPort

`func (o *ConsulIngressListener) SetPort(v int32)`

SetPort sets Port field to given value.

### HasPort

`func (o *ConsulIngressListener) HasPort() bool`

HasPort returns a boolean if a field has been set.

### GetProtocol

`func (o *ConsulIngressListener) GetProtocol() string`

GetProtocol returns the Protocol field if non-nil, zero value otherwise.

### GetProtocolOk

`func (o *ConsulIngressListener) GetProtocolOk() (*string, bool)`

GetProtocolOk returns a tuple with the Protocol field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetProtocol

`func (o *ConsulIngressListener) SetProtocol(v string)`

SetProtocol sets Protocol field to given value.

### HasProtocol

`func (o *ConsulIngressListener) HasProtocol() bool`

HasProtocol returns a boolean if a field has been set.

### GetServices

`func (o *ConsulIngressListener) GetServices() []ConsulIngressService`

GetServices returns the Services field if non-nil, zero value otherwise.

### GetServicesOk

`func (o *ConsulIngressListener) GetServicesOk() (*[]ConsulIngressService, bool)`

GetServicesOk returns a tuple with the Services field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetServices

`func (o *ConsulIngressListener) SetServices(v []ConsulIngressService)`

SetServices sets Services field to given value.

### HasServices

`func (o *ConsulIngressListener) HasServices() bool`

HasServices returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


