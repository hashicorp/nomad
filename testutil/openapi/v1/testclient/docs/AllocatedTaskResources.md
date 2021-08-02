# AllocatedTaskResources

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Cpu** | Pointer to [**AllocatedCpuResources**](AllocatedCpuResources.md) |  | [optional] 
**Devices** | Pointer to [**[]AllocatedDeviceResource**](AllocatedDeviceResource.md) |  | [optional] 
**Memory** | Pointer to [**AllocatedMemoryResources**](AllocatedMemoryResources.md) |  | [optional] 
**Networks** | Pointer to [**[]NetworkResource**](NetworkResource.md) |  | [optional] 

## Methods

### NewAllocatedTaskResources

`func NewAllocatedTaskResources() *AllocatedTaskResources`

NewAllocatedTaskResources instantiates a new AllocatedTaskResources object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewAllocatedTaskResourcesWithDefaults

`func NewAllocatedTaskResourcesWithDefaults() *AllocatedTaskResources`

NewAllocatedTaskResourcesWithDefaults instantiates a new AllocatedTaskResources object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetCpu

`func (o *AllocatedTaskResources) GetCpu() AllocatedCpuResources`

GetCpu returns the Cpu field if non-nil, zero value otherwise.

### GetCpuOk

`func (o *AllocatedTaskResources) GetCpuOk() (*AllocatedCpuResources, bool)`

GetCpuOk returns a tuple with the Cpu field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCpu

`func (o *AllocatedTaskResources) SetCpu(v AllocatedCpuResources)`

SetCpu sets Cpu field to given value.

### HasCpu

`func (o *AllocatedTaskResources) HasCpu() bool`

HasCpu returns a boolean if a field has been set.

### GetDevices

`func (o *AllocatedTaskResources) GetDevices() []AllocatedDeviceResource`

GetDevices returns the Devices field if non-nil, zero value otherwise.

### GetDevicesOk

`func (o *AllocatedTaskResources) GetDevicesOk() (*[]AllocatedDeviceResource, bool)`

GetDevicesOk returns a tuple with the Devices field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDevices

`func (o *AllocatedTaskResources) SetDevices(v []AllocatedDeviceResource)`

SetDevices sets Devices field to given value.

### HasDevices

`func (o *AllocatedTaskResources) HasDevices() bool`

HasDevices returns a boolean if a field has been set.

### GetMemory

`func (o *AllocatedTaskResources) GetMemory() AllocatedMemoryResources`

GetMemory returns the Memory field if non-nil, zero value otherwise.

### GetMemoryOk

`func (o *AllocatedTaskResources) GetMemoryOk() (*AllocatedMemoryResources, bool)`

GetMemoryOk returns a tuple with the Memory field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMemory

`func (o *AllocatedTaskResources) SetMemory(v AllocatedMemoryResources)`

SetMemory sets Memory field to given value.

### HasMemory

`func (o *AllocatedTaskResources) HasMemory() bool`

HasMemory returns a boolean if a field has been set.

### GetNetworks

`func (o *AllocatedTaskResources) GetNetworks() []NetworkResource`

GetNetworks returns the Networks field if non-nil, zero value otherwise.

### GetNetworksOk

`func (o *AllocatedTaskResources) GetNetworksOk() (*[]NetworkResource, bool)`

GetNetworksOk returns a tuple with the Networks field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNetworks

`func (o *AllocatedTaskResources) SetNetworks(v []NetworkResource)`

SetNetworks sets Networks field to given value.

### HasNetworks

`func (o *AllocatedTaskResources) HasNetworks() bool`

HasNetworks returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


