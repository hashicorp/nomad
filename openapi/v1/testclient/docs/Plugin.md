# Plugin

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Config** | [**PluginConfig**](PluginConfig.md) |  | 
**Enabled** | **bool** | True if the plugin is running. False if the plugin is not running, only installed. | 
**Id** | Pointer to **string** | Id | [optional] 
**Name** | **string** | name | 
**PluginReference** | Pointer to **string** | plugin remote reference used to push/pull the plugin | [optional] 
**Settings** | [**PluginSettings**](PluginSettings.md) |  | 

## Methods

### NewPlugin

`func NewPlugin(config PluginConfig, enabled bool, name string, settings PluginSettings, ) *Plugin`

NewPlugin instantiates a new Plugin object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewPluginWithDefaults

`func NewPluginWithDefaults() *Plugin`

NewPluginWithDefaults instantiates a new Plugin object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetConfig

`func (o *Plugin) GetConfig() PluginConfig`

GetConfig returns the Config field if non-nil, zero value otherwise.

### GetConfigOk

`func (o *Plugin) GetConfigOk() (*PluginConfig, bool)`

GetConfigOk returns a tuple with the Config field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetConfig

`func (o *Plugin) SetConfig(v PluginConfig)`

SetConfig sets Config field to given value.


### GetEnabled

`func (o *Plugin) GetEnabled() bool`

GetEnabled returns the Enabled field if non-nil, zero value otherwise.

### GetEnabledOk

`func (o *Plugin) GetEnabledOk() (*bool, bool)`

GetEnabledOk returns a tuple with the Enabled field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEnabled

`func (o *Plugin) SetEnabled(v bool)`

SetEnabled sets Enabled field to given value.


### GetId

`func (o *Plugin) GetId() string`

GetId returns the Id field if non-nil, zero value otherwise.

### GetIdOk

`func (o *Plugin) GetIdOk() (*string, bool)`

GetIdOk returns a tuple with the Id field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetId

`func (o *Plugin) SetId(v string)`

SetId sets Id field to given value.

### HasId

`func (o *Plugin) HasId() bool`

HasId returns a boolean if a field has been set.

### GetName

`func (o *Plugin) GetName() string`

GetName returns the Name field if non-nil, zero value otherwise.

### GetNameOk

`func (o *Plugin) GetNameOk() (*string, bool)`

GetNameOk returns a tuple with the Name field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetName

`func (o *Plugin) SetName(v string)`

SetName sets Name field to given value.


### GetPluginReference

`func (o *Plugin) GetPluginReference() string`

GetPluginReference returns the PluginReference field if non-nil, zero value otherwise.

### GetPluginReferenceOk

`func (o *Plugin) GetPluginReferenceOk() (*string, bool)`

GetPluginReferenceOk returns a tuple with the PluginReference field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPluginReference

`func (o *Plugin) SetPluginReference(v string)`

SetPluginReference sets PluginReference field to given value.

### HasPluginReference

`func (o *Plugin) HasPluginReference() bool`

HasPluginReference returns a boolean if a field has been set.

### GetSettings

`func (o *Plugin) GetSettings() PluginSettings`

GetSettings returns the Settings field if non-nil, zero value otherwise.

### GetSettingsOk

`func (o *Plugin) GetSettingsOk() (*PluginSettings, bool)`

GetSettingsOk returns a tuple with the Settings field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSettings

`func (o *Plugin) SetSettings(v PluginSettings)`

SetSettings sets Settings field to given value.



[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


