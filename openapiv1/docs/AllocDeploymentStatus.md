# AllocDeploymentStatus

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Canary** | Pointer to **bool** | Canary marks whether the allocation is a canary or not. A canary that has been promoted will have this field set to false. | [optional] 
**Healthy** | Pointer to **bool** | Healthy marks whether the allocation has been marked healthy or unhealthy as part of a deployment. It can be unset if it has neither been marked healthy or unhealthy. | [optional] 
**ModifyIndex** | Pointer to **int32** | ModifyIndex is the raft index in which the deployment status was last changed. | [optional] 
**Timestamp** | Pointer to **time.Time** | Timestamp is the time at which the health status was set. | [optional] 

## Methods

### NewAllocDeploymentStatus

`func NewAllocDeploymentStatus() *AllocDeploymentStatus`

NewAllocDeploymentStatus instantiates a new AllocDeploymentStatus object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewAllocDeploymentStatusWithDefaults

`func NewAllocDeploymentStatusWithDefaults() *AllocDeploymentStatus`

NewAllocDeploymentStatusWithDefaults instantiates a new AllocDeploymentStatus object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetCanary

`func (o *AllocDeploymentStatus) GetCanary() bool`

GetCanary returns the Canary field if non-nil, zero value otherwise.

### GetCanaryOk

`func (o *AllocDeploymentStatus) GetCanaryOk() (*bool, bool)`

GetCanaryOk returns a tuple with the Canary field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCanary

`func (o *AllocDeploymentStatus) SetCanary(v bool)`

SetCanary sets Canary field to given value.

### HasCanary

`func (o *AllocDeploymentStatus) HasCanary() bool`

HasCanary returns a boolean if a field has been set.

### GetHealthy

`func (o *AllocDeploymentStatus) GetHealthy() bool`

GetHealthy returns the Healthy field if non-nil, zero value otherwise.

### GetHealthyOk

`func (o *AllocDeploymentStatus) GetHealthyOk() (*bool, bool)`

GetHealthyOk returns a tuple with the Healthy field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetHealthy

`func (o *AllocDeploymentStatus) SetHealthy(v bool)`

SetHealthy sets Healthy field to given value.

### HasHealthy

`func (o *AllocDeploymentStatus) HasHealthy() bool`

HasHealthy returns a boolean if a field has been set.

### GetModifyIndex

`func (o *AllocDeploymentStatus) GetModifyIndex() int32`

GetModifyIndex returns the ModifyIndex field if non-nil, zero value otherwise.

### GetModifyIndexOk

`func (o *AllocDeploymentStatus) GetModifyIndexOk() (*int32, bool)`

GetModifyIndexOk returns a tuple with the ModifyIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetModifyIndex

`func (o *AllocDeploymentStatus) SetModifyIndex(v int32)`

SetModifyIndex sets ModifyIndex field to given value.

### HasModifyIndex

`func (o *AllocDeploymentStatus) HasModifyIndex() bool`

HasModifyIndex returns a boolean if a field has been set.

### GetTimestamp

`func (o *AllocDeploymentStatus) GetTimestamp() time.Time`

GetTimestamp returns the Timestamp field if non-nil, zero value otherwise.

### GetTimestampOk

`func (o *AllocDeploymentStatus) GetTimestampOk() (*time.Time, bool)`

GetTimestampOk returns a tuple with the Timestamp field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTimestamp

`func (o *AllocDeploymentStatus) SetTimestamp(v time.Time)`

SetTimestamp sets Timestamp field to given value.

### HasTimestamp

`func (o *AllocDeploymentStatus) HasTimestamp() bool`

HasTimestamp returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


