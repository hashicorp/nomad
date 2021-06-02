# ConsulProxy

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Config** | Pointer to **map[string]map[string]interface{}** |  | [optional] 
**ExposeConfig** | Pointer to [**ConsulExposeConfig**](ConsulExposeConfig.md) |  | [optional] 
**LocalServiceAddress** | Pointer to **string** |  | [optional] 
**LocalServicePort** | Pointer to **int64** |  | [optional] 
**Upstreams** | Pointer to [**[]ConsulUpstream**](ConsulUpstream.md) |  | [optional] 

## Methods

### NewConsulProxy

`func NewConsulProxy() *ConsulProxy`

NewConsulProxy instantiates a new ConsulProxy object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewConsulProxyWithDefaults

`func NewConsulProxyWithDefaults() *ConsulProxy`

NewConsulProxyWithDefaults instantiates a new ConsulProxy object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetConfig

`func (o *ConsulProxy) GetConfig() map[string]map[string]interface{}`

GetConfig returns the Config field if non-nil, zero value otherwise.

### GetConfigOk

`func (o *ConsulProxy) GetConfigOk() (*map[string]map[string]interface{}, bool)`

GetConfigOk returns a tuple with the Config field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetConfig

`func (o *ConsulProxy) SetConfig(v map[string]map[string]interface{})`

SetConfig sets Config field to given value.

### HasConfig

`func (o *ConsulProxy) HasConfig() bool`

HasConfig returns a boolean if a field has been set.

### GetExposeConfig

`func (o *ConsulProxy) GetExposeConfig() ConsulExposeConfig`

GetExposeConfig returns the ExposeConfig field if non-nil, zero value otherwise.

### GetExposeConfigOk

`func (o *ConsulProxy) GetExposeConfigOk() (*ConsulExposeConfig, bool)`

GetExposeConfigOk returns a tuple with the ExposeConfig field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetExposeConfig

`func (o *ConsulProxy) SetExposeConfig(v ConsulExposeConfig)`

SetExposeConfig sets ExposeConfig field to given value.

### HasExposeConfig

`func (o *ConsulProxy) HasExposeConfig() bool`

HasExposeConfig returns a boolean if a field has been set.

### GetLocalServiceAddress

`func (o *ConsulProxy) GetLocalServiceAddress() string`

GetLocalServiceAddress returns the LocalServiceAddress field if non-nil, zero value otherwise.

### GetLocalServiceAddressOk

`func (o *ConsulProxy) GetLocalServiceAddressOk() (*string, bool)`

GetLocalServiceAddressOk returns a tuple with the LocalServiceAddress field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLocalServiceAddress

`func (o *ConsulProxy) SetLocalServiceAddress(v string)`

SetLocalServiceAddress sets LocalServiceAddress field to given value.

### HasLocalServiceAddress

`func (o *ConsulProxy) HasLocalServiceAddress() bool`

HasLocalServiceAddress returns a boolean if a field has been set.

### GetLocalServicePort

`func (o *ConsulProxy) GetLocalServicePort() int64`

GetLocalServicePort returns the LocalServicePort field if non-nil, zero value otherwise.

### GetLocalServicePortOk

`func (o *ConsulProxy) GetLocalServicePortOk() (*int64, bool)`

GetLocalServicePortOk returns a tuple with the LocalServicePort field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLocalServicePort

`func (o *ConsulProxy) SetLocalServicePort(v int64)`

SetLocalServicePort sets LocalServicePort field to given value.

### HasLocalServicePort

`func (o *ConsulProxy) HasLocalServicePort() bool`

HasLocalServicePort returns a boolean if a field has been set.

### GetUpstreams

`func (o *ConsulProxy) GetUpstreams() []ConsulUpstream`

GetUpstreams returns the Upstreams field if non-nil, zero value otherwise.

### GetUpstreamsOk

`func (o *ConsulProxy) GetUpstreamsOk() (*[]ConsulUpstream, bool)`

GetUpstreamsOk returns a tuple with the Upstreams field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetUpstreams

`func (o *ConsulProxy) SetUpstreams(v []ConsulUpstream)`

SetUpstreams sets Upstreams field to given value.

### HasUpstreams

`func (o *ConsulProxy) HasUpstreams() bool`

HasUpstreams returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


