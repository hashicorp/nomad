# JobVersionsResponse

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Diffs** | Pointer to [**[]JobDiff**](JobDiff.md) |  | [optional] 
**KnownLeader** | Pointer to **bool** | Is there a known leader | [optional] 
**LastContact** | Pointer to **int64** | A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years. | [optional] 
**LastIndex** | Pointer to **int32** | LastIndex. This can be used as a WaitIndex to perform a blocking query | [optional] 
**RequestTime** | Pointer to **int64** | A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years. | [optional] 
**Versions** | Pointer to [**[]Job**](Job.md) |  | [optional] 

## Methods

### NewJobVersionsResponse

`func NewJobVersionsResponse() *JobVersionsResponse`

NewJobVersionsResponse instantiates a new JobVersionsResponse object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewJobVersionsResponseWithDefaults

`func NewJobVersionsResponseWithDefaults() *JobVersionsResponse`

NewJobVersionsResponseWithDefaults instantiates a new JobVersionsResponse object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetDiffs

`func (o *JobVersionsResponse) GetDiffs() []JobDiff`

GetDiffs returns the Diffs field if non-nil, zero value otherwise.

### GetDiffsOk

`func (o *JobVersionsResponse) GetDiffsOk() (*[]JobDiff, bool)`

GetDiffsOk returns a tuple with the Diffs field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDiffs

`func (o *JobVersionsResponse) SetDiffs(v []JobDiff)`

SetDiffs sets Diffs field to given value.

### HasDiffs

`func (o *JobVersionsResponse) HasDiffs() bool`

HasDiffs returns a boolean if a field has been set.

### GetKnownLeader

`func (o *JobVersionsResponse) GetKnownLeader() bool`

GetKnownLeader returns the KnownLeader field if non-nil, zero value otherwise.

### GetKnownLeaderOk

`func (o *JobVersionsResponse) GetKnownLeaderOk() (*bool, bool)`

GetKnownLeaderOk returns a tuple with the KnownLeader field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetKnownLeader

`func (o *JobVersionsResponse) SetKnownLeader(v bool)`

SetKnownLeader sets KnownLeader field to given value.

### HasKnownLeader

`func (o *JobVersionsResponse) HasKnownLeader() bool`

HasKnownLeader returns a boolean if a field has been set.

### GetLastContact

`func (o *JobVersionsResponse) GetLastContact() int64`

GetLastContact returns the LastContact field if non-nil, zero value otherwise.

### GetLastContactOk

`func (o *JobVersionsResponse) GetLastContactOk() (*int64, bool)`

GetLastContactOk returns a tuple with the LastContact field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLastContact

`func (o *JobVersionsResponse) SetLastContact(v int64)`

SetLastContact sets LastContact field to given value.

### HasLastContact

`func (o *JobVersionsResponse) HasLastContact() bool`

HasLastContact returns a boolean if a field has been set.

### GetLastIndex

`func (o *JobVersionsResponse) GetLastIndex() int32`

GetLastIndex returns the LastIndex field if non-nil, zero value otherwise.

### GetLastIndexOk

`func (o *JobVersionsResponse) GetLastIndexOk() (*int32, bool)`

GetLastIndexOk returns a tuple with the LastIndex field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLastIndex

`func (o *JobVersionsResponse) SetLastIndex(v int32)`

SetLastIndex sets LastIndex field to given value.

### HasLastIndex

`func (o *JobVersionsResponse) HasLastIndex() bool`

HasLastIndex returns a boolean if a field has been set.

### GetRequestTime

`func (o *JobVersionsResponse) GetRequestTime() int64`

GetRequestTime returns the RequestTime field if non-nil, zero value otherwise.

### GetRequestTimeOk

`func (o *JobVersionsResponse) GetRequestTimeOk() (*int64, bool)`

GetRequestTimeOk returns a tuple with the RequestTime field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetRequestTime

`func (o *JobVersionsResponse) SetRequestTime(v int64)`

SetRequestTime sets RequestTime field to given value.

### HasRequestTime

`func (o *JobVersionsResponse) HasRequestTime() bool`

HasRequestTime returns a boolean if a field has been set.

### GetVersions

`func (o *JobVersionsResponse) GetVersions() []Job`

GetVersions returns the Versions field if non-nil, zero value otherwise.

### GetVersionsOk

`func (o *JobVersionsResponse) GetVersionsOk() (*[]Job, bool)`

GetVersionsOk returns a tuple with the Versions field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetVersions

`func (o *JobVersionsResponse) SetVersions(v []Job)`

SetVersions sets Versions field to given value.

### HasVersions

`func (o *JobVersionsResponse) HasVersions() bool`

HasVersions returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


