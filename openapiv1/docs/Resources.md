# Resources

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**CPU** | Pointer to **int64** |  | [optional] 
**Cores** | Pointer to **int64** |  | [optional] 
**Devices** | Pointer to [**[]RequestedDevice**](RequestedDevice.md) |  | [optional] 
**DiskMB** | Pointer to **int64** |  | [optional] 
**IOPS** | Pointer to **int64** | COMPAT(0.10) XXX Deprecated. Please do not use. The field will be removed in Nomad 0.10 and is only being kept to allow any references to be removed before then. | [optional] 
**MemoryMB** | Pointer to **int64** |  | [optional] 
**MemoryMaxMB** | Pointer to **int64** |  | [optional] 
**Networks** | Pointer to [**[]NetworkResource**](NetworkResource.md) |  | [optional] 

## Methods

### NewResources

`func NewResources() *Resources`

NewResources instantiates a new Resources object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewResourcesWithDefaults

`func NewResourcesWithDefaults() *Resources`

NewResourcesWithDefaults instantiates a new Resources object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetCPU

`func (o *Resources) GetCPU() int64`

GetCPU returns the CPU field if non-nil, zero value otherwise.

### GetCPUOk

`func (o *Resources) GetCPUOk() (*int64, bool)`

GetCPUOk returns a tuple with the CPU field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCPU

`func (o *Resources) SetCPU(v int64)`

SetCPU sets CPU field to given value.

### HasCPU

`func (o *Resources) HasCPU() bool`

HasCPU returns a boolean if a field has been set.

### GetCores

`func (o *Resources) GetCores() int64`

GetCores returns the Cores field if non-nil, zero value otherwise.

### GetCoresOk

`func (o *Resources) GetCoresOk() (*int64, bool)`

GetCoresOk returns a tuple with the Cores field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCores

`func (o *Resources) SetCores(v int64)`

SetCores sets Cores field to given value.

### HasCores

`func (o *Resources) HasCores() bool`

HasCores returns a boolean if a field has been set.

### GetDevices

`func (o *Resources) GetDevices() []RequestedDevice`

GetDevices returns the Devices field if non-nil, zero value otherwise.

### GetDevicesOk

`func (o *Resources) GetDevicesOk() (*[]RequestedDevice, bool)`

GetDevicesOk returns a tuple with the Devices field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDevices

`func (o *Resources) SetDevices(v []RequestedDevice)`

SetDevices sets Devices field to given value.

### HasDevices

`func (o *Resources) HasDevices() bool`

HasDevices returns a boolean if a field has been set.

### GetDiskMB

`func (o *Resources) GetDiskMB() int64`

GetDiskMB returns the DiskMB field if non-nil, zero value otherwise.

### GetDiskMBOk

`func (o *Resources) GetDiskMBOk() (*int64, bool)`

GetDiskMBOk returns a tuple with the DiskMB field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDiskMB

`func (o *Resources) SetDiskMB(v int64)`

SetDiskMB sets DiskMB field to given value.

### HasDiskMB

`func (o *Resources) HasDiskMB() bool`

HasDiskMB returns a boolean if a field has been set.

### GetIOPS

`func (o *Resources) GetIOPS() int64`

GetIOPS returns the IOPS field if non-nil, zero value otherwise.

### GetIOPSOk

`func (o *Resources) GetIOPSOk() (*int64, bool)`

GetIOPSOk returns a tuple with the IOPS field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetIOPS

`func (o *Resources) SetIOPS(v int64)`

SetIOPS sets IOPS field to given value.

### HasIOPS

`func (o *Resources) HasIOPS() bool`

HasIOPS returns a boolean if a field has been set.

### GetMemoryMB

`func (o *Resources) GetMemoryMB() int64`

GetMemoryMB returns the MemoryMB field if non-nil, zero value otherwise.

### GetMemoryMBOk

`func (o *Resources) GetMemoryMBOk() (*int64, bool)`

GetMemoryMBOk returns a tuple with the MemoryMB field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMemoryMB

`func (o *Resources) SetMemoryMB(v int64)`

SetMemoryMB sets MemoryMB field to given value.

### HasMemoryMB

`func (o *Resources) HasMemoryMB() bool`

HasMemoryMB returns a boolean if a field has been set.

### GetMemoryMaxMB

`func (o *Resources) GetMemoryMaxMB() int64`

GetMemoryMaxMB returns the MemoryMaxMB field if non-nil, zero value otherwise.

### GetMemoryMaxMBOk

`func (o *Resources) GetMemoryMaxMBOk() (*int64, bool)`

GetMemoryMaxMBOk returns a tuple with the MemoryMaxMB field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMemoryMaxMB

`func (o *Resources) SetMemoryMaxMB(v int64)`

SetMemoryMaxMB sets MemoryMaxMB field to given value.

### HasMemoryMaxMB

`func (o *Resources) HasMemoryMaxMB() bool`

HasMemoryMaxMB returns a boolean if a field has been set.

### GetNetworks

`func (o *Resources) GetNetworks() []NetworkResource`

GetNetworks returns the Networks field if non-nil, zero value otherwise.

### GetNetworksOk

`func (o *Resources) GetNetworksOk() (*[]NetworkResource, bool)`

GetNetworksOk returns a tuple with the Networks field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNetworks

`func (o *Resources) SetNetworks(v []NetworkResource)`

SetNetworks sets Networks field to given value.

### HasNetworks

`func (o *Resources) HasNetworks() bool`

HasNetworks returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


