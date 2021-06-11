# DesiredTransition

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**ForceReschedule** | Pointer to **bool** | ForceReschedule is used to indicate that this allocation must be rescheduled. This field is only used when operators want to force a placement even if a failed allocation is not eligible to be rescheduled | [optional] 
**Migrate** | Pointer to **bool** | Migrate is used to indicate that this allocation should be stopped and migrated to another node. | [optional] 
**Reschedule** | Pointer to **bool** | Reschedule is used to indicate that this allocation is eligible to be rescheduled. Most allocations are automatically eligible for rescheduling, so this field is only required when an allocation is not automatically eligible. An example is an allocation that is part of a deployment. | [optional] 

## Methods

### NewDesiredTransition

`func NewDesiredTransition() *DesiredTransition`

NewDesiredTransition instantiates a new DesiredTransition object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewDesiredTransitionWithDefaults

`func NewDesiredTransitionWithDefaults() *DesiredTransition`

NewDesiredTransitionWithDefaults instantiates a new DesiredTransition object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetForceReschedule

`func (o *DesiredTransition) GetForceReschedule() bool`

GetForceReschedule returns the ForceReschedule field if non-nil, zero value otherwise.

### GetForceRescheduleOk

`func (o *DesiredTransition) GetForceRescheduleOk() (*bool, bool)`

GetForceRescheduleOk returns a tuple with the ForceReschedule field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetForceReschedule

`func (o *DesiredTransition) SetForceReschedule(v bool)`

SetForceReschedule sets ForceReschedule field to given value.

### HasForceReschedule

`func (o *DesiredTransition) HasForceReschedule() bool`

HasForceReschedule returns a boolean if a field has been set.

### GetMigrate

`func (o *DesiredTransition) GetMigrate() bool`

GetMigrate returns the Migrate field if non-nil, zero value otherwise.

### GetMigrateOk

`func (o *DesiredTransition) GetMigrateOk() (*bool, bool)`

GetMigrateOk returns a tuple with the Migrate field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMigrate

`func (o *DesiredTransition) SetMigrate(v bool)`

SetMigrate sets Migrate field to given value.

### HasMigrate

`func (o *DesiredTransition) HasMigrate() bool`

HasMigrate returns a boolean if a field has been set.

### GetReschedule

`func (o *DesiredTransition) GetReschedule() bool`

GetReschedule returns the Reschedule field if non-nil, zero value otherwise.

### GetRescheduleOk

`func (o *DesiredTransition) GetRescheduleOk() (*bool, bool)`

GetRescheduleOk returns a tuple with the Reschedule field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetReschedule

`func (o *DesiredTransition) SetReschedule(v bool)`

SetReschedule sets Reschedule field to given value.

### HasReschedule

`func (o *DesiredTransition) HasReschedule() bool`

HasReschedule returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


