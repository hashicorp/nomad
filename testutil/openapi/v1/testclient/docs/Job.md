# Job

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Affinities** | Pointer to [**[]Affinity**](Affinity.md) |  | [optional] 
**AllAtOnce** | Pointer to **bool** |  | [optional] 
**Constraints** | Pointer to [**[]Constraint**](Constraint.md) |  | [optional] 
**ConsulNamespace** | Pointer to **string** |  | [optional] 
**ConsulToken** | Pointer to **string** |  | [optional] 
**CreateIndex** | Pointer to **int32** |  | [optional] 
**Datacenters** | Pointer to **[]string** |  | [optional] 
**Dispatched** | Pointer to **bool** |  | [optional] 
**ID** | Pointer to **string** |  | [optional] 
**JobModifyIndex** | Pointer to **int32** |  | [optional] 
**Meta** | Pointer to **map[string]string** |  | [optional] 
**Migrate** | Pointer to [**MigrateStrategy**](MigrateStrategy.md) |  | [optional] 
**ModifyIndex** | Pointer to **int32** |  | [optional] 
**Multiregion** | Pointer to [**Multiregion**](Multiregion.md) |  | [optional] 
**Name** | Pointer to **string** |  | [optional] 
**Namespace** | Pointer to **string** |  | [optional] 
**NomadTokenID** | Pointer to **string** |  | [optional] 
**ParameterizedJob** | Pointer to [**ParameterizedJobConfig**](ParameterizedJobConfig.md) |  | [optional] 
**ParentID** | Pointer to **string** |  | [optional] 
**Payload** | Pointer to **string** |  | [optional] 
**Periodic** | Pointer to [**PeriodicConfig**](PeriodicConfig.md) |  | [optional] 
**Priority** | Pointer to **int32** |  | [optional] 
**Region** | Pointer to **string** |  | [optional] 
**Reschedule** | Pointer to [**ReschedulePolicy**](ReschedulePolicy.md) |  | [optional] 
**Spreads** | Pointer to [**[]Spread**](Spread.md) |  | [optional] 
**Stable** | Pointer to **bool** |  | [optional] 
**Status** | Pointer to **string** |  | [optional] 
**StatusDescription** | Pointer to **string** |  | [optional] 
**Stop** | Pointer to **bool** |  | [optional] 
**SubmitTime** | Pointer to **int64** |  | [optional] 
**TaskGroups** | Pointer to [**[]TaskGroup**](TaskGroup.md) |  | [optional] 
**Type** | Pointer to **string** |  | [optional] 
**Update** | Pointer to [**UpdateStrategy**](UpdateStrategy.md) |  | [optional] 
**VaultNamespace** | Pointer to **string** |  | [optional] 
**VaultToken** | Pointer to **string** |  | [optional] 
**Version** | Pointer to **int32** |  | [optional] 

## Methods

### NewJob

`func NewJob() *Job`

NewJob instantiates a new Job object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewJobWithDefaults

`func NewJobWithDefaults() *Job`

NewJobWithDefaults instantiates a new Job object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetAffinities

`func (o *Job) GetAffinities() []Affinity`

GetAffinities returns the Affinities field if non-nil, zero value otherwise.

### GetAffinitiesOk

`func (o *Job) GetAffinitiesOk() (*[]Affinity, bool)`

GetAffinitiesOk returns a tuple with the Affinities field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAffinities

`func (o *Job) SetAffinities(v []Affinity)`

SetAffinities sets Affinities field to given value.

### HasAffinities

`func (o *Job) HasAffinities() bool`

HasAffinities returns a boolean if a field has been set.

### GetAllAtOnce

`func (o *Job) GetAllAtOnce() bool`

GetAllAtOnce returns the AllAtOnce field if non-nil, zero value otherwise.

### GetAllAtOnceOk

`func (o *Job) GetAllAtOnceOk() (*bool, bool)`

GetAllAtOnceOk returns a tuple with the AllAtOnce field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAllAtOnce

`func (o *Job) SetAllAtOnce(v bool)`

SetAllAtOnce sets AllAtOnce field to given value.

### HasAllAtOnce

`func (o *Job) HasAllAtOnce() bool`

HasAllAtOnce returns a boolean if a field has been set.

### GetConstraints

`func (o *Job) GetConstraints() []Constraint`

GetConstraints returns the Constraints field if non-nil, zero value otherwise.

### GetConstraintsOk

`func (o *Job) GetConstraintsOk() (*[]Constraint, bool)`

GetConstraintsOk returns a tuple with the Constraints field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetConstraints

`func (o *Job) SetConstraints(v []Constraint)`

SetConstraints sets Constraints field to given value.

### HasConstraints

`func (o *Job) HasConstraints() bool`

HasConstraints returns a boolean if a field has been set.

### GetConsulNamespace

`func (o *Job) GetConsulNamespace() string`

GetConsulNamespace returns the ConsulNamespace field if non-nil, zero value otherwise.

### GetConsulNamespaceOk

`func (o *Job) GetConsulNamespaceOk() (*string, bool)`

GetConsulNamespaceOk returns a tuple with the ConsulNamespace field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetConsulNamespace

`func (o *Job) SetConsulNamespace(v string)`

SetConsulNamespace sets ConsulNamespace field to given value.

### HasConsulNamespace

`func (o *Job) HasConsulNamespace() bool`

HasConsulNamespace returns a boolean if a field has been set.

### GetConsulToken

`func (o *Job) GetConsulToken() string`

GetConsulToken returns the ConsulToken field if non-nil, zero value otherwise.

### GetConsulTokenOk

`func (o *Job) GetConsulTokenOk() (*string, bool)`

GetConsulTokenOk returns a tuple with the ConsulToken field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetConsulToken

`func (o *Job) SetConsulToken(v string)`

SetConsulToken sets ConsulToken field to given value.

### HasConsulToken

`func (o *Job) HasConsulToken() bool`

HasConsulToken returns a boolean if a field has been set.

### GetCreateIndex

`func (o *Job) GetCreateIndex() int32`

GetCreateIndex returns the CreateIndex field if non-nil, zero value otherwise.

### GetCreateIndexOk

`func (o *Job) GetCreateIndexOk() (*int32, bool)`

GetCreateIndexOk returns a tuple with the CreateIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCreateIndex

`func (o *Job) SetCreateIndex(v int32)`

SetCreateIndex sets CreateIndex field to given value.

### HasCreateIndex

`func (o *Job) HasCreateIndex() bool`

HasCreateIndex returns a boolean if a field has been set.

### GetDatacenters

`func (o *Job) GetDatacenters() []string`

GetDatacenters returns the Datacenters field if non-nil, zero value otherwise.

### GetDatacentersOk

`func (o *Job) GetDatacentersOk() (*[]string, bool)`

GetDatacentersOk returns a tuple with the Datacenters field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDatacenters

`func (o *Job) SetDatacenters(v []string)`

SetDatacenters sets Datacenters field to given value.

### HasDatacenters

`func (o *Job) HasDatacenters() bool`

HasDatacenters returns a boolean if a field has been set.

### GetDispatched

`func (o *Job) GetDispatched() bool`

GetDispatched returns the Dispatched field if non-nil, zero value otherwise.

### GetDispatchedOk

`func (o *Job) GetDispatchedOk() (*bool, bool)`

GetDispatchedOk returns a tuple with the Dispatched field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDispatched

`func (o *Job) SetDispatched(v bool)`

SetDispatched sets Dispatched field to given value.

### HasDispatched

`func (o *Job) HasDispatched() bool`

HasDispatched returns a boolean if a field has been set.

### GetID

`func (o *Job) GetID() string`

GetID returns the ID field if non-nil, zero value otherwise.

### GetIDOk

`func (o *Job) GetIDOk() (*string, bool)`

GetIDOk returns a tuple with the ID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetID

`func (o *Job) SetID(v string)`

SetID sets ID field to given value.

### HasID

`func (o *Job) HasID() bool`

HasID returns a boolean if a field has been set.

### GetJobModifyIndex

`func (o *Job) GetJobModifyIndex() int32`

GetJobModifyIndex returns the JobModifyIndex field if non-nil, zero value otherwise.

### GetJobModifyIndexOk

`func (o *Job) GetJobModifyIndexOk() (*int32, bool)`

GetJobModifyIndexOk returns a tuple with the JobModifyIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetJobModifyIndex

`func (o *Job) SetJobModifyIndex(v int32)`

SetJobModifyIndex sets JobModifyIndex field to given value.

### HasJobModifyIndex

`func (o *Job) HasJobModifyIndex() bool`

HasJobModifyIndex returns a boolean if a field has been set.

### GetMeta

`func (o *Job) GetMeta() map[string]string`

GetMeta returns the Meta field if non-nil, zero value otherwise.

### GetMetaOk

`func (o *Job) GetMetaOk() (*map[string]string, bool)`

GetMetaOk returns a tuple with the Meta field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMeta

`func (o *Job) SetMeta(v map[string]string)`

SetMeta sets Meta field to given value.

### HasMeta

`func (o *Job) HasMeta() bool`

HasMeta returns a boolean if a field has been set.

### GetMigrate

`func (o *Job) GetMigrate() MigrateStrategy`

GetMigrate returns the Migrate field if non-nil, zero value otherwise.

### GetMigrateOk

`func (o *Job) GetMigrateOk() (*MigrateStrategy, bool)`

GetMigrateOk returns a tuple with the Migrate field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMigrate

`func (o *Job) SetMigrate(v MigrateStrategy)`

SetMigrate sets Migrate field to given value.

### HasMigrate

`func (o *Job) HasMigrate() bool`

HasMigrate returns a boolean if a field has been set.

### GetModifyIndex

`func (o *Job) GetModifyIndex() int32`

GetModifyIndex returns the ModifyIndex field if non-nil, zero value otherwise.

### GetModifyIndexOk

`func (o *Job) GetModifyIndexOk() (*int32, bool)`

GetModifyIndexOk returns a tuple with the ModifyIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetModifyIndex

`func (o *Job) SetModifyIndex(v int32)`

SetModifyIndex sets ModifyIndex field to given value.

### HasModifyIndex

`func (o *Job) HasModifyIndex() bool`

HasModifyIndex returns a boolean if a field has been set.

### GetMultiregion

`func (o *Job) GetMultiregion() Multiregion`

GetMultiregion returns the Multiregion field if non-nil, zero value otherwise.

### GetMultiregionOk

`func (o *Job) GetMultiregionOk() (*Multiregion, bool)`

GetMultiregionOk returns a tuple with the Multiregion field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMultiregion

`func (o *Job) SetMultiregion(v Multiregion)`

SetMultiregion sets Multiregion field to given value.

### HasMultiregion

`func (o *Job) HasMultiregion() bool`

HasMultiregion returns a boolean if a field has been set.

### GetName

`func (o *Job) GetName() string`

GetName returns the Name field if non-nil, zero value otherwise.

### GetNameOk

`func (o *Job) GetNameOk() (*string, bool)`

GetNameOk returns a tuple with the Name field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetName

`func (o *Job) SetName(v string)`

SetName sets Name field to given value.

### HasName

`func (o *Job) HasName() bool`

HasName returns a boolean if a field has been set.

### GetNamespace

`func (o *Job) GetNamespace() string`

GetNamespace returns the Namespace field if non-nil, zero value otherwise.

### GetNamespaceOk

`func (o *Job) GetNamespaceOk() (*string, bool)`

GetNamespaceOk returns a tuple with the Namespace field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNamespace

`func (o *Job) SetNamespace(v string)`

SetNamespace sets Namespace field to given value.

### HasNamespace

`func (o *Job) HasNamespace() bool`

HasNamespace returns a boolean if a field has been set.

### GetNomadTokenID

`func (o *Job) GetNomadTokenID() string`

GetNomadTokenID returns the NomadTokenID field if non-nil, zero value otherwise.

### GetNomadTokenIDOk

`func (o *Job) GetNomadTokenIDOk() (*string, bool)`

GetNomadTokenIDOk returns a tuple with the NomadTokenID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNomadTokenID

`func (o *Job) SetNomadTokenID(v string)`

SetNomadTokenID sets NomadTokenID field to given value.

### HasNomadTokenID

`func (o *Job) HasNomadTokenID() bool`

HasNomadTokenID returns a boolean if a field has been set.

### GetParameterizedJob

`func (o *Job) GetParameterizedJob() ParameterizedJobConfig`

GetParameterizedJob returns the ParameterizedJob field if non-nil, zero value otherwise.

### GetParameterizedJobOk

`func (o *Job) GetParameterizedJobOk() (*ParameterizedJobConfig, bool)`

GetParameterizedJobOk returns a tuple with the ParameterizedJob field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetParameterizedJob

`func (o *Job) SetParameterizedJob(v ParameterizedJobConfig)`

SetParameterizedJob sets ParameterizedJob field to given value.

### HasParameterizedJob

`func (o *Job) HasParameterizedJob() bool`

HasParameterizedJob returns a boolean if a field has been set.

### GetParentID

`func (o *Job) GetParentID() string`

GetParentID returns the ParentID field if non-nil, zero value otherwise.

### GetParentIDOk

`func (o *Job) GetParentIDOk() (*string, bool)`

GetParentIDOk returns a tuple with the ParentID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetParentID

`func (o *Job) SetParentID(v string)`

SetParentID sets ParentID field to given value.

### HasParentID

`func (o *Job) HasParentID() bool`

HasParentID returns a boolean if a field has been set.

### GetPayload

`func (o *Job) GetPayload() string`

GetPayload returns the Payload field if non-nil, zero value otherwise.

### GetPayloadOk

`func (o *Job) GetPayloadOk() (*string, bool)`

GetPayloadOk returns a tuple with the Payload field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPayload

`func (o *Job) SetPayload(v string)`

SetPayload sets Payload field to given value.

### HasPayload

`func (o *Job) HasPayload() bool`

HasPayload returns a boolean if a field has been set.

### GetPeriodic

`func (o *Job) GetPeriodic() PeriodicConfig`

GetPeriodic returns the Periodic field if non-nil, zero value otherwise.

### GetPeriodicOk

`func (o *Job) GetPeriodicOk() (*PeriodicConfig, bool)`

GetPeriodicOk returns a tuple with the Periodic field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPeriodic

`func (o *Job) SetPeriodic(v PeriodicConfig)`

SetPeriodic sets Periodic field to given value.

### HasPeriodic

`func (o *Job) HasPeriodic() bool`

HasPeriodic returns a boolean if a field has been set.

### GetPriority

`func (o *Job) GetPriority() int32`

GetPriority returns the Priority field if non-nil, zero value otherwise.

### GetPriorityOk

`func (o *Job) GetPriorityOk() (*int32, bool)`

GetPriorityOk returns a tuple with the Priority field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPriority

`func (o *Job) SetPriority(v int32)`

SetPriority sets Priority field to given value.

### HasPriority

`func (o *Job) HasPriority() bool`

HasPriority returns a boolean if a field has been set.

### GetRegion

`func (o *Job) GetRegion() string`

GetRegion returns the Region field if non-nil, zero value otherwise.

### GetRegionOk

`func (o *Job) GetRegionOk() (*string, bool)`

GetRegionOk returns a tuple with the Region field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetRegion

`func (o *Job) SetRegion(v string)`

SetRegion sets Region field to given value.

### HasRegion

`func (o *Job) HasRegion() bool`

HasRegion returns a boolean if a field has been set.

### GetReschedule

`func (o *Job) GetReschedule() ReschedulePolicy`

GetReschedule returns the Reschedule field if non-nil, zero value otherwise.

### GetRescheduleOk

`func (o *Job) GetRescheduleOk() (*ReschedulePolicy, bool)`

GetRescheduleOk returns a tuple with the Reschedule field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetReschedule

`func (o *Job) SetReschedule(v ReschedulePolicy)`

SetReschedule sets Reschedule field to given value.

### HasReschedule

`func (o *Job) HasReschedule() bool`

HasReschedule returns a boolean if a field has been set.

### GetSpreads

`func (o *Job) GetSpreads() []Spread`

GetSpreads returns the Spreads field if non-nil, zero value otherwise.

### GetSpreadsOk

`func (o *Job) GetSpreadsOk() (*[]Spread, bool)`

GetSpreadsOk returns a tuple with the Spreads field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSpreads

`func (o *Job) SetSpreads(v []Spread)`

SetSpreads sets Spreads field to given value.

### HasSpreads

`func (o *Job) HasSpreads() bool`

HasSpreads returns a boolean if a field has been set.

### GetStable

`func (o *Job) GetStable() bool`

GetStable returns the Stable field if non-nil, zero value otherwise.

### GetStableOk

`func (o *Job) GetStableOk() (*bool, bool)`

GetStableOk returns a tuple with the Stable field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetStable

`func (o *Job) SetStable(v bool)`

SetStable sets Stable field to given value.

### HasStable

`func (o *Job) HasStable() bool`

HasStable returns a boolean if a field has been set.

### GetStatus

`func (o *Job) GetStatus() string`

GetStatus returns the Status field if non-nil, zero value otherwise.

### GetStatusOk

`func (o *Job) GetStatusOk() (*string, bool)`

GetStatusOk returns a tuple with the Status field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetStatus

`func (o *Job) SetStatus(v string)`

SetStatus sets Status field to given value.

### HasStatus

`func (o *Job) HasStatus() bool`

HasStatus returns a boolean if a field has been set.

### GetStatusDescription

`func (o *Job) GetStatusDescription() string`

GetStatusDescription returns the StatusDescription field if non-nil, zero value otherwise.

### GetStatusDescriptionOk

`func (o *Job) GetStatusDescriptionOk() (*string, bool)`

GetStatusDescriptionOk returns a tuple with the StatusDescription field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetStatusDescription

`func (o *Job) SetStatusDescription(v string)`

SetStatusDescription sets StatusDescription field to given value.

### HasStatusDescription

`func (o *Job) HasStatusDescription() bool`

HasStatusDescription returns a boolean if a field has been set.

### GetStop

`func (o *Job) GetStop() bool`

GetStop returns the Stop field if non-nil, zero value otherwise.

### GetStopOk

`func (o *Job) GetStopOk() (*bool, bool)`

GetStopOk returns a tuple with the Stop field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetStop

`func (o *Job) SetStop(v bool)`

SetStop sets Stop field to given value.

### HasStop

`func (o *Job) HasStop() bool`

HasStop returns a boolean if a field has been set.

### GetSubmitTime

`func (o *Job) GetSubmitTime() int64`

GetSubmitTime returns the SubmitTime field if non-nil, zero value otherwise.

### GetSubmitTimeOk

`func (o *Job) GetSubmitTimeOk() (*int64, bool)`

GetSubmitTimeOk returns a tuple with the SubmitTime field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSubmitTime

`func (o *Job) SetSubmitTime(v int64)`

SetSubmitTime sets SubmitTime field to given value.

### HasSubmitTime

`func (o *Job) HasSubmitTime() bool`

HasSubmitTime returns a boolean if a field has been set.

### GetTaskGroups

`func (o *Job) GetTaskGroups() []TaskGroup`

GetTaskGroups returns the TaskGroups field if non-nil, zero value otherwise.

### GetTaskGroupsOk

`func (o *Job) GetTaskGroupsOk() (*[]TaskGroup, bool)`

GetTaskGroupsOk returns a tuple with the TaskGroups field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTaskGroups

`func (o *Job) SetTaskGroups(v []TaskGroup)`

SetTaskGroups sets TaskGroups field to given value.

### HasTaskGroups

`func (o *Job) HasTaskGroups() bool`

HasTaskGroups returns a boolean if a field has been set.

### GetType

`func (o *Job) GetType() string`

GetType returns the Type field if non-nil, zero value otherwise.

### GetTypeOk

`func (o *Job) GetTypeOk() (*string, bool)`

GetTypeOk returns a tuple with the Type field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetType

`func (o *Job) SetType(v string)`

SetType sets Type field to given value.

### HasType

`func (o *Job) HasType() bool`

HasType returns a boolean if a field has been set.

### GetUpdate

`func (o *Job) GetUpdate() UpdateStrategy`

GetUpdate returns the Update field if non-nil, zero value otherwise.

### GetUpdateOk

`func (o *Job) GetUpdateOk() (*UpdateStrategy, bool)`

GetUpdateOk returns a tuple with the Update field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetUpdate

`func (o *Job) SetUpdate(v UpdateStrategy)`

SetUpdate sets Update field to given value.

### HasUpdate

`func (o *Job) HasUpdate() bool`

HasUpdate returns a boolean if a field has been set.

### GetVaultNamespace

`func (o *Job) GetVaultNamespace() string`

GetVaultNamespace returns the VaultNamespace field if non-nil, zero value otherwise.

### GetVaultNamespaceOk

`func (o *Job) GetVaultNamespaceOk() (*string, bool)`

GetVaultNamespaceOk returns a tuple with the VaultNamespace field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetVaultNamespace

`func (o *Job) SetVaultNamespace(v string)`

SetVaultNamespace sets VaultNamespace field to given value.

### HasVaultNamespace

`func (o *Job) HasVaultNamespace() bool`

HasVaultNamespace returns a boolean if a field has been set.

### GetVaultToken

`func (o *Job) GetVaultToken() string`

GetVaultToken returns the VaultToken field if non-nil, zero value otherwise.

### GetVaultTokenOk

`func (o *Job) GetVaultTokenOk() (*string, bool)`

GetVaultTokenOk returns a tuple with the VaultToken field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetVaultToken

`func (o *Job) SetVaultToken(v string)`

SetVaultToken sets VaultToken field to given value.

### HasVaultToken

`func (o *Job) HasVaultToken() bool`

HasVaultToken returns a boolean if a field has been set.

### GetVersion

`func (o *Job) GetVersion() int32`

GetVersion returns the Version field if non-nil, zero value otherwise.

### GetVersionOk

`func (o *Job) GetVersionOk() (*int32, bool)`

GetVersionOk returns a tuple with the Version field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetVersion

`func (o *Job) SetVersion(v int32)`

SetVersion sets Version field to given value.

### HasVersion

`func (o *Job) HasVersion() bool`

HasVersion returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


