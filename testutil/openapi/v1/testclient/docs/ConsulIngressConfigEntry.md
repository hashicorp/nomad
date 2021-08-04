# ConsulIngressConfigEntry

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Listeners** | Pointer to [**[]ConsulIngressListener**](ConsulIngressListener.md) |  | [optional] 
**TLS** | Pointer to [**ConsulGatewayTLSConfig**](ConsulGatewayTLSConfig.md) |  | [optional] 

## Methods

### NewConsulIngressConfigEntry

`func NewConsulIngressConfigEntry() *ConsulIngressConfigEntry`

NewConsulIngressConfigEntry instantiates a new ConsulIngressConfigEntry object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewConsulIngressConfigEntryWithDefaults

`func NewConsulIngressConfigEntryWithDefaults() *ConsulIngressConfigEntry`

NewConsulIngressConfigEntryWithDefaults instantiates a new ConsulIngressConfigEntry object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetListeners

`func (o *ConsulIngressConfigEntry) GetListeners() []ConsulIngressListener`

GetListeners returns the Listeners field if non-nil, zero value otherwise.

### GetListenersOk

`func (o *ConsulIngressConfigEntry) GetListenersOk() (*[]ConsulIngressListener, bool)`

GetListenersOk returns a tuple with the Listeners field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetListeners

`func (o *ConsulIngressConfigEntry) SetListeners(v []ConsulIngressListener)`

SetListeners sets Listeners field to given value.

### HasListeners

`func (o *ConsulIngressConfigEntry) HasListeners() bool`

HasListeners returns a boolean if a field has been set.

### GetTLS

`func (o *ConsulIngressConfigEntry) GetTLS() ConsulGatewayTLSConfig`

GetTLS returns the TLS field if non-nil, zero value otherwise.

### GetTLSOk

`func (o *ConsulIngressConfigEntry) GetTLSOk() (*ConsulGatewayTLSConfig, bool)`

GetTLSOk returns a tuple with the TLS field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTLS

`func (o *ConsulIngressConfigEntry) SetTLS(v ConsulGatewayTLSConfig)`

SetTLS sets TLS field to given value.

### HasTLS

`func (o *ConsulIngressConfigEntry) HasTLS() bool`

HasTLS returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


