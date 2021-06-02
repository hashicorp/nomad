# ConsulSidecarService

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**DisableDefaultTCPCheck** | Pointer to **bool** |  | [optional] 
**Port** | Pointer to **string** |  | [optional] 
**Proxy** | Pointer to [**ConsulProxy**](ConsulProxy.md) |  | [optional] 
**Tags** | Pointer to **[]string** |  | [optional] 

## Methods

### NewConsulSidecarService

`func NewConsulSidecarService() *ConsulSidecarService`

NewConsulSidecarService instantiates a new ConsulSidecarService object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewConsulSidecarServiceWithDefaults

`func NewConsulSidecarServiceWithDefaults() *ConsulSidecarService`

NewConsulSidecarServiceWithDefaults instantiates a new ConsulSidecarService object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetDisableDefaultTCPCheck

`func (o *ConsulSidecarService) GetDisableDefaultTCPCheck() bool`

GetDisableDefaultTCPCheck returns the DisableDefaultTCPCheck field if non-nil, zero value otherwise.

### GetDisableDefaultTCPCheckOk

`func (o *ConsulSidecarService) GetDisableDefaultTCPCheckOk() (*bool, bool)`

GetDisableDefaultTCPCheckOk returns a tuple with the DisableDefaultTCPCheck field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDisableDefaultTCPCheck

`func (o *ConsulSidecarService) SetDisableDefaultTCPCheck(v bool)`

SetDisableDefaultTCPCheck sets DisableDefaultTCPCheck field to given value.

### HasDisableDefaultTCPCheck

`func (o *ConsulSidecarService) HasDisableDefaultTCPCheck() bool`

HasDisableDefaultTCPCheck returns a boolean if a field has been set.

### GetPort

`func (o *ConsulSidecarService) GetPort() string`

GetPort returns the Port field if non-nil, zero value otherwise.

### GetPortOk

`func (o *ConsulSidecarService) GetPortOk() (*string, bool)`

GetPortOk returns a tuple with the Port field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPort

`func (o *ConsulSidecarService) SetPort(v string)`

SetPort sets Port field to given value.

### HasPort

`func (o *ConsulSidecarService) HasPort() bool`

HasPort returns a boolean if a field has been set.

### GetProxy

`func (o *ConsulSidecarService) GetProxy() ConsulProxy`

GetProxy returns the Proxy field if non-nil, zero value otherwise.

### GetProxyOk

`func (o *ConsulSidecarService) GetProxyOk() (*ConsulProxy, bool)`

GetProxyOk returns a tuple with the Proxy field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetProxy

`func (o *ConsulSidecarService) SetProxy(v ConsulProxy)`

SetProxy sets Proxy field to given value.

### HasProxy

`func (o *ConsulSidecarService) HasProxy() bool`

HasProxy returns a boolean if a field has been set.

### GetTags

`func (o *ConsulSidecarService) GetTags() []string`

GetTags returns the Tags field if non-nil, zero value otherwise.

### GetTagsOk

`func (o *ConsulSidecarService) GetTagsOk() (*[]string, bool)`

GetTagsOk returns a tuple with the Tags field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTags

`func (o *ConsulSidecarService) SetTags(v []string)`

SetTags sets Tags field to given value.

### HasTags

`func (o *ConsulSidecarService) HasTags() bool`

HasTags returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


