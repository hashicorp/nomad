# TaskState

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Events** | Pointer to [**[]TaskEvent**](TaskEvent.md) | Series of task events that transition the state of the task. | [optional] 
**Failed** | Pointer to **bool** | Failed marks a task as having failed | [optional] 
**FinishedAt** | Pointer to **time.Time** | FinishedAt is the time at which the task transitioned to dead and will not be started again. | [optional] 
**LastRestart** | Pointer to **time.Time** | LastRestart is the time the task last restarted. It is updated each time the task restarts | [optional] 
**Restarts** | Pointer to **int32** | Restarts is the number of times the task has restarted | [optional] 
**StartedAt** | Pointer to **time.Time** | StartedAt is the time the task is started. It is updated each time the task starts | [optional] 
**State** | Pointer to **string** | The current state of the task. | [optional] 
**TaskHandle** | Pointer to [**TaskHandle**](TaskHandle.md) |  | [optional] 

## Methods

### NewTaskState

`func NewTaskState() *TaskState`

NewTaskState instantiates a new TaskState object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewTaskStateWithDefaults

`func NewTaskStateWithDefaults() *TaskState`

NewTaskStateWithDefaults instantiates a new TaskState object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetEvents

`func (o *TaskState) GetEvents() []TaskEvent`

GetEvents returns the Events field if non-nil, zero value otherwise.

### GetEventsOk

`func (o *TaskState) GetEventsOk() (*[]TaskEvent, bool)`

GetEventsOk returns a tuple with the Events field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEvents

`func (o *TaskState) SetEvents(v []TaskEvent)`

SetEvents sets Events field to given value.

### HasEvents

`func (o *TaskState) HasEvents() bool`

HasEvents returns a boolean if a field has been set.

### GetFailed

`func (o *TaskState) GetFailed() bool`

GetFailed returns the Failed field if non-nil, zero value otherwise.

### GetFailedOk

`func (o *TaskState) GetFailedOk() (*bool, bool)`

GetFailedOk returns a tuple with the Failed field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetFailed

`func (o *TaskState) SetFailed(v bool)`

SetFailed sets Failed field to given value.

### HasFailed

`func (o *TaskState) HasFailed() bool`

HasFailed returns a boolean if a field has been set.

### GetFinishedAt

`func (o *TaskState) GetFinishedAt() time.Time`

GetFinishedAt returns the FinishedAt field if non-nil, zero value otherwise.

### GetFinishedAtOk

`func (o *TaskState) GetFinishedAtOk() (*time.Time, bool)`

GetFinishedAtOk returns a tuple with the FinishedAt field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetFinishedAt

`func (o *TaskState) SetFinishedAt(v time.Time)`

SetFinishedAt sets FinishedAt field to given value.

### HasFinishedAt

`func (o *TaskState) HasFinishedAt() bool`

HasFinishedAt returns a boolean if a field has been set.

### GetLastRestart

`func (o *TaskState) GetLastRestart() time.Time`

GetLastRestart returns the LastRestart field if non-nil, zero value otherwise.

### GetLastRestartOk

`func (o *TaskState) GetLastRestartOk() (*time.Time, bool)`

GetLastRestartOk returns a tuple with the LastRestart field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLastRestart

`func (o *TaskState) SetLastRestart(v time.Time)`

SetLastRestart sets LastRestart field to given value.

### HasLastRestart

`func (o *TaskState) HasLastRestart() bool`

HasLastRestart returns a boolean if a field has been set.

### GetRestarts

`func (o *TaskState) GetRestarts() int32`

GetRestarts returns the Restarts field if non-nil, zero value otherwise.

### GetRestartsOk

`func (o *TaskState) GetRestartsOk() (*int32, bool)`

GetRestartsOk returns a tuple with the Restarts field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetRestarts

`func (o *TaskState) SetRestarts(v int32)`

SetRestarts sets Restarts field to given value.

### HasRestarts

`func (o *TaskState) HasRestarts() bool`

HasRestarts returns a boolean if a field has been set.

### GetStartedAt

`func (o *TaskState) GetStartedAt() time.Time`

GetStartedAt returns the StartedAt field if non-nil, zero value otherwise.

### GetStartedAtOk

`func (o *TaskState) GetStartedAtOk() (*time.Time, bool)`

GetStartedAtOk returns a tuple with the StartedAt field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetStartedAt

`func (o *TaskState) SetStartedAt(v time.Time)`

SetStartedAt sets StartedAt field to given value.

### HasStartedAt

`func (o *TaskState) HasStartedAt() bool`

HasStartedAt returns a boolean if a field has been set.

### GetState

`func (o *TaskState) GetState() string`

GetState returns the State field if non-nil, zero value otherwise.

### GetStateOk

`func (o *TaskState) GetStateOk() (*string, bool)`

GetStateOk returns a tuple with the State field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetState

`func (o *TaskState) SetState(v string)`

SetState sets State field to given value.

### HasState

`func (o *TaskState) HasState() bool`

HasState returns a boolean if a field has been set.

### GetTaskHandle

`func (o *TaskState) GetTaskHandle() TaskHandle`

GetTaskHandle returns the TaskHandle field if non-nil, zero value otherwise.

### GetTaskHandleOk

`func (o *TaskState) GetTaskHandleOk() (*TaskHandle, bool)`

GetTaskHandleOk returns a tuple with the TaskHandle field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTaskHandle

`func (o *TaskState) SetTaskHandle(v TaskHandle)`

SetTaskHandle sets TaskHandle field to given value.

### HasTaskHandle

`func (o *TaskState) HasTaskHandle() bool`

HasTaskHandle returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


