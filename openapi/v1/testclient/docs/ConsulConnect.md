# ConsulConnect

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Gateway** | Pointer to [**ConsulGateway**](ConsulGateway.md) |  | [optional] 
**Native** | Pointer to **bool** |  | [optional] 
**SidecarService** | Pointer to [**ConsulSidecarService**](ConsulSidecarService.md) |  | [optional] 
**SidecarTask** | Pointer to [**SidecarTask**](SidecarTask.md) |  | [optional] 

## Methods

### NewConsulConnect

`func NewConsulConnect() *ConsulConnect`

NewConsulConnect instantiates a new ConsulConnect object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewConsulConnectWithDefaults

`func NewConsulConnectWithDefaults() *ConsulConnect`

NewConsulConnectWithDefaults instantiates a new ConsulConnect object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetGateway

`func (o *ConsulConnect) GetGateway() ConsulGateway`

GetGateway returns the Gateway field if non-nil, zero value otherwise.

### GetGatewayOk

`func (o *ConsulConnect) GetGatewayOk() (*ConsulGateway, bool)`

GetGatewayOk returns a tuple with the Gateway field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetGateway

`func (o *ConsulConnect) SetGateway(v ConsulGateway)`

SetGateway sets Gateway field to given value.

### HasGateway

`func (o *ConsulConnect) HasGateway() bool`

HasGateway returns a boolean if a field has been set.

### GetNative

`func (o *ConsulConnect) GetNative() bool`

GetNative returns the Native field if non-nil, zero value otherwise.

### GetNativeOk

`func (o *ConsulConnect) GetNativeOk() (*bool, bool)`

GetNativeOk returns a tuple with the Native field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNative

`func (o *ConsulConnect) SetNative(v bool)`

SetNative sets Native field to given value.

### HasNative

`func (o *ConsulConnect) HasNative() bool`

HasNative returns a boolean if a field has been set.

### GetSidecarService

`func (o *ConsulConnect) GetSidecarService() ConsulSidecarService`

GetSidecarService returns the SidecarService field if non-nil, zero value otherwise.

### GetSidecarServiceOk

`func (o *ConsulConnect) GetSidecarServiceOk() (*ConsulSidecarService, bool)`

GetSidecarServiceOk returns a tuple with the SidecarService field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSidecarService

`func (o *ConsulConnect) SetSidecarService(v ConsulSidecarService)`

SetSidecarService sets SidecarService field to given value.

### HasSidecarService

`func (o *ConsulConnect) HasSidecarService() bool`

HasSidecarService returns a boolean if a field has been set.

### GetSidecarTask

`func (o *ConsulConnect) GetSidecarTask() SidecarTask`

GetSidecarTask returns the SidecarTask field if non-nil, zero value otherwise.

### GetSidecarTaskOk

`func (o *ConsulConnect) GetSidecarTaskOk() (*SidecarTask, bool)`

GetSidecarTaskOk returns a tuple with the SidecarTask field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSidecarTask

`func (o *ConsulConnect) SetSidecarTask(v SidecarTask)`

SetSidecarTask sets SidecarTask field to given value.

### HasSidecarTask

`func (o *ConsulConnect) HasSidecarTask() bool`

HasSidecarTask returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


