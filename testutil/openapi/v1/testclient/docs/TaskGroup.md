# TaskGroup

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Affinities** | Pointer to [**[]Affinity**](Affinity.md) |  | [optional] 
**Constraints** | Pointer to [**[]Constraint**](Constraint.md) |  | [optional] 
**Consul** | Pointer to [**Consul**](Consul.md) |  | [optional] 
**Count** | Pointer to **int32** |  | [optional] 
**EphemeralDisk** | Pointer to [**EphemeralDisk**](EphemeralDisk.md) |  | [optional] 
**Meta** | Pointer to **map[string]string** |  | [optional] 
**Migrate** | Pointer to [**MigrateStrategy**](MigrateStrategy.md) |  | [optional] 
**Name** | Pointer to **string** |  | [optional] 
**Networks** | Pointer to [**[]NetworkResource**](NetworkResource.md) |  | [optional] 
**ReschedulePolicy** | Pointer to [**ReschedulePolicy**](ReschedulePolicy.md) |  | [optional] 
**RestartPolicy** | Pointer to [**RestartPolicy**](RestartPolicy.md) |  | [optional] 
**Scaling** | Pointer to [**ScalingPolicy**](ScalingPolicy.md) |  | [optional] 
**Services** | Pointer to [**[]Service**](Service.md) |  | [optional] 
**ShutdownDelay** | Pointer to **int64** |  | [optional] 
**Spreads** | Pointer to [**[]Spread**](Spread.md) |  | [optional] 
**StopAfterClientDisconnect** | Pointer to **int64** |  | [optional] 
**Tasks** | Pointer to [**[]Task**](Task.md) |  | [optional] 
**Update** | Pointer to [**UpdateStrategy**](UpdateStrategy.md) |  | [optional] 
**Volumes** | Pointer to [**map[string]VolumeRequest**](VolumeRequest.md) |  | [optional] 

## Methods

### NewTaskGroup

`func NewTaskGroup() *TaskGroup`

NewTaskGroup instantiates a new TaskGroup object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewTaskGroupWithDefaults

`func NewTaskGroupWithDefaults() *TaskGroup`

NewTaskGroupWithDefaults instantiates a new TaskGroup object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetAffinities

`func (o *TaskGroup) GetAffinities() []Affinity`

GetAffinities returns the Affinities field if non-nil, zero value otherwise.

### GetAffinitiesOk

`func (o *TaskGroup) GetAffinitiesOk() (*[]Affinity, bool)`

GetAffinitiesOk returns a tuple with the Affinities field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAffinities

`func (o *TaskGroup) SetAffinities(v []Affinity)`

SetAffinities sets Affinities field to given value.

### HasAffinities

`func (o *TaskGroup) HasAffinities() bool`

HasAffinities returns a boolean if a field has been set.

### GetConstraints

`func (o *TaskGroup) GetConstraints() []Constraint`

GetConstraints returns the Constraints field if non-nil, zero value otherwise.

### GetConstraintsOk

`func (o *TaskGroup) GetConstraintsOk() (*[]Constraint, bool)`

GetConstraintsOk returns a tuple with the Constraints field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetConstraints

`func (o *TaskGroup) SetConstraints(v []Constraint)`

SetConstraints sets Constraints field to given value.

### HasConstraints

`func (o *TaskGroup) HasConstraints() bool`

HasConstraints returns a boolean if a field has been set.

### GetConsul

`func (o *TaskGroup) GetConsul() Consul`

GetConsul returns the Consul field if non-nil, zero value otherwise.

### GetConsulOk

`func (o *TaskGroup) GetConsulOk() (*Consul, bool)`

GetConsulOk returns a tuple with the Consul field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetConsul

`func (o *TaskGroup) SetConsul(v Consul)`

SetConsul sets Consul field to given value.

### HasConsul

`func (o *TaskGroup) HasConsul() bool`

HasConsul returns a boolean if a field has been set.

### GetCount

`func (o *TaskGroup) GetCount() int32`

GetCount returns the Count field if non-nil, zero value otherwise.

### GetCountOk

`func (o *TaskGroup) GetCountOk() (*int32, bool)`

GetCountOk returns a tuple with the Count field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCount

`func (o *TaskGroup) SetCount(v int32)`

SetCount sets Count field to given value.

### HasCount

`func (o *TaskGroup) HasCount() bool`

HasCount returns a boolean if a field has been set.

### GetEphemeralDisk

`func (o *TaskGroup) GetEphemeralDisk() EphemeralDisk`

GetEphemeralDisk returns the EphemeralDisk field if non-nil, zero value otherwise.

### GetEphemeralDiskOk

`func (o *TaskGroup) GetEphemeralDiskOk() (*EphemeralDisk, bool)`

GetEphemeralDiskOk returns a tuple with the EphemeralDisk field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEphemeralDisk

`func (o *TaskGroup) SetEphemeralDisk(v EphemeralDisk)`

SetEphemeralDisk sets EphemeralDisk field to given value.

### HasEphemeralDisk

`func (o *TaskGroup) HasEphemeralDisk() bool`

HasEphemeralDisk returns a boolean if a field has been set.

### GetMeta

`func (o *TaskGroup) GetMeta() map[string]string`

GetMeta returns the Meta field if non-nil, zero value otherwise.

### GetMetaOk

`func (o *TaskGroup) GetMetaOk() (*map[string]string, bool)`

GetMetaOk returns a tuple with the Meta field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMeta

`func (o *TaskGroup) SetMeta(v map[string]string)`

SetMeta sets Meta field to given value.

### HasMeta

`func (o *TaskGroup) HasMeta() bool`

HasMeta returns a boolean if a field has been set.

### GetMigrate

`func (o *TaskGroup) GetMigrate() MigrateStrategy`

GetMigrate returns the Migrate field if non-nil, zero value otherwise.

### GetMigrateOk

`func (o *TaskGroup) GetMigrateOk() (*MigrateStrategy, bool)`

GetMigrateOk returns a tuple with the Migrate field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMigrate

`func (o *TaskGroup) SetMigrate(v MigrateStrategy)`

SetMigrate sets Migrate field to given value.

### HasMigrate

`func (o *TaskGroup) HasMigrate() bool`

HasMigrate returns a boolean if a field has been set.

### GetName

`func (o *TaskGroup) GetName() string`

GetName returns the Name field if non-nil, zero value otherwise.

### GetNameOk

`func (o *TaskGroup) GetNameOk() (*string, bool)`

GetNameOk returns a tuple with the Name field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetName

`func (o *TaskGroup) SetName(v string)`

SetName sets Name field to given value.

### HasName

`func (o *TaskGroup) HasName() bool`

HasName returns a boolean if a field has been set.

### GetNetworks

`func (o *TaskGroup) GetNetworks() []NetworkResource`

GetNetworks returns the Networks field if non-nil, zero value otherwise.

### GetNetworksOk

`func (o *TaskGroup) GetNetworksOk() (*[]NetworkResource, bool)`

GetNetworksOk returns a tuple with the Networks field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNetworks

`func (o *TaskGroup) SetNetworks(v []NetworkResource)`

SetNetworks sets Networks field to given value.

### HasNetworks

`func (o *TaskGroup) HasNetworks() bool`

HasNetworks returns a boolean if a field has been set.

### GetReschedulePolicy

`func (o *TaskGroup) GetReschedulePolicy() ReschedulePolicy`

GetReschedulePolicy returns the ReschedulePolicy field if non-nil, zero value otherwise.

### GetReschedulePolicyOk

`func (o *TaskGroup) GetReschedulePolicyOk() (*ReschedulePolicy, bool)`

GetReschedulePolicyOk returns a tuple with the ReschedulePolicy field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetReschedulePolicy

`func (o *TaskGroup) SetReschedulePolicy(v ReschedulePolicy)`

SetReschedulePolicy sets ReschedulePolicy field to given value.

### HasReschedulePolicy

`func (o *TaskGroup) HasReschedulePolicy() bool`

HasReschedulePolicy returns a boolean if a field has been set.

### GetRestartPolicy

`func (o *TaskGroup) GetRestartPolicy() RestartPolicy`

GetRestartPolicy returns the RestartPolicy field if non-nil, zero value otherwise.

### GetRestartPolicyOk

`func (o *TaskGroup) GetRestartPolicyOk() (*RestartPolicy, bool)`

GetRestartPolicyOk returns a tuple with the RestartPolicy field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetRestartPolicy

`func (o *TaskGroup) SetRestartPolicy(v RestartPolicy)`

SetRestartPolicy sets RestartPolicy field to given value.

### HasRestartPolicy

`func (o *TaskGroup) HasRestartPolicy() bool`

HasRestartPolicy returns a boolean if a field has been set.

### GetScaling

`func (o *TaskGroup) GetScaling() ScalingPolicy`

GetScaling returns the Scaling field if non-nil, zero value otherwise.

### GetScalingOk

`func (o *TaskGroup) GetScalingOk() (*ScalingPolicy, bool)`

GetScalingOk returns a tuple with the Scaling field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetScaling

`func (o *TaskGroup) SetScaling(v ScalingPolicy)`

SetScaling sets Scaling field to given value.

### HasScaling

`func (o *TaskGroup) HasScaling() bool`

HasScaling returns a boolean if a field has been set.

### GetServices

`func (o *TaskGroup) GetServices() []Service`

GetServices returns the Services field if non-nil, zero value otherwise.

### GetServicesOk

`func (o *TaskGroup) GetServicesOk() (*[]Service, bool)`

GetServicesOk returns a tuple with the Services field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetServices

`func (o *TaskGroup) SetServices(v []Service)`

SetServices sets Services field to given value.

### HasServices

`func (o *TaskGroup) HasServices() bool`

HasServices returns a boolean if a field has been set.

### GetShutdownDelay

`func (o *TaskGroup) GetShutdownDelay() int64`

GetShutdownDelay returns the ShutdownDelay field if non-nil, zero value otherwise.

### GetShutdownDelayOk

`func (o *TaskGroup) GetShutdownDelayOk() (*int64, bool)`

GetShutdownDelayOk returns a tuple with the ShutdownDelay field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetShutdownDelay

`func (o *TaskGroup) SetShutdownDelay(v int64)`

SetShutdownDelay sets ShutdownDelay field to given value.

### HasShutdownDelay

`func (o *TaskGroup) HasShutdownDelay() bool`

HasShutdownDelay returns a boolean if a field has been set.

### GetSpreads

`func (o *TaskGroup) GetSpreads() []Spread`

GetSpreads returns the Spreads field if non-nil, zero value otherwise.

### GetSpreadsOk

`func (o *TaskGroup) GetSpreadsOk() (*[]Spread, bool)`

GetSpreadsOk returns a tuple with the Spreads field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSpreads

`func (o *TaskGroup) SetSpreads(v []Spread)`

SetSpreads sets Spreads field to given value.

### HasSpreads

`func (o *TaskGroup) HasSpreads() bool`

HasSpreads returns a boolean if a field has been set.

### GetStopAfterClientDisconnect

`func (o *TaskGroup) GetStopAfterClientDisconnect() int64`

GetStopAfterClientDisconnect returns the StopAfterClientDisconnect field if non-nil, zero value otherwise.

### GetStopAfterClientDisconnectOk

`func (o *TaskGroup) GetStopAfterClientDisconnectOk() (*int64, bool)`

GetStopAfterClientDisconnectOk returns a tuple with the StopAfterClientDisconnect field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetStopAfterClientDisconnect

`func (o *TaskGroup) SetStopAfterClientDisconnect(v int64)`

SetStopAfterClientDisconnect sets StopAfterClientDisconnect field to given value.

### HasStopAfterClientDisconnect

`func (o *TaskGroup) HasStopAfterClientDisconnect() bool`

HasStopAfterClientDisconnect returns a boolean if a field has been set.

### GetTasks

`func (o *TaskGroup) GetTasks() []Task`

GetTasks returns the Tasks field if non-nil, zero value otherwise.

### GetTasksOk

`func (o *TaskGroup) GetTasksOk() (*[]Task, bool)`

GetTasksOk returns a tuple with the Tasks field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTasks

`func (o *TaskGroup) SetTasks(v []Task)`

SetTasks sets Tasks field to given value.

### HasTasks

`func (o *TaskGroup) HasTasks() bool`

HasTasks returns a boolean if a field has been set.

### GetUpdate

`func (o *TaskGroup) GetUpdate() UpdateStrategy`

GetUpdate returns the Update field if non-nil, zero value otherwise.

### GetUpdateOk

`func (o *TaskGroup) GetUpdateOk() (*UpdateStrategy, bool)`

GetUpdateOk returns a tuple with the Update field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetUpdate

`func (o *TaskGroup) SetUpdate(v UpdateStrategy)`

SetUpdate sets Update field to given value.

### HasUpdate

`func (o *TaskGroup) HasUpdate() bool`

HasUpdate returns a boolean if a field has been set.

### GetVolumes

`func (o *TaskGroup) GetVolumes() map[string]VolumeRequest`

GetVolumes returns the Volumes field if non-nil, zero value otherwise.

### GetVolumesOk

`func (o *TaskGroup) GetVolumesOk() (*map[string]VolumeRequest, bool)`

GetVolumesOk returns a tuple with the Volumes field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetVolumes

`func (o *TaskGroup) SetVolumes(v map[string]VolumeRequest)`

SetVolumes sets Volumes field to given value.

### HasVolumes

`func (o *TaskGroup) HasVolumes() bool`

HasVolumes returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


