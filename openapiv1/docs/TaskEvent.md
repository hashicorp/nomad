# TaskEvent

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Details** | Pointer to **map[string]string** | Details is a map with annotated info about the event | [optional] 
**DiskLimit** | Pointer to **int64** | The maximum allowed task disk size. Deprecated, use Details[\&quot;disk_limit\&quot;] to access this. | [optional] 
**DisplayMessage** | Pointer to **string** | DisplayMessage is a human friendly message about the event | [optional] 
**DownloadError** | Pointer to **string** | Artifact Download fields Deprecated, use Details[\&quot;download_error\&quot;] to access this. | [optional] 
**DriverError** | Pointer to **string** | Driver Failure fields. Deprecated, use Details[\&quot;driver_error\&quot;] to access this. | [optional] 
**DriverMessage** | Pointer to **string** | DriverMessage indicates a driver action being taken. Deprecated, use Details[\&quot;driver_message\&quot;] to access this. | [optional] 
**ExitCode** | Pointer to **int64** | Deprecated, use Details[\&quot;exit_code\&quot;] to access this. | [optional] 
**FailedSibling** | Pointer to **string** | Name of the sibling task that caused termination of the task that the TaskEvent refers to. Deprecated, use Details[\&quot;failed_sibling\&quot;] to access this. | [optional] 
**FailsTask** | Pointer to **bool** | FailsTask marks whether this event fails the task. Deprecated, use Details[\&quot;fails_task\&quot;] to access this. | [optional] 
**GenericSource** | Pointer to **string** | GenericSource is the source of a message. Deprecated, is redundant with event type. | [optional] 
**KillError** | Pointer to **string** | Task Killed Fields. Deprecated, use Details[\&quot;kill_error\&quot;] to access this. | [optional] 
**KillReason** | Pointer to **string** | KillReason is the reason the task was killed Deprecated, use Details[\&quot;kill_reason\&quot;] to access this. | [optional] 
**KillTimeout** | Pointer to **int64** | A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years. | [optional] 
**Message** | Pointer to **string** |  | [optional] 
**RestartReason** | Pointer to **string** | Restart fields. Deprecated, use Details[\&quot;restart_reason\&quot;] to access this. | [optional] 
**SetupError** | Pointer to **string** | Setup Failure fields. Deprecated, use Details[\&quot;setup_error\&quot;] to access this. | [optional] 
**Signal** | Pointer to **int64** | Deprecated, use Details[\&quot;signal\&quot;] to access this. | [optional] 
**StartDelay** | Pointer to **int64** | TaskRestarting fields. Deprecated, use Details[\&quot;start_delay\&quot;] to access this. | [optional] 
**TaskSignal** | Pointer to **string** | TaskSignal is the signal that was sent to the task Deprecated, use Details[\&quot;task_signal\&quot;] to access this. | [optional] 
**TaskSignalReason** | Pointer to **string** | TaskSignalReason indicates the reason the task is being signalled. Deprecated, use Details[\&quot;task_signal_reason\&quot;] to access this. | [optional] 
**Time** | Pointer to **int64** |  | [optional] 
**Type** | Pointer to **string** |  | [optional] 
**ValidationError** | Pointer to **string** | Validation fields Deprecated, use Details[\&quot;validation_error\&quot;] to access this. | [optional] 
**VaultError** | Pointer to **string** | VaultError is the error from token renewal Deprecated, use Details[\&quot;vault_renewal_error\&quot;] to access this. | [optional] 

## Methods

### NewTaskEvent

`func NewTaskEvent() *TaskEvent`

NewTaskEvent instantiates a new TaskEvent object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewTaskEventWithDefaults

`func NewTaskEventWithDefaults() *TaskEvent`

NewTaskEventWithDefaults instantiates a new TaskEvent object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetDetails

`func (o *TaskEvent) GetDetails() map[string]string`

GetDetails returns the Details field if non-nil, zero value otherwise.

### GetDetailsOk

`func (o *TaskEvent) GetDetailsOk() (*map[string]string, bool)`

GetDetailsOk returns a tuple with the Details field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDetails

`func (o *TaskEvent) SetDetails(v map[string]string)`

SetDetails sets Details field to given value.

### HasDetails

`func (o *TaskEvent) HasDetails() bool`

HasDetails returns a boolean if a field has been set.

### GetDiskLimit

`func (o *TaskEvent) GetDiskLimit() int64`

GetDiskLimit returns the DiskLimit field if non-nil, zero value otherwise.

### GetDiskLimitOk

`func (o *TaskEvent) GetDiskLimitOk() (*int64, bool)`

GetDiskLimitOk returns a tuple with the DiskLimit field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDiskLimit

`func (o *TaskEvent) SetDiskLimit(v int64)`

SetDiskLimit sets DiskLimit field to given value.

### HasDiskLimit

`func (o *TaskEvent) HasDiskLimit() bool`

HasDiskLimit returns a boolean if a field has been set.

### GetDisplayMessage

`func (o *TaskEvent) GetDisplayMessage() string`

GetDisplayMessage returns the DisplayMessage field if non-nil, zero value otherwise.

### GetDisplayMessageOk

`func (o *TaskEvent) GetDisplayMessageOk() (*string, bool)`

GetDisplayMessageOk returns a tuple with the DisplayMessage field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDisplayMessage

`func (o *TaskEvent) SetDisplayMessage(v string)`

SetDisplayMessage sets DisplayMessage field to given value.

### HasDisplayMessage

`func (o *TaskEvent) HasDisplayMessage() bool`

HasDisplayMessage returns a boolean if a field has been set.

### GetDownloadError

`func (o *TaskEvent) GetDownloadError() string`

GetDownloadError returns the DownloadError field if non-nil, zero value otherwise.

### GetDownloadErrorOk

`func (o *TaskEvent) GetDownloadErrorOk() (*string, bool)`

GetDownloadErrorOk returns a tuple with the DownloadError field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDownloadError

`func (o *TaskEvent) SetDownloadError(v string)`

SetDownloadError sets DownloadError field to given value.

### HasDownloadError

`func (o *TaskEvent) HasDownloadError() bool`

HasDownloadError returns a boolean if a field has been set.

### GetDriverError

`func (o *TaskEvent) GetDriverError() string`

GetDriverError returns the DriverError field if non-nil, zero value otherwise.

### GetDriverErrorOk

`func (o *TaskEvent) GetDriverErrorOk() (*string, bool)`

GetDriverErrorOk returns a tuple with the DriverError field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDriverError

`func (o *TaskEvent) SetDriverError(v string)`

SetDriverError sets DriverError field to given value.

### HasDriverError

`func (o *TaskEvent) HasDriverError() bool`

HasDriverError returns a boolean if a field has been set.

### GetDriverMessage

`func (o *TaskEvent) GetDriverMessage() string`

GetDriverMessage returns the DriverMessage field if non-nil, zero value otherwise.

### GetDriverMessageOk

`func (o *TaskEvent) GetDriverMessageOk() (*string, bool)`

GetDriverMessageOk returns a tuple with the DriverMessage field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDriverMessage

`func (o *TaskEvent) SetDriverMessage(v string)`

SetDriverMessage sets DriverMessage field to given value.

### HasDriverMessage

`func (o *TaskEvent) HasDriverMessage() bool`

HasDriverMessage returns a boolean if a field has been set.

### GetExitCode

`func (o *TaskEvent) GetExitCode() int64`

GetExitCode returns the ExitCode field if non-nil, zero value otherwise.

### GetExitCodeOk

`func (o *TaskEvent) GetExitCodeOk() (*int64, bool)`

GetExitCodeOk returns a tuple with the ExitCode field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetExitCode

`func (o *TaskEvent) SetExitCode(v int64)`

SetExitCode sets ExitCode field to given value.

### HasExitCode

`func (o *TaskEvent) HasExitCode() bool`

HasExitCode returns a boolean if a field has been set.

### GetFailedSibling

`func (o *TaskEvent) GetFailedSibling() string`

GetFailedSibling returns the FailedSibling field if non-nil, zero value otherwise.

### GetFailedSiblingOk

`func (o *TaskEvent) GetFailedSiblingOk() (*string, bool)`

GetFailedSiblingOk returns a tuple with the FailedSibling field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetFailedSibling

`func (o *TaskEvent) SetFailedSibling(v string)`

SetFailedSibling sets FailedSibling field to given value.

### HasFailedSibling

`func (o *TaskEvent) HasFailedSibling() bool`

HasFailedSibling returns a boolean if a field has been set.

### GetFailsTask

`func (o *TaskEvent) GetFailsTask() bool`

GetFailsTask returns the FailsTask field if non-nil, zero value otherwise.

### GetFailsTaskOk

`func (o *TaskEvent) GetFailsTaskOk() (*bool, bool)`

GetFailsTaskOk returns a tuple with the FailsTask field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetFailsTask

`func (o *TaskEvent) SetFailsTask(v bool)`

SetFailsTask sets FailsTask field to given value.

### HasFailsTask

`func (o *TaskEvent) HasFailsTask() bool`

HasFailsTask returns a boolean if a field has been set.

### GetGenericSource

`func (o *TaskEvent) GetGenericSource() string`

GetGenericSource returns the GenericSource field if non-nil, zero value otherwise.

### GetGenericSourceOk

`func (o *TaskEvent) GetGenericSourceOk() (*string, bool)`

GetGenericSourceOk returns a tuple with the GenericSource field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetGenericSource

`func (o *TaskEvent) SetGenericSource(v string)`

SetGenericSource sets GenericSource field to given value.

### HasGenericSource

`func (o *TaskEvent) HasGenericSource() bool`

HasGenericSource returns a boolean if a field has been set.

### GetKillError

`func (o *TaskEvent) GetKillError() string`

GetKillError returns the KillError field if non-nil, zero value otherwise.

### GetKillErrorOk

`func (o *TaskEvent) GetKillErrorOk() (*string, bool)`

GetKillErrorOk returns a tuple with the KillError field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetKillError

`func (o *TaskEvent) SetKillError(v string)`

SetKillError sets KillError field to given value.

### HasKillError

`func (o *TaskEvent) HasKillError() bool`

HasKillError returns a boolean if a field has been set.

### GetKillReason

`func (o *TaskEvent) GetKillReason() string`

GetKillReason returns the KillReason field if non-nil, zero value otherwise.

### GetKillReasonOk

`func (o *TaskEvent) GetKillReasonOk() (*string, bool)`

GetKillReasonOk returns a tuple with the KillReason field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetKillReason

`func (o *TaskEvent) SetKillReason(v string)`

SetKillReason sets KillReason field to given value.

### HasKillReason

`func (o *TaskEvent) HasKillReason() bool`

HasKillReason returns a boolean if a field has been set.

### GetKillTimeout

`func (o *TaskEvent) GetKillTimeout() int64`

GetKillTimeout returns the KillTimeout field if non-nil, zero value otherwise.

### GetKillTimeoutOk

`func (o *TaskEvent) GetKillTimeoutOk() (*int64, bool)`

GetKillTimeoutOk returns a tuple with the KillTimeout field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetKillTimeout

`func (o *TaskEvent) SetKillTimeout(v int64)`

SetKillTimeout sets KillTimeout field to given value.

### HasKillTimeout

`func (o *TaskEvent) HasKillTimeout() bool`

HasKillTimeout returns a boolean if a field has been set.

### GetMessage

`func (o *TaskEvent) GetMessage() string`

GetMessage returns the Message field if non-nil, zero value otherwise.

### GetMessageOk

`func (o *TaskEvent) GetMessageOk() (*string, bool)`

GetMessageOk returns a tuple with the Message field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMessage

`func (o *TaskEvent) SetMessage(v string)`

SetMessage sets Message field to given value.

### HasMessage

`func (o *TaskEvent) HasMessage() bool`

HasMessage returns a boolean if a field has been set.

### GetRestartReason

`func (o *TaskEvent) GetRestartReason() string`

GetRestartReason returns the RestartReason field if non-nil, zero value otherwise.

### GetRestartReasonOk

`func (o *TaskEvent) GetRestartReasonOk() (*string, bool)`

GetRestartReasonOk returns a tuple with the RestartReason field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetRestartReason

`func (o *TaskEvent) SetRestartReason(v string)`

SetRestartReason sets RestartReason field to given value.

### HasRestartReason

`func (o *TaskEvent) HasRestartReason() bool`

HasRestartReason returns a boolean if a field has been set.

### GetSetupError

`func (o *TaskEvent) GetSetupError() string`

GetSetupError returns the SetupError field if non-nil, zero value otherwise.

### GetSetupErrorOk

`func (o *TaskEvent) GetSetupErrorOk() (*string, bool)`

GetSetupErrorOk returns a tuple with the SetupError field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSetupError

`func (o *TaskEvent) SetSetupError(v string)`

SetSetupError sets SetupError field to given value.

### HasSetupError

`func (o *TaskEvent) HasSetupError() bool`

HasSetupError returns a boolean if a field has been set.

### GetSignal

`func (o *TaskEvent) GetSignal() int64`

GetSignal returns the Signal field if non-nil, zero value otherwise.

### GetSignalOk

`func (o *TaskEvent) GetSignalOk() (*int64, bool)`

GetSignalOk returns a tuple with the Signal field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSignal

`func (o *TaskEvent) SetSignal(v int64)`

SetSignal sets Signal field to given value.

### HasSignal

`func (o *TaskEvent) HasSignal() bool`

HasSignal returns a boolean if a field has been set.

### GetStartDelay

`func (o *TaskEvent) GetStartDelay() int64`

GetStartDelay returns the StartDelay field if non-nil, zero value otherwise.

### GetStartDelayOk

`func (o *TaskEvent) GetStartDelayOk() (*int64, bool)`

GetStartDelayOk returns a tuple with the StartDelay field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetStartDelay

`func (o *TaskEvent) SetStartDelay(v int64)`

SetStartDelay sets StartDelay field to given value.

### HasStartDelay

`func (o *TaskEvent) HasStartDelay() bool`

HasStartDelay returns a boolean if a field has been set.

### GetTaskSignal

`func (o *TaskEvent) GetTaskSignal() string`

GetTaskSignal returns the TaskSignal field if non-nil, zero value otherwise.

### GetTaskSignalOk

`func (o *TaskEvent) GetTaskSignalOk() (*string, bool)`

GetTaskSignalOk returns a tuple with the TaskSignal field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTaskSignal

`func (o *TaskEvent) SetTaskSignal(v string)`

SetTaskSignal sets TaskSignal field to given value.

### HasTaskSignal

`func (o *TaskEvent) HasTaskSignal() bool`

HasTaskSignal returns a boolean if a field has been set.

### GetTaskSignalReason

`func (o *TaskEvent) GetTaskSignalReason() string`

GetTaskSignalReason returns the TaskSignalReason field if non-nil, zero value otherwise.

### GetTaskSignalReasonOk

`func (o *TaskEvent) GetTaskSignalReasonOk() (*string, bool)`

GetTaskSignalReasonOk returns a tuple with the TaskSignalReason field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTaskSignalReason

`func (o *TaskEvent) SetTaskSignalReason(v string)`

SetTaskSignalReason sets TaskSignalReason field to given value.

### HasTaskSignalReason

`func (o *TaskEvent) HasTaskSignalReason() bool`

HasTaskSignalReason returns a boolean if a field has been set.

### GetTime

`func (o *TaskEvent) GetTime() int64`

GetTime returns the Time field if non-nil, zero value otherwise.

### GetTimeOk

`func (o *TaskEvent) GetTimeOk() (*int64, bool)`

GetTimeOk returns a tuple with the Time field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTime

`func (o *TaskEvent) SetTime(v int64)`

SetTime sets Time field to given value.

### HasTime

`func (o *TaskEvent) HasTime() bool`

HasTime returns a boolean if a field has been set.

### GetType

`func (o *TaskEvent) GetType() string`

GetType returns the Type field if non-nil, zero value otherwise.

### GetTypeOk

`func (o *TaskEvent) GetTypeOk() (*string, bool)`

GetTypeOk returns a tuple with the Type field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetType

`func (o *TaskEvent) SetType(v string)`

SetType sets Type field to given value.

### HasType

`func (o *TaskEvent) HasType() bool`

HasType returns a boolean if a field has been set.

### GetValidationError

`func (o *TaskEvent) GetValidationError() string`

GetValidationError returns the ValidationError field if non-nil, zero value otherwise.

### GetValidationErrorOk

`func (o *TaskEvent) GetValidationErrorOk() (*string, bool)`

GetValidationErrorOk returns a tuple with the ValidationError field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetValidationError

`func (o *TaskEvent) SetValidationError(v string)`

SetValidationError sets ValidationError field to given value.

### HasValidationError

`func (o *TaskEvent) HasValidationError() bool`

HasValidationError returns a boolean if a field has been set.

### GetVaultError

`func (o *TaskEvent) GetVaultError() string`

GetVaultError returns the VaultError field if non-nil, zero value otherwise.

### GetVaultErrorOk

`func (o *TaskEvent) GetVaultErrorOk() (*string, bool)`

GetVaultErrorOk returns a tuple with the VaultError field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetVaultError

`func (o *TaskEvent) SetVaultError(v string)`

SetVaultError sets VaultError field to given value.

### HasVaultError

`func (o *TaskEvent) HasVaultError() bool`

HasVaultError returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


