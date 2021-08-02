# ConsulGateway

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Ingress** | Pointer to [**ConsulIngressConfigEntry**](ConsulIngressConfigEntry.md) |  | [optional] 
**Mesh** | Pointer to **interface{}** |  | [optional] 
**Proxy** | Pointer to [**ConsulGatewayProxy**](ConsulGatewayProxy.md) |  | [optional] 
**Terminating** | Pointer to [**ConsulTerminatingConfigEntry**](ConsulTerminatingConfigEntry.md) |  | [optional] 

## Methods

### NewConsulGateway

`func NewConsulGateway() *ConsulGateway`

NewConsulGateway instantiates a new ConsulGateway object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewConsulGatewayWithDefaults

`func NewConsulGatewayWithDefaults() *ConsulGateway`

NewConsulGatewayWithDefaults instantiates a new ConsulGateway object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetIngress

`func (o *ConsulGateway) GetIngress() ConsulIngressConfigEntry`

GetIngress returns the Ingress field if non-nil, zero value otherwise.

### GetIngressOk

`func (o *ConsulGateway) GetIngressOk() (*ConsulIngressConfigEntry, bool)`

GetIngressOk returns a tuple with the Ingress field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetIngress

`func (o *ConsulGateway) SetIngress(v ConsulIngressConfigEntry)`

SetIngress sets Ingress field to given value.

### HasIngress

`func (o *ConsulGateway) HasIngress() bool`

HasIngress returns a boolean if a field has been set.

### GetMesh

`func (o *ConsulGateway) GetMesh() interface{}`

GetMesh returns the Mesh field if non-nil, zero value otherwise.

### GetMeshOk

`func (o *ConsulGateway) GetMeshOk() (*interface{}, bool)`

GetMeshOk returns a tuple with the Mesh field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMesh

`func (o *ConsulGateway) SetMesh(v interface{})`

SetMesh sets Mesh field to given value.

### HasMesh

`func (o *ConsulGateway) HasMesh() bool`

HasMesh returns a boolean if a field has been set.

### SetMeshNil

`func (o *ConsulGateway) SetMeshNil(b bool)`

 SetMeshNil sets the value for Mesh to be an explicit nil

### UnsetMesh
`func (o *ConsulGateway) UnsetMesh()`

UnsetMesh ensures that no value is present for Mesh, not even an explicit nil
### GetProxy

`func (o *ConsulGateway) GetProxy() ConsulGatewayProxy`

GetProxy returns the Proxy field if non-nil, zero value otherwise.

### GetProxyOk

`func (o *ConsulGateway) GetProxyOk() (*ConsulGatewayProxy, bool)`

GetProxyOk returns a tuple with the Proxy field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetProxy

`func (o *ConsulGateway) SetProxy(v ConsulGatewayProxy)`

SetProxy sets Proxy field to given value.

### HasProxy

`func (o *ConsulGateway) HasProxy() bool`

HasProxy returns a boolean if a field has been set.

### GetTerminating

`func (o *ConsulGateway) GetTerminating() ConsulTerminatingConfigEntry`

GetTerminating returns the Terminating field if non-nil, zero value otherwise.

### GetTerminatingOk

`func (o *ConsulGateway) GetTerminatingOk() (*ConsulTerminatingConfigEntry, bool)`

GetTerminatingOk returns a tuple with the Terminating field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTerminating

`func (o *ConsulGateway) SetTerminating(v ConsulTerminatingConfigEntry)`

SetTerminating sets Terminating field to given value.

### HasTerminating

`func (o *ConsulGateway) HasTerminating() bool`

HasTerminating returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


