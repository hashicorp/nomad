# SidecarTask

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Config** | Pointer to **map[string]interface{}** |  | [optional] 
**Driver** | Pointer to **string** |  | [optional] 
**Env** | Pointer to **map[string]string** |  | [optional] 
**KillSignal** | Pointer to **string** |  | [optional] 
**KillTimeout** | Pointer to **int64** |  | [optional] 
**LogConfig** | Pointer to [**LogConfig**](LogConfig.md) |  | [optional] 
**Meta** | Pointer to **map[string]string** |  | [optional] 
**Name** | Pointer to **string** |  | [optional] 
**Resources** | Pointer to [**Resources**](Resources.md) |  | [optional] 
**ShutdownDelay** | Pointer to **int64** |  | [optional] 
**User** | Pointer to **string** |  | [optional] 

## Methods

### NewSidecarTask

`func NewSidecarTask() *SidecarTask`

NewSidecarTask instantiates a new SidecarTask object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewSidecarTaskWithDefaults

`func NewSidecarTaskWithDefaults() *SidecarTask`

NewSidecarTaskWithDefaults instantiates a new SidecarTask object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetConfig

`func (o *SidecarTask) GetConfig() map[string]interface{}`

GetConfig returns the Config field if non-nil, zero value otherwise.

### GetConfigOk

`func (o *SidecarTask) GetConfigOk() (*map[string]interface{}, bool)`

GetConfigOk returns a tuple with the Config field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetConfig

`func (o *SidecarTask) SetConfig(v map[string]interface{})`

SetConfig sets Config field to given value.

### HasConfig

`func (o *SidecarTask) HasConfig() bool`

HasConfig returns a boolean if a field has been set.

### GetDriver

`func (o *SidecarTask) GetDriver() string`

GetDriver returns the Driver field if non-nil, zero value otherwise.

### GetDriverOk

`func (o *SidecarTask) GetDriverOk() (*string, bool)`

GetDriverOk returns a tuple with the Driver field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDriver

`func (o *SidecarTask) SetDriver(v string)`

SetDriver sets Driver field to given value.

### HasDriver

`func (o *SidecarTask) HasDriver() bool`

HasDriver returns a boolean if a field has been set.

### GetEnv

`func (o *SidecarTask) GetEnv() map[string]string`

GetEnv returns the Env field if non-nil, zero value otherwise.

### GetEnvOk

`func (o *SidecarTask) GetEnvOk() (*map[string]string, bool)`

GetEnvOk returns a tuple with the Env field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEnv

`func (o *SidecarTask) SetEnv(v map[string]string)`

SetEnv sets Env field to given value.

### HasEnv

`func (o *SidecarTask) HasEnv() bool`

HasEnv returns a boolean if a field has been set.

### GetKillSignal

`func (o *SidecarTask) GetKillSignal() string`

GetKillSignal returns the KillSignal field if non-nil, zero value otherwise.

### GetKillSignalOk

`func (o *SidecarTask) GetKillSignalOk() (*string, bool)`

GetKillSignalOk returns a tuple with the KillSignal field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetKillSignal

`func (o *SidecarTask) SetKillSignal(v string)`

SetKillSignal sets KillSignal field to given value.

### HasKillSignal

`func (o *SidecarTask) HasKillSignal() bool`

HasKillSignal returns a boolean if a field has been set.

### GetKillTimeout

`func (o *SidecarTask) GetKillTimeout() int64`

GetKillTimeout returns the KillTimeout field if non-nil, zero value otherwise.

### GetKillTimeoutOk

`func (o *SidecarTask) GetKillTimeoutOk() (*int64, bool)`

GetKillTimeoutOk returns a tuple with the KillTimeout field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetKillTimeout

`func (o *SidecarTask) SetKillTimeout(v int64)`

SetKillTimeout sets KillTimeout field to given value.

### HasKillTimeout

`func (o *SidecarTask) HasKillTimeout() bool`

HasKillTimeout returns a boolean if a field has been set.

### GetLogConfig

`func (o *SidecarTask) GetLogConfig() LogConfig`

GetLogConfig returns the LogConfig field if non-nil, zero value otherwise.

### GetLogConfigOk

`func (o *SidecarTask) GetLogConfigOk() (*LogConfig, bool)`

GetLogConfigOk returns a tuple with the LogConfig field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLogConfig

`func (o *SidecarTask) SetLogConfig(v LogConfig)`

SetLogConfig sets LogConfig field to given value.

### HasLogConfig

`func (o *SidecarTask) HasLogConfig() bool`

HasLogConfig returns a boolean if a field has been set.

### GetMeta

`func (o *SidecarTask) GetMeta() map[string]string`

GetMeta returns the Meta field if non-nil, zero value otherwise.

### GetMetaOk

`func (o *SidecarTask) GetMetaOk() (*map[string]string, bool)`

GetMetaOk returns a tuple with the Meta field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMeta

`func (o *SidecarTask) SetMeta(v map[string]string)`

SetMeta sets Meta field to given value.

### HasMeta

`func (o *SidecarTask) HasMeta() bool`

HasMeta returns a boolean if a field has been set.

### GetName

`func (o *SidecarTask) GetName() string`

GetName returns the Name field if non-nil, zero value otherwise.

### GetNameOk

`func (o *SidecarTask) GetNameOk() (*string, bool)`

GetNameOk returns a tuple with the Name field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetName

`func (o *SidecarTask) SetName(v string)`

SetName sets Name field to given value.

### HasName

`func (o *SidecarTask) HasName() bool`

HasName returns a boolean if a field has been set.

### GetResources

`func (o *SidecarTask) GetResources() Resources`

GetResources returns the Resources field if non-nil, zero value otherwise.

### GetResourcesOk

`func (o *SidecarTask) GetResourcesOk() (*Resources, bool)`

GetResourcesOk returns a tuple with the Resources field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetResources

`func (o *SidecarTask) SetResources(v Resources)`

SetResources sets Resources field to given value.

### HasResources

`func (o *SidecarTask) HasResources() bool`

HasResources returns a boolean if a field has been set.

### GetShutdownDelay

`func (o *SidecarTask) GetShutdownDelay() int64`

GetShutdownDelay returns the ShutdownDelay field if non-nil, zero value otherwise.

### GetShutdownDelayOk

`func (o *SidecarTask) GetShutdownDelayOk() (*int64, bool)`

GetShutdownDelayOk returns a tuple with the ShutdownDelay field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetShutdownDelay

`func (o *SidecarTask) SetShutdownDelay(v int64)`

SetShutdownDelay sets ShutdownDelay field to given value.

### HasShutdownDelay

`func (o *SidecarTask) HasShutdownDelay() bool`

HasShutdownDelay returns a boolean if a field has been set.

### GetUser

`func (o *SidecarTask) GetUser() string`

GetUser returns the User field if non-nil, zero value otherwise.

### GetUserOk

`func (o *SidecarTask) GetUserOk() (*string, bool)`

GetUserOk returns a tuple with the User field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetUser

`func (o *SidecarTask) SetUser(v string)`

SetUser sets User field to given value.

### HasUser

`func (o *SidecarTask) HasUser() bool`

HasUser returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


