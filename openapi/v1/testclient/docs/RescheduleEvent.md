# RescheduleEvent

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Delay** | Pointer to **int64** | A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years. | [optional] 
**PrevAllocID** | Pointer to **string** | PrevAllocID is the ID of the previous allocation being restarted | [optional] 
**PrevNodeID** | Pointer to **string** | PrevNodeID is the node ID of the previous allocation | [optional] 
**RescheduleTime** | Pointer to **int64** | RescheduleTime is the timestamp of a reschedule attempt | [optional] 

## Methods

### NewRescheduleEvent

`func NewRescheduleEvent() *RescheduleEvent`

NewRescheduleEvent instantiates a new RescheduleEvent object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewRescheduleEventWithDefaults

`func NewRescheduleEventWithDefaults() *RescheduleEvent`

NewRescheduleEventWithDefaults instantiates a new RescheduleEvent object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetDelay

`func (o *RescheduleEvent) GetDelay() int64`

GetDelay returns the Delay field if non-nil, zero value otherwise.

### GetDelayOk

`func (o *RescheduleEvent) GetDelayOk() (*int64, bool)`

GetDelayOk returns a tuple with the Delay field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDelay

`func (o *RescheduleEvent) SetDelay(v int64)`

SetDelay sets Delay field to given value.

### HasDelay

`func (o *RescheduleEvent) HasDelay() bool`

HasDelay returns a boolean if a field has been set.

### GetPrevAllocID

`func (o *RescheduleEvent) GetPrevAllocID() string`

GetPrevAllocID returns the PrevAllocID field if non-nil, zero value otherwise.

### GetPrevAllocIDOk

`func (o *RescheduleEvent) GetPrevAllocIDOk() (*string, bool)`

GetPrevAllocIDOk returns a tuple with the PrevAllocID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPrevAllocID

`func (o *RescheduleEvent) SetPrevAllocID(v string)`

SetPrevAllocID sets PrevAllocID field to given value.

### HasPrevAllocID

`func (o *RescheduleEvent) HasPrevAllocID() bool`

HasPrevAllocID returns a boolean if a field has been set.

### GetPrevNodeID

`func (o *RescheduleEvent) GetPrevNodeID() string`

GetPrevNodeID returns the PrevNodeID field if non-nil, zero value otherwise.

### GetPrevNodeIDOk

`func (o *RescheduleEvent) GetPrevNodeIDOk() (*string, bool)`

GetPrevNodeIDOk returns a tuple with the PrevNodeID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPrevNodeID

`func (o *RescheduleEvent) SetPrevNodeID(v string)`

SetPrevNodeID sets PrevNodeID field to given value.

### HasPrevNodeID

`func (o *RescheduleEvent) HasPrevNodeID() bool`

HasPrevNodeID returns a boolean if a field has been set.

### GetRescheduleTime

`func (o *RescheduleEvent) GetRescheduleTime() int64`

GetRescheduleTime returns the RescheduleTime field if non-nil, zero value otherwise.

### GetRescheduleTimeOk

`func (o *RescheduleEvent) GetRescheduleTimeOk() (*int64, bool)`

GetRescheduleTimeOk returns a tuple with the RescheduleTime field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetRescheduleTime

`func (o *RescheduleEvent) SetRescheduleTime(v int64)`

SetRescheduleTime sets RescheduleTime field to given value.

### HasRescheduleTime

`func (o *RescheduleEvent) HasRescheduleTime() bool`

HasRescheduleTime returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


