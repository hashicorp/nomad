# ConsulGatewayProxy

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Config** | Pointer to **map[string]map[string]interface{}** |  | [optional] 
**ConnectTimeout** | Pointer to **int64** | A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years. | [optional] 
**EnvoyDNSDiscoveryType** | Pointer to **string** |  | [optional] 
**EnvoyGatewayBindAddresses** | Pointer to [**map[string]ConsulGatewayBindAddress**](ConsulGatewayBindAddress.md) |  | [optional] 
**EnvoyGatewayBindTaggedAddresses** | Pointer to **bool** |  | [optional] 
**EnvoyGatewayNoDefaultBind** | Pointer to **bool** |  | [optional] 

## Methods

### NewConsulGatewayProxy

`func NewConsulGatewayProxy() *ConsulGatewayProxy`

NewConsulGatewayProxy instantiates a new ConsulGatewayProxy object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewConsulGatewayProxyWithDefaults

`func NewConsulGatewayProxyWithDefaults() *ConsulGatewayProxy`

NewConsulGatewayProxyWithDefaults instantiates a new ConsulGatewayProxy object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetConfig

`func (o *ConsulGatewayProxy) GetConfig() map[string]map[string]interface{}`

GetConfig returns the Config field if non-nil, zero value otherwise.

### GetConfigOk

`func (o *ConsulGatewayProxy) GetConfigOk() (*map[string]map[string]interface{}, bool)`

GetConfigOk returns a tuple with the Config field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetConfig

`func (o *ConsulGatewayProxy) SetConfig(v map[string]map[string]interface{})`

SetConfig sets Config field to given value.

### HasConfig

`func (o *ConsulGatewayProxy) HasConfig() bool`

HasConfig returns a boolean if a field has been set.

### GetConnectTimeout

`func (o *ConsulGatewayProxy) GetConnectTimeout() int64`

GetConnectTimeout returns the ConnectTimeout field if non-nil, zero value otherwise.

### GetConnectTimeoutOk

`func (o *ConsulGatewayProxy) GetConnectTimeoutOk() (*int64, bool)`

GetConnectTimeoutOk returns a tuple with the ConnectTimeout field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetConnectTimeout

`func (o *ConsulGatewayProxy) SetConnectTimeout(v int64)`

SetConnectTimeout sets ConnectTimeout field to given value.

### HasConnectTimeout

`func (o *ConsulGatewayProxy) HasConnectTimeout() bool`

HasConnectTimeout returns a boolean if a field has been set.

### GetEnvoyDNSDiscoveryType

`func (o *ConsulGatewayProxy) GetEnvoyDNSDiscoveryType() string`

GetEnvoyDNSDiscoveryType returns the EnvoyDNSDiscoveryType field if non-nil, zero value otherwise.

### GetEnvoyDNSDiscoveryTypeOk

`func (o *ConsulGatewayProxy) GetEnvoyDNSDiscoveryTypeOk() (*string, bool)`

GetEnvoyDNSDiscoveryTypeOk returns a tuple with the EnvoyDNSDiscoveryType field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEnvoyDNSDiscoveryType

`func (o *ConsulGatewayProxy) SetEnvoyDNSDiscoveryType(v string)`

SetEnvoyDNSDiscoveryType sets EnvoyDNSDiscoveryType field to given value.

### HasEnvoyDNSDiscoveryType

`func (o *ConsulGatewayProxy) HasEnvoyDNSDiscoveryType() bool`

HasEnvoyDNSDiscoveryType returns a boolean if a field has been set.

### GetEnvoyGatewayBindAddresses

`func (o *ConsulGatewayProxy) GetEnvoyGatewayBindAddresses() map[string]ConsulGatewayBindAddress`

GetEnvoyGatewayBindAddresses returns the EnvoyGatewayBindAddresses field if non-nil, zero value otherwise.

### GetEnvoyGatewayBindAddressesOk

`func (o *ConsulGatewayProxy) GetEnvoyGatewayBindAddressesOk() (*map[string]ConsulGatewayBindAddress, bool)`

GetEnvoyGatewayBindAddressesOk returns a tuple with the EnvoyGatewayBindAddresses field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEnvoyGatewayBindAddresses

`func (o *ConsulGatewayProxy) SetEnvoyGatewayBindAddresses(v map[string]ConsulGatewayBindAddress)`

SetEnvoyGatewayBindAddresses sets EnvoyGatewayBindAddresses field to given value.

### HasEnvoyGatewayBindAddresses

`func (o *ConsulGatewayProxy) HasEnvoyGatewayBindAddresses() bool`

HasEnvoyGatewayBindAddresses returns a boolean if a field has been set.

### GetEnvoyGatewayBindTaggedAddresses

`func (o *ConsulGatewayProxy) GetEnvoyGatewayBindTaggedAddresses() bool`

GetEnvoyGatewayBindTaggedAddresses returns the EnvoyGatewayBindTaggedAddresses field if non-nil, zero value otherwise.

### GetEnvoyGatewayBindTaggedAddressesOk

`func (o *ConsulGatewayProxy) GetEnvoyGatewayBindTaggedAddressesOk() (*bool, bool)`

GetEnvoyGatewayBindTaggedAddressesOk returns a tuple with the EnvoyGatewayBindTaggedAddresses field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEnvoyGatewayBindTaggedAddresses

`func (o *ConsulGatewayProxy) SetEnvoyGatewayBindTaggedAddresses(v bool)`

SetEnvoyGatewayBindTaggedAddresses sets EnvoyGatewayBindTaggedAddresses field to given value.

### HasEnvoyGatewayBindTaggedAddresses

`func (o *ConsulGatewayProxy) HasEnvoyGatewayBindTaggedAddresses() bool`

HasEnvoyGatewayBindTaggedAddresses returns a boolean if a field has been set.

### GetEnvoyGatewayNoDefaultBind

`func (o *ConsulGatewayProxy) GetEnvoyGatewayNoDefaultBind() bool`

GetEnvoyGatewayNoDefaultBind returns the EnvoyGatewayNoDefaultBind field if non-nil, zero value otherwise.

### GetEnvoyGatewayNoDefaultBindOk

`func (o *ConsulGatewayProxy) GetEnvoyGatewayNoDefaultBindOk() (*bool, bool)`

GetEnvoyGatewayNoDefaultBindOk returns a tuple with the EnvoyGatewayNoDefaultBind field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEnvoyGatewayNoDefaultBind

`func (o *ConsulGatewayProxy) SetEnvoyGatewayNoDefaultBind(v bool)`

SetEnvoyGatewayNoDefaultBind sets EnvoyGatewayNoDefaultBind field to given value.

### HasEnvoyGatewayNoDefaultBind

`func (o *ConsulGatewayProxy) HasEnvoyGatewayNoDefaultBind() bool`

HasEnvoyGatewayNoDefaultBind returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


