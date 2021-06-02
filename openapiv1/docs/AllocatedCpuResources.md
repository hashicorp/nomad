# AllocatedCpuResources

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**CpuShares** | Pointer to **int64** |  | [optional] 
**ReservedCores** | Pointer to **[]int32** |  | [optional] 

## Methods

### NewAllocatedCpuResources

`func NewAllocatedCpuResources() *AllocatedCpuResources`

NewAllocatedCpuResources instantiates a new AllocatedCpuResources object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewAllocatedCpuResourcesWithDefaults

`func NewAllocatedCpuResourcesWithDefaults() *AllocatedCpuResources`

NewAllocatedCpuResourcesWithDefaults instantiates a new AllocatedCpuResources object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetCpuShares

`func (o *AllocatedCpuResources) GetCpuShares() int64`

GetCpuShares returns the CpuShares field if non-nil, zero value otherwise.

### GetCpuSharesOk

`func (o *AllocatedCpuResources) GetCpuSharesOk() (*int64, bool)`

GetCpuSharesOk returns a tuple with the CpuShares field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCpuShares

`func (o *AllocatedCpuResources) SetCpuShares(v int64)`

SetCpuShares sets CpuShares field to given value.

### HasCpuShares

`func (o *AllocatedCpuResources) HasCpuShares() bool`

HasCpuShares returns a boolean if a field has been set.

### GetReservedCores

`func (o *AllocatedCpuResources) GetReservedCores() []int32`

GetReservedCores returns the ReservedCores field if non-nil, zero value otherwise.

### GetReservedCoresOk

`func (o *AllocatedCpuResources) GetReservedCoresOk() (*[]int32, bool)`

GetReservedCoresOk returns a tuple with the ReservedCores field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetReservedCores

`func (o *AllocatedCpuResources) SetReservedCores(v []int32)`

SetReservedCores sets ReservedCores field to given value.

### HasReservedCores

`func (o *AllocatedCpuResources) HasReservedCores() bool`

HasReservedCores returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


