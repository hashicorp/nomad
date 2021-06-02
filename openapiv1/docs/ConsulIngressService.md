# ConsulIngressService

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Hosts** | Pointer to **[]string** |  | [optional] 
**Name** | Pointer to **string** | Namespace is not yet supported. Namespace string | [optional] 

## Methods

### NewConsulIngressService

`func NewConsulIngressService() *ConsulIngressService`

NewConsulIngressService instantiates a new ConsulIngressService object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewConsulIngressServiceWithDefaults

`func NewConsulIngressServiceWithDefaults() *ConsulIngressService`

NewConsulIngressServiceWithDefaults instantiates a new ConsulIngressService object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetHosts

`func (o *ConsulIngressService) GetHosts() []string`

GetHosts returns the Hosts field if non-nil, zero value otherwise.

### GetHostsOk

`func (o *ConsulIngressService) GetHostsOk() (*[]string, bool)`

GetHostsOk returns a tuple with the Hosts field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetHosts

`func (o *ConsulIngressService) SetHosts(v []string)`

SetHosts sets Hosts field to given value.

### HasHosts

`func (o *ConsulIngressService) HasHosts() bool`

HasHosts returns a boolean if a field has been set.

### GetName

`func (o *ConsulIngressService) GetName() string`

GetName returns the Name field if non-nil, zero value otherwise.

### GetNameOk

`func (o *ConsulIngressService) GetNameOk() (*string, bool)`

GetNameOk returns a tuple with the Name field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetName

`func (o *ConsulIngressService) SetName(v string)`

SetName sets Name field to given value.

### HasName

`func (o *ConsulIngressService) HasName() bool`

HasName returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


