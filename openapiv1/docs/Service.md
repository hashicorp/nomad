# Service

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**AddressMode** | Pointer to **string** |  | [optional] 
**CanaryMeta** | Pointer to **map[string]string** |  | [optional] 
**CanaryTags** | Pointer to **[]string** |  | [optional] 
**CheckRestart** | Pointer to [**CheckRestart**](CheckRestart.md) |  | [optional] 
**Checks** | Pointer to [**[]ServiceCheck**](ServiceCheck.md) |  | [optional] 
**Connect** | Pointer to [**ConsulConnect**](ConsulConnect.md) |  | [optional] 
**EnableTagOverride** | Pointer to **bool** |  | [optional] 
**Id** | Pointer to **string** | FIXME Id is unused. Remove? | [optional] 
**Meta** | Pointer to **map[string]string** |  | [optional] 
**Name** | Pointer to **string** |  | [optional] 
**OnUpdate** | Pointer to **string** |  | [optional] 
**PortLabel** | Pointer to **string** |  | [optional] 
**Tags** | Pointer to **[]string** |  | [optional] 
**TaskName** | Pointer to **string** |  | [optional] 

## Methods

### NewService

`func NewService() *Service`

NewService instantiates a new Service object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewServiceWithDefaults

`func NewServiceWithDefaults() *Service`

NewServiceWithDefaults instantiates a new Service object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetAddressMode

`func (o *Service) GetAddressMode() string`

GetAddressMode returns the AddressMode field if non-nil, zero value otherwise.

### GetAddressModeOk

`func (o *Service) GetAddressModeOk() (*string, bool)`

GetAddressModeOk returns a tuple with the AddressMode field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAddressMode

`func (o *Service) SetAddressMode(v string)`

SetAddressMode sets AddressMode field to given value.

### HasAddressMode

`func (o *Service) HasAddressMode() bool`

HasAddressMode returns a boolean if a field has been set.

### GetCanaryMeta

`func (o *Service) GetCanaryMeta() map[string]string`

GetCanaryMeta returns the CanaryMeta field if non-nil, zero value otherwise.

### GetCanaryMetaOk

`func (o *Service) GetCanaryMetaOk() (*map[string]string, bool)`

GetCanaryMetaOk returns a tuple with the CanaryMeta field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCanaryMeta

`func (o *Service) SetCanaryMeta(v map[string]string)`

SetCanaryMeta sets CanaryMeta field to given value.

### HasCanaryMeta

`func (o *Service) HasCanaryMeta() bool`

HasCanaryMeta returns a boolean if a field has been set.

### GetCanaryTags

`func (o *Service) GetCanaryTags() []string`

GetCanaryTags returns the CanaryTags field if non-nil, zero value otherwise.

### GetCanaryTagsOk

`func (o *Service) GetCanaryTagsOk() (*[]string, bool)`

GetCanaryTagsOk returns a tuple with the CanaryTags field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCanaryTags

`func (o *Service) SetCanaryTags(v []string)`

SetCanaryTags sets CanaryTags field to given value.

### HasCanaryTags

`func (o *Service) HasCanaryTags() bool`

HasCanaryTags returns a boolean if a field has been set.

### GetCheckRestart

`func (o *Service) GetCheckRestart() CheckRestart`

GetCheckRestart returns the CheckRestart field if non-nil, zero value otherwise.

### GetCheckRestartOk

`func (o *Service) GetCheckRestartOk() (*CheckRestart, bool)`

GetCheckRestartOk returns a tuple with the CheckRestart field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCheckRestart

`func (o *Service) SetCheckRestart(v CheckRestart)`

SetCheckRestart sets CheckRestart field to given value.

### HasCheckRestart

`func (o *Service) HasCheckRestart() bool`

HasCheckRestart returns a boolean if a field has been set.

### GetChecks

`func (o *Service) GetChecks() []ServiceCheck`

GetChecks returns the Checks field if non-nil, zero value otherwise.

### GetChecksOk

`func (o *Service) GetChecksOk() (*[]ServiceCheck, bool)`

GetChecksOk returns a tuple with the Checks field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetChecks

`func (o *Service) SetChecks(v []ServiceCheck)`

SetChecks sets Checks field to given value.

### HasChecks

`func (o *Service) HasChecks() bool`

HasChecks returns a boolean if a field has been set.

### GetConnect

`func (o *Service) GetConnect() ConsulConnect`

GetConnect returns the Connect field if non-nil, zero value otherwise.

### GetConnectOk

`func (o *Service) GetConnectOk() (*ConsulConnect, bool)`

GetConnectOk returns a tuple with the Connect field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetConnect

`func (o *Service) SetConnect(v ConsulConnect)`

SetConnect sets Connect field to given value.

### HasConnect

`func (o *Service) HasConnect() bool`

HasConnect returns a boolean if a field has been set.

### GetEnableTagOverride

`func (o *Service) GetEnableTagOverride() bool`

GetEnableTagOverride returns the EnableTagOverride field if non-nil, zero value otherwise.

### GetEnableTagOverrideOk

`func (o *Service) GetEnableTagOverrideOk() (*bool, bool)`

GetEnableTagOverrideOk returns a tuple with the EnableTagOverride field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEnableTagOverride

`func (o *Service) SetEnableTagOverride(v bool)`

SetEnableTagOverride sets EnableTagOverride field to given value.

### HasEnableTagOverride

`func (o *Service) HasEnableTagOverride() bool`

HasEnableTagOverride returns a boolean if a field has been set.

### GetId

`func (o *Service) GetId() string`

GetId returns the Id field if non-nil, zero value otherwise.

### GetIdOk

`func (o *Service) GetIdOk() (*string, bool)`

GetIdOk returns a tuple with the Id field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetId

`func (o *Service) SetId(v string)`

SetId sets Id field to given value.

### HasId

`func (o *Service) HasId() bool`

HasId returns a boolean if a field has been set.

### GetMeta

`func (o *Service) GetMeta() map[string]string`

GetMeta returns the Meta field if non-nil, zero value otherwise.

### GetMetaOk

`func (o *Service) GetMetaOk() (*map[string]string, bool)`

GetMetaOk returns a tuple with the Meta field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMeta

`func (o *Service) SetMeta(v map[string]string)`

SetMeta sets Meta field to given value.

### HasMeta

`func (o *Service) HasMeta() bool`

HasMeta returns a boolean if a field has been set.

### GetName

`func (o *Service) GetName() string`

GetName returns the Name field if non-nil, zero value otherwise.

### GetNameOk

`func (o *Service) GetNameOk() (*string, bool)`

GetNameOk returns a tuple with the Name field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetName

`func (o *Service) SetName(v string)`

SetName sets Name field to given value.

### HasName

`func (o *Service) HasName() bool`

HasName returns a boolean if a field has been set.

### GetOnUpdate

`func (o *Service) GetOnUpdate() string`

GetOnUpdate returns the OnUpdate field if non-nil, zero value otherwise.

### GetOnUpdateOk

`func (o *Service) GetOnUpdateOk() (*string, bool)`

GetOnUpdateOk returns a tuple with the OnUpdate field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetOnUpdate

`func (o *Service) SetOnUpdate(v string)`

SetOnUpdate sets OnUpdate field to given value.

### HasOnUpdate

`func (o *Service) HasOnUpdate() bool`

HasOnUpdate returns a boolean if a field has been set.

### GetPortLabel

`func (o *Service) GetPortLabel() string`

GetPortLabel returns the PortLabel field if non-nil, zero value otherwise.

### GetPortLabelOk

`func (o *Service) GetPortLabelOk() (*string, bool)`

GetPortLabelOk returns a tuple with the PortLabel field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPortLabel

`func (o *Service) SetPortLabel(v string)`

SetPortLabel sets PortLabel field to given value.

### HasPortLabel

`func (o *Service) HasPortLabel() bool`

HasPortLabel returns a boolean if a field has been set.

### GetTags

`func (o *Service) GetTags() []string`

GetTags returns the Tags field if non-nil, zero value otherwise.

### GetTagsOk

`func (o *Service) GetTagsOk() (*[]string, bool)`

GetTagsOk returns a tuple with the Tags field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTags

`func (o *Service) SetTags(v []string)`

SetTags sets Tags field to given value.

### HasTags

`func (o *Service) HasTags() bool`

HasTags returns a boolean if a field has been set.

### GetTaskName

`func (o *Service) GetTaskName() string`

GetTaskName returns the TaskName field if non-nil, zero value otherwise.

### GetTaskNameOk

`func (o *Service) GetTaskNameOk() (*string, bool)`

GetTaskNameOk returns a tuple with the TaskName field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTaskName

`func (o *Service) SetTaskName(v string)`

SetTaskName sets TaskName field to given value.

### HasTaskName

`func (o *Service) HasTaskName() bool`

HasTaskName returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


