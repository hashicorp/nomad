# Task

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Affinities** | Pointer to [**[]Affinity**](Affinity.md) |  | [optional] 
**Artifacts** | Pointer to [**[]TaskArtifact**](TaskArtifact.md) |  | [optional] 
**CSIPluginConfig** | Pointer to [**TaskCSIPluginConfig**](TaskCSIPluginConfig.md) |  | [optional] 
**Config** | Pointer to **map[string]map[string]interface{}** |  | [optional] 
**Constraints** | Pointer to [**[]Constraint**](Constraint.md) |  | [optional] 
**DispatchPayload** | Pointer to [**DispatchPayloadConfig**](DispatchPayloadConfig.md) |  | [optional] 
**Driver** | Pointer to **string** |  | [optional] 
**Env** | Pointer to **map[string]string** |  | [optional] 
**KillSignal** | Pointer to **string** |  | [optional] 
**KillTimeout** | Pointer to **int64** | A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years. | [optional] 
**Kind** | Pointer to **string** |  | [optional] 
**Leader** | Pointer to **bool** |  | [optional] 
**Lifecycle** | Pointer to [**TaskLifecycle**](TaskLifecycle.md) |  | [optional] 
**LogConfig** | Pointer to [**LogConfig**](LogConfig.md) |  | [optional] 
**Meta** | Pointer to **map[string]string** |  | [optional] 
**Name** | Pointer to **string** |  | [optional] 
**Resources** | Pointer to [**Resources**](Resources.md) |  | [optional] 
**RestartPolicy** | Pointer to [**RestartPolicy**](RestartPolicy.md) |  | [optional] 
**ScalingPolicies** | Pointer to [**[]ScalingPolicy**](ScalingPolicy.md) |  | [optional] 
**Services** | Pointer to [**[]Service**](Service.md) |  | [optional] 
**ShutdownDelay** | Pointer to **int64** | A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years. | [optional] 
**Templates** | Pointer to [**[]Template**](Template.md) |  | [optional] 
**User** | Pointer to **string** |  | [optional] 
**Vault** | Pointer to [**Vault**](Vault.md) |  | [optional] 
**VolumeMounts** | Pointer to [**[]VolumeMount**](VolumeMount.md) |  | [optional] 

## Methods

### NewTask

`func NewTask() *Task`

NewTask instantiates a new Task object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewTaskWithDefaults

`func NewTaskWithDefaults() *Task`

NewTaskWithDefaults instantiates a new Task object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetAffinities

`func (o *Task) GetAffinities() []Affinity`

GetAffinities returns the Affinities field if non-nil, zero value otherwise.

### GetAffinitiesOk

`func (o *Task) GetAffinitiesOk() (*[]Affinity, bool)`

GetAffinitiesOk returns a tuple with the Affinities field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAffinities

`func (o *Task) SetAffinities(v []Affinity)`

SetAffinities sets Affinities field to given value.

### HasAffinities

`func (o *Task) HasAffinities() bool`

HasAffinities returns a boolean if a field has been set.

### GetArtifacts

`func (o *Task) GetArtifacts() []TaskArtifact`

GetArtifacts returns the Artifacts field if non-nil, zero value otherwise.

### GetArtifactsOk

`func (o *Task) GetArtifactsOk() (*[]TaskArtifact, bool)`

GetArtifactsOk returns a tuple with the Artifacts field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetArtifacts

`func (o *Task) SetArtifacts(v []TaskArtifact)`

SetArtifacts sets Artifacts field to given value.

### HasArtifacts

`func (o *Task) HasArtifacts() bool`

HasArtifacts returns a boolean if a field has been set.

### GetCSIPluginConfig

`func (o *Task) GetCSIPluginConfig() TaskCSIPluginConfig`

GetCSIPluginConfig returns the CSIPluginConfig field if non-nil, zero value otherwise.

### GetCSIPluginConfigOk

`func (o *Task) GetCSIPluginConfigOk() (*TaskCSIPluginConfig, bool)`

GetCSIPluginConfigOk returns a tuple with the CSIPluginConfig field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCSIPluginConfig

`func (o *Task) SetCSIPluginConfig(v TaskCSIPluginConfig)`

SetCSIPluginConfig sets CSIPluginConfig field to given value.

### HasCSIPluginConfig

`func (o *Task) HasCSIPluginConfig() bool`

HasCSIPluginConfig returns a boolean if a field has been set.

### GetConfig

`func (o *Task) GetConfig() map[string]map[string]interface{}`

GetConfig returns the Config field if non-nil, zero value otherwise.

### GetConfigOk

`func (o *Task) GetConfigOk() (*map[string]map[string]interface{}, bool)`

GetConfigOk returns a tuple with the Config field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetConfig

`func (o *Task) SetConfig(v map[string]map[string]interface{})`

SetConfig sets Config field to given value.

### HasConfig

`func (o *Task) HasConfig() bool`

HasConfig returns a boolean if a field has been set.

### GetConstraints

`func (o *Task) GetConstraints() []Constraint`

GetConstraints returns the Constraints field if non-nil, zero value otherwise.

### GetConstraintsOk

`func (o *Task) GetConstraintsOk() (*[]Constraint, bool)`

GetConstraintsOk returns a tuple with the Constraints field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetConstraints

`func (o *Task) SetConstraints(v []Constraint)`

SetConstraints sets Constraints field to given value.

### HasConstraints

`func (o *Task) HasConstraints() bool`

HasConstraints returns a boolean if a field has been set.

### GetDispatchPayload

`func (o *Task) GetDispatchPayload() DispatchPayloadConfig`

GetDispatchPayload returns the DispatchPayload field if non-nil, zero value otherwise.

### GetDispatchPayloadOk

`func (o *Task) GetDispatchPayloadOk() (*DispatchPayloadConfig, bool)`

GetDispatchPayloadOk returns a tuple with the DispatchPayload field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDispatchPayload

`func (o *Task) SetDispatchPayload(v DispatchPayloadConfig)`

SetDispatchPayload sets DispatchPayload field to given value.

### HasDispatchPayload

`func (o *Task) HasDispatchPayload() bool`

HasDispatchPayload returns a boolean if a field has been set.

### GetDriver

`func (o *Task) GetDriver() string`

GetDriver returns the Driver field if non-nil, zero value otherwise.

### GetDriverOk

`func (o *Task) GetDriverOk() (*string, bool)`

GetDriverOk returns a tuple with the Driver field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDriver

`func (o *Task) SetDriver(v string)`

SetDriver sets Driver field to given value.

### HasDriver

`func (o *Task) HasDriver() bool`

HasDriver returns a boolean if a field has been set.

### GetEnv

`func (o *Task) GetEnv() map[string]string`

GetEnv returns the Env field if non-nil, zero value otherwise.

### GetEnvOk

`func (o *Task) GetEnvOk() (*map[string]string, bool)`

GetEnvOk returns a tuple with the Env field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEnv

`func (o *Task) SetEnv(v map[string]string)`

SetEnv sets Env field to given value.

### HasEnv

`func (o *Task) HasEnv() bool`

HasEnv returns a boolean if a field has been set.

### GetKillSignal

`func (o *Task) GetKillSignal() string`

GetKillSignal returns the KillSignal field if non-nil, zero value otherwise.

### GetKillSignalOk

`func (o *Task) GetKillSignalOk() (*string, bool)`

GetKillSignalOk returns a tuple with the KillSignal field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetKillSignal

`func (o *Task) SetKillSignal(v string)`

SetKillSignal sets KillSignal field to given value.

### HasKillSignal

`func (o *Task) HasKillSignal() bool`

HasKillSignal returns a boolean if a field has been set.

### GetKillTimeout

`func (o *Task) GetKillTimeout() int64`

GetKillTimeout returns the KillTimeout field if non-nil, zero value otherwise.

### GetKillTimeoutOk

`func (o *Task) GetKillTimeoutOk() (*int64, bool)`

GetKillTimeoutOk returns a tuple with the KillTimeout field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetKillTimeout

`func (o *Task) SetKillTimeout(v int64)`

SetKillTimeout sets KillTimeout field to given value.

### HasKillTimeout

`func (o *Task) HasKillTimeout() bool`

HasKillTimeout returns a boolean if a field has been set.

### GetKind

`func (o *Task) GetKind() string`

GetKind returns the Kind field if non-nil, zero value otherwise.

### GetKindOk

`func (o *Task) GetKindOk() (*string, bool)`

GetKindOk returns a tuple with the Kind field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetKind

`func (o *Task) SetKind(v string)`

SetKind sets Kind field to given value.

### HasKind

`func (o *Task) HasKind() bool`

HasKind returns a boolean if a field has been set.

### GetLeader

`func (o *Task) GetLeader() bool`

GetLeader returns the Leader field if non-nil, zero value otherwise.

### GetLeaderOk

`func (o *Task) GetLeaderOk() (*bool, bool)`

GetLeaderOk returns a tuple with the Leader field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLeader

`func (o *Task) SetLeader(v bool)`

SetLeader sets Leader field to given value.

### HasLeader

`func (o *Task) HasLeader() bool`

HasLeader returns a boolean if a field has been set.

### GetLifecycle

`func (o *Task) GetLifecycle() TaskLifecycle`

GetLifecycle returns the Lifecycle field if non-nil, zero value otherwise.

### GetLifecycleOk

`func (o *Task) GetLifecycleOk() (*TaskLifecycle, bool)`

GetLifecycleOk returns a tuple with the Lifecycle field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLifecycle

`func (o *Task) SetLifecycle(v TaskLifecycle)`

SetLifecycle sets Lifecycle field to given value.

### HasLifecycle

`func (o *Task) HasLifecycle() bool`

HasLifecycle returns a boolean if a field has been set.

### GetLogConfig

`func (o *Task) GetLogConfig() LogConfig`

GetLogConfig returns the LogConfig field if non-nil, zero value otherwise.

### GetLogConfigOk

`func (o *Task) GetLogConfigOk() (*LogConfig, bool)`

GetLogConfigOk returns a tuple with the LogConfig field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLogConfig

`func (o *Task) SetLogConfig(v LogConfig)`

SetLogConfig sets LogConfig field to given value.

### HasLogConfig

`func (o *Task) HasLogConfig() bool`

HasLogConfig returns a boolean if a field has been set.

### GetMeta

`func (o *Task) GetMeta() map[string]string`

GetMeta returns the Meta field if non-nil, zero value otherwise.

### GetMetaOk

`func (o *Task) GetMetaOk() (*map[string]string, bool)`

GetMetaOk returns a tuple with the Meta field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMeta

`func (o *Task) SetMeta(v map[string]string)`

SetMeta sets Meta field to given value.

### HasMeta

`func (o *Task) HasMeta() bool`

HasMeta returns a boolean if a field has been set.

### GetName

`func (o *Task) GetName() string`

GetName returns the Name field if non-nil, zero value otherwise.

### GetNameOk

`func (o *Task) GetNameOk() (*string, bool)`

GetNameOk returns a tuple with the Name field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetName

`func (o *Task) SetName(v string)`

SetName sets Name field to given value.

### HasName

`func (o *Task) HasName() bool`

HasName returns a boolean if a field has been set.

### GetResources

`func (o *Task) GetResources() Resources`

GetResources returns the Resources field if non-nil, zero value otherwise.

### GetResourcesOk

`func (o *Task) GetResourcesOk() (*Resources, bool)`

GetResourcesOk returns a tuple with the Resources field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetResources

`func (o *Task) SetResources(v Resources)`

SetResources sets Resources field to given value.

### HasResources

`func (o *Task) HasResources() bool`

HasResources returns a boolean if a field has been set.

### GetRestartPolicy

`func (o *Task) GetRestartPolicy() RestartPolicy`

GetRestartPolicy returns the RestartPolicy field if non-nil, zero value otherwise.

### GetRestartPolicyOk

`func (o *Task) GetRestartPolicyOk() (*RestartPolicy, bool)`

GetRestartPolicyOk returns a tuple with the RestartPolicy field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetRestartPolicy

`func (o *Task) SetRestartPolicy(v RestartPolicy)`

SetRestartPolicy sets RestartPolicy field to given value.

### HasRestartPolicy

`func (o *Task) HasRestartPolicy() bool`

HasRestartPolicy returns a boolean if a field has been set.

### GetScalingPolicies

`func (o *Task) GetScalingPolicies() []ScalingPolicy`

GetScalingPolicies returns the ScalingPolicies field if non-nil, zero value otherwise.

### GetScalingPoliciesOk

`func (o *Task) GetScalingPoliciesOk() (*[]ScalingPolicy, bool)`

GetScalingPoliciesOk returns a tuple with the ScalingPolicies field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetScalingPolicies

`func (o *Task) SetScalingPolicies(v []ScalingPolicy)`

SetScalingPolicies sets ScalingPolicies field to given value.

### HasScalingPolicies

`func (o *Task) HasScalingPolicies() bool`

HasScalingPolicies returns a boolean if a field has been set.

### GetServices

`func (o *Task) GetServices() []Service`

GetServices returns the Services field if non-nil, zero value otherwise.

### GetServicesOk

`func (o *Task) GetServicesOk() (*[]Service, bool)`

GetServicesOk returns a tuple with the Services field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetServices

`func (o *Task) SetServices(v []Service)`

SetServices sets Services field to given value.

### HasServices

`func (o *Task) HasServices() bool`

HasServices returns a boolean if a field has been set.

### GetShutdownDelay

`func (o *Task) GetShutdownDelay() int64`

GetShutdownDelay returns the ShutdownDelay field if non-nil, zero value otherwise.

### GetShutdownDelayOk

`func (o *Task) GetShutdownDelayOk() (*int64, bool)`

GetShutdownDelayOk returns a tuple with the ShutdownDelay field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetShutdownDelay

`func (o *Task) SetShutdownDelay(v int64)`

SetShutdownDelay sets ShutdownDelay field to given value.

### HasShutdownDelay

`func (o *Task) HasShutdownDelay() bool`

HasShutdownDelay returns a boolean if a field has been set.

### GetTemplates

`func (o *Task) GetTemplates() []Template`

GetTemplates returns the Templates field if non-nil, zero value otherwise.

### GetTemplatesOk

`func (o *Task) GetTemplatesOk() (*[]Template, bool)`

GetTemplatesOk returns a tuple with the Templates field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTemplates

`func (o *Task) SetTemplates(v []Template)`

SetTemplates sets Templates field to given value.

### HasTemplates

`func (o *Task) HasTemplates() bool`

HasTemplates returns a boolean if a field has been set.

### GetUser

`func (o *Task) GetUser() string`

GetUser returns the User field if non-nil, zero value otherwise.

### GetUserOk

`func (o *Task) GetUserOk() (*string, bool)`

GetUserOk returns a tuple with the User field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetUser

`func (o *Task) SetUser(v string)`

SetUser sets User field to given value.

### HasUser

`func (o *Task) HasUser() bool`

HasUser returns a boolean if a field has been set.

### GetVault

`func (o *Task) GetVault() Vault`

GetVault returns the Vault field if non-nil, zero value otherwise.

### GetVaultOk

`func (o *Task) GetVaultOk() (*Vault, bool)`

GetVaultOk returns a tuple with the Vault field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetVault

`func (o *Task) SetVault(v Vault)`

SetVault sets Vault field to given value.

### HasVault

`func (o *Task) HasVault() bool`

HasVault returns a boolean if a field has been set.

### GetVolumeMounts

`func (o *Task) GetVolumeMounts() []VolumeMount`

GetVolumeMounts returns the VolumeMounts field if non-nil, zero value otherwise.

### GetVolumeMountsOk

`func (o *Task) GetVolumeMountsOk() (*[]VolumeMount, bool)`

GetVolumeMountsOk returns a tuple with the VolumeMounts field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetVolumeMounts

`func (o *Task) SetVolumeMounts(v []VolumeMount)`

SetVolumeMounts sets VolumeMounts field to given value.

### HasVolumeMounts

`func (o *Task) HasVolumeMounts() bool`

HasVolumeMounts returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


