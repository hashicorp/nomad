# AllocatedSharedResources

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**DiskMB** | Pointer to **int64** |  | [optional] 
**Networks** | Pointer to [**[]NetworkResource**](NetworkResource.md) |  | [optional] 
**Ports** | Pointer to [**[]PortMapping**](PortMapping.md) |  | [optional] 

## Methods

### NewAllocatedSharedResources

`func NewAllocatedSharedResources() *AllocatedSharedResources`

NewAllocatedSharedResources instantiates a new AllocatedSharedResources object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewAllocatedSharedResourcesWithDefaults

`func NewAllocatedSharedResourcesWithDefaults() *AllocatedSharedResources`

NewAllocatedSharedResourcesWithDefaults instantiates a new AllocatedSharedResources object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetDiskMB

`func (o *AllocatedSharedResources) GetDiskMB() int64`

GetDiskMB returns the DiskMB field if non-nil, zero value otherwise.

### GetDiskMBOk

`func (o *AllocatedSharedResources) GetDiskMBOk() (*int64, bool)`

GetDiskMBOk returns a tuple with the DiskMB field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDiskMB

`func (o *AllocatedSharedResources) SetDiskMB(v int64)`

SetDiskMB sets DiskMB field to given value.

### HasDiskMB

`func (o *AllocatedSharedResources) HasDiskMB() bool`

HasDiskMB returns a boolean if a field has been set.

### GetNetworks

`func (o *AllocatedSharedResources) GetNetworks() []NetworkResource`

GetNetworks returns the Networks field if non-nil, zero value otherwise.

### GetNetworksOk

`func (o *AllocatedSharedResources) GetNetworksOk() (*[]NetworkResource, bool)`

GetNetworksOk returns a tuple with the Networks field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNetworks

`func (o *AllocatedSharedResources) SetNetworks(v []NetworkResource)`

SetNetworks sets Networks field to given value.

### HasNetworks

`func (o *AllocatedSharedResources) HasNetworks() bool`

HasNetworks returns a boolean if a field has been set.

### GetPorts

`func (o *AllocatedSharedResources) GetPorts() []PortMapping`

GetPorts returns the Ports field if non-nil, zero value otherwise.

### GetPortsOk

`func (o *AllocatedSharedResources) GetPortsOk() (*[]PortMapping, bool)`

GetPortsOk returns a tuple with the Ports field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPorts

`func (o *AllocatedSharedResources) SetPorts(v []PortMapping)`

SetPorts sets Ports field to given value.

### HasPorts

`func (o *AllocatedSharedResources) HasPorts() bool`

HasPorts returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


