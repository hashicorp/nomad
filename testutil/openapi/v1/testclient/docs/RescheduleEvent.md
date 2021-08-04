# RescheduleEvent

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**PrevAllocID** | Pointer to **string** |  | [optional] 
**PrevNodeID** | Pointer to **string** |  | [optional] 
**RescheduleTime** | Pointer to **int64** |  | [optional] 

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


