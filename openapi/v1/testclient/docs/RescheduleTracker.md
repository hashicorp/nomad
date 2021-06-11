# RescheduleTracker

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Events** | Pointer to [**[]RescheduleEvent**](RescheduleEvent.md) |  | [optional] 

## Methods

### NewRescheduleTracker

`func NewRescheduleTracker() *RescheduleTracker`

NewRescheduleTracker instantiates a new RescheduleTracker object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewRescheduleTrackerWithDefaults

`func NewRescheduleTrackerWithDefaults() *RescheduleTracker`

NewRescheduleTrackerWithDefaults instantiates a new RescheduleTracker object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetEvents

`func (o *RescheduleTracker) GetEvents() []RescheduleEvent`

GetEvents returns the Events field if non-nil, zero value otherwise.

### GetEventsOk

`func (o *RescheduleTracker) GetEventsOk() (*[]RescheduleEvent, bool)`

GetEventsOk returns a tuple with the Events field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEvents

`func (o *RescheduleTracker) SetEvents(v []RescheduleEvent)`

SetEvents sets Events field to given value.

### HasEvents

`func (o *RescheduleTracker) HasEvents() bool`

HasEvents returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


