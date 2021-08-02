# RequestedDevice

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Affinities** | Pointer to [**[]Affinity**](Affinity.md) |  | [optional] 
**Constraints** | Pointer to [**[]Constraint**](Constraint.md) |  | [optional] 
**Count** | Pointer to **int32** |  | [optional] 
**Name** | Pointer to **string** |  | [optional] 

## Methods

### NewRequestedDevice

`func NewRequestedDevice() *RequestedDevice`

NewRequestedDevice instantiates a new RequestedDevice object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewRequestedDeviceWithDefaults

`func NewRequestedDeviceWithDefaults() *RequestedDevice`

NewRequestedDeviceWithDefaults instantiates a new RequestedDevice object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetAffinities

`func (o *RequestedDevice) GetAffinities() []Affinity`

GetAffinities returns the Affinities field if non-nil, zero value otherwise.

### GetAffinitiesOk

`func (o *RequestedDevice) GetAffinitiesOk() (*[]Affinity, bool)`

GetAffinitiesOk returns a tuple with the Affinities field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAffinities

`func (o *RequestedDevice) SetAffinities(v []Affinity)`

SetAffinities sets Affinities field to given value.

### HasAffinities

`func (o *RequestedDevice) HasAffinities() bool`

HasAffinities returns a boolean if a field has been set.

### GetConstraints

`func (o *RequestedDevice) GetConstraints() []Constraint`

GetConstraints returns the Constraints field if non-nil, zero value otherwise.

### GetConstraintsOk

`func (o *RequestedDevice) GetConstraintsOk() (*[]Constraint, bool)`

GetConstraintsOk returns a tuple with the Constraints field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetConstraints

`func (o *RequestedDevice) SetConstraints(v []Constraint)`

SetConstraints sets Constraints field to given value.

### HasConstraints

`func (o *RequestedDevice) HasConstraints() bool`

HasConstraints returns a boolean if a field has been set.

### GetCount

`func (o *RequestedDevice) GetCount() int32`

GetCount returns the Count field if non-nil, zero value otherwise.

### GetCountOk

`func (o *RequestedDevice) GetCountOk() (*int32, bool)`

GetCountOk returns a tuple with the Count field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCount

`func (o *RequestedDevice) SetCount(v int32)`

SetCount sets Count field to given value.

### HasCount

`func (o *RequestedDevice) HasCount() bool`

HasCount returns a boolean if a field has been set.

### GetName

`func (o *RequestedDevice) GetName() string`

GetName returns the Name field if non-nil, zero value otherwise.

### GetNameOk

`func (o *RequestedDevice) GetNameOk() (*string, bool)`

GetNameOk returns a tuple with the Name field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetName

`func (o *RequestedDevice) SetName(v string)`

SetName sets Name field to given value.

### HasName

`func (o *RequestedDevice) HasName() bool`

HasName returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


