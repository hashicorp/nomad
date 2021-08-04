# ServiceCheck

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**AddressMode** | Pointer to **string** |  | [optional] 
**Args** | Pointer to **[]string** |  | [optional] 
**Body** | Pointer to **string** |  | [optional] 
**CheckRestart** | Pointer to [**CheckRestart**](CheckRestart.md) |  | [optional] 
**Command** | Pointer to **string** |  | [optional] 
**Expose** | Pointer to **bool** |  | [optional] 
**FailuresBeforeCritical** | Pointer to **int32** |  | [optional] 
**GRPCService** | Pointer to **string** |  | [optional] 
**GRPCUseTLS** | Pointer to **bool** |  | [optional] 
**Header** | Pointer to **map[string][]string** |  | [optional] 
**Id** | Pointer to **string** |  | [optional] 
**InitialStatus** | Pointer to **string** |  | [optional] 
**Interval** | Pointer to **int64** |  | [optional] 
**Method** | Pointer to **string** |  | [optional] 
**Name** | Pointer to **string** |  | [optional] 
**OnUpdate** | Pointer to **string** |  | [optional] 
**Path** | Pointer to **string** |  | [optional] 
**PortLabel** | Pointer to **string** |  | [optional] 
**Protocol** | Pointer to **string** |  | [optional] 
**SuccessBeforePassing** | Pointer to **int32** |  | [optional] 
**TLSSkipVerify** | Pointer to **bool** |  | [optional] 
**TaskName** | Pointer to **string** |  | [optional] 
**Timeout** | Pointer to **int64** |  | [optional] 
**Type** | Pointer to **string** |  | [optional] 

## Methods

### NewServiceCheck

`func NewServiceCheck() *ServiceCheck`

NewServiceCheck instantiates a new ServiceCheck object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewServiceCheckWithDefaults

`func NewServiceCheckWithDefaults() *ServiceCheck`

NewServiceCheckWithDefaults instantiates a new ServiceCheck object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetAddressMode

`func (o *ServiceCheck) GetAddressMode() string`

GetAddressMode returns the AddressMode field if non-nil, zero value otherwise.

### GetAddressModeOk

`func (o *ServiceCheck) GetAddressModeOk() (*string, bool)`

GetAddressModeOk returns a tuple with the AddressMode field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAddressMode

`func (o *ServiceCheck) SetAddressMode(v string)`

SetAddressMode sets AddressMode field to given value.

### HasAddressMode

`func (o *ServiceCheck) HasAddressMode() bool`

HasAddressMode returns a boolean if a field has been set.

### GetArgs

`func (o *ServiceCheck) GetArgs() []string`

GetArgs returns the Args field if non-nil, zero value otherwise.

### GetArgsOk

`func (o *ServiceCheck) GetArgsOk() (*[]string, bool)`

GetArgsOk returns a tuple with the Args field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetArgs

`func (o *ServiceCheck) SetArgs(v []string)`

SetArgs sets Args field to given value.

### HasArgs

`func (o *ServiceCheck) HasArgs() bool`

HasArgs returns a boolean if a field has been set.

### GetBody

`func (o *ServiceCheck) GetBody() string`

GetBody returns the Body field if non-nil, zero value otherwise.

### GetBodyOk

`func (o *ServiceCheck) GetBodyOk() (*string, bool)`

GetBodyOk returns a tuple with the Body field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetBody

`func (o *ServiceCheck) SetBody(v string)`

SetBody sets Body field to given value.

### HasBody

`func (o *ServiceCheck) HasBody() bool`

HasBody returns a boolean if a field has been set.

### GetCheckRestart

`func (o *ServiceCheck) GetCheckRestart() CheckRestart`

GetCheckRestart returns the CheckRestart field if non-nil, zero value otherwise.

### GetCheckRestartOk

`func (o *ServiceCheck) GetCheckRestartOk() (*CheckRestart, bool)`

GetCheckRestartOk returns a tuple with the CheckRestart field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCheckRestart

`func (o *ServiceCheck) SetCheckRestart(v CheckRestart)`

SetCheckRestart sets CheckRestart field to given value.

### HasCheckRestart

`func (o *ServiceCheck) HasCheckRestart() bool`

HasCheckRestart returns a boolean if a field has been set.

### GetCommand

`func (o *ServiceCheck) GetCommand() string`

GetCommand returns the Command field if non-nil, zero value otherwise.

### GetCommandOk

`func (o *ServiceCheck) GetCommandOk() (*string, bool)`

GetCommandOk returns a tuple with the Command field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCommand

`func (o *ServiceCheck) SetCommand(v string)`

SetCommand sets Command field to given value.

### HasCommand

`func (o *ServiceCheck) HasCommand() bool`

HasCommand returns a boolean if a field has been set.

### GetExpose

`func (o *ServiceCheck) GetExpose() bool`

GetExpose returns the Expose field if non-nil, zero value otherwise.

### GetExposeOk

`func (o *ServiceCheck) GetExposeOk() (*bool, bool)`

GetExposeOk returns a tuple with the Expose field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetExpose

`func (o *ServiceCheck) SetExpose(v bool)`

SetExpose sets Expose field to given value.

### HasExpose

`func (o *ServiceCheck) HasExpose() bool`

HasExpose returns a boolean if a field has been set.

### GetFailuresBeforeCritical

`func (o *ServiceCheck) GetFailuresBeforeCritical() int32`

GetFailuresBeforeCritical returns the FailuresBeforeCritical field if non-nil, zero value otherwise.

### GetFailuresBeforeCriticalOk

`func (o *ServiceCheck) GetFailuresBeforeCriticalOk() (*int32, bool)`

GetFailuresBeforeCriticalOk returns a tuple with the FailuresBeforeCritical field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetFailuresBeforeCritical

`func (o *ServiceCheck) SetFailuresBeforeCritical(v int32)`

SetFailuresBeforeCritical sets FailuresBeforeCritical field to given value.

### HasFailuresBeforeCritical

`func (o *ServiceCheck) HasFailuresBeforeCritical() bool`

HasFailuresBeforeCritical returns a boolean if a field has been set.

### GetGRPCService

`func (o *ServiceCheck) GetGRPCService() string`

GetGRPCService returns the GRPCService field if non-nil, zero value otherwise.

### GetGRPCServiceOk

`func (o *ServiceCheck) GetGRPCServiceOk() (*string, bool)`

GetGRPCServiceOk returns a tuple with the GRPCService field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetGRPCService

`func (o *ServiceCheck) SetGRPCService(v string)`

SetGRPCService sets GRPCService field to given value.

### HasGRPCService

`func (o *ServiceCheck) HasGRPCService() bool`

HasGRPCService returns a boolean if a field has been set.

### GetGRPCUseTLS

`func (o *ServiceCheck) GetGRPCUseTLS() bool`

GetGRPCUseTLS returns the GRPCUseTLS field if non-nil, zero value otherwise.

### GetGRPCUseTLSOk

`func (o *ServiceCheck) GetGRPCUseTLSOk() (*bool, bool)`

GetGRPCUseTLSOk returns a tuple with the GRPCUseTLS field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetGRPCUseTLS

`func (o *ServiceCheck) SetGRPCUseTLS(v bool)`

SetGRPCUseTLS sets GRPCUseTLS field to given value.

### HasGRPCUseTLS

`func (o *ServiceCheck) HasGRPCUseTLS() bool`

HasGRPCUseTLS returns a boolean if a field has been set.

### GetHeader

`func (o *ServiceCheck) GetHeader() map[string][]string`

GetHeader returns the Header field if non-nil, zero value otherwise.

### GetHeaderOk

`func (o *ServiceCheck) GetHeaderOk() (*map[string][]string, bool)`

GetHeaderOk returns a tuple with the Header field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetHeader

`func (o *ServiceCheck) SetHeader(v map[string][]string)`

SetHeader sets Header field to given value.

### HasHeader

`func (o *ServiceCheck) HasHeader() bool`

HasHeader returns a boolean if a field has been set.

### GetId

`func (o *ServiceCheck) GetId() string`

GetId returns the Id field if non-nil, zero value otherwise.

### GetIdOk

`func (o *ServiceCheck) GetIdOk() (*string, bool)`

GetIdOk returns a tuple with the Id field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetId

`func (o *ServiceCheck) SetId(v string)`

SetId sets Id field to given value.

### HasId

`func (o *ServiceCheck) HasId() bool`

HasId returns a boolean if a field has been set.

### GetInitialStatus

`func (o *ServiceCheck) GetInitialStatus() string`

GetInitialStatus returns the InitialStatus field if non-nil, zero value otherwise.

### GetInitialStatusOk

`func (o *ServiceCheck) GetInitialStatusOk() (*string, bool)`

GetInitialStatusOk returns a tuple with the InitialStatus field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetInitialStatus

`func (o *ServiceCheck) SetInitialStatus(v string)`

SetInitialStatus sets InitialStatus field to given value.

### HasInitialStatus

`func (o *ServiceCheck) HasInitialStatus() bool`

HasInitialStatus returns a boolean if a field has been set.

### GetInterval

`func (o *ServiceCheck) GetInterval() int64`

GetInterval returns the Interval field if non-nil, zero value otherwise.

### GetIntervalOk

`func (o *ServiceCheck) GetIntervalOk() (*int64, bool)`

GetIntervalOk returns a tuple with the Interval field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetInterval

`func (o *ServiceCheck) SetInterval(v int64)`

SetInterval sets Interval field to given value.

### HasInterval

`func (o *ServiceCheck) HasInterval() bool`

HasInterval returns a boolean if a field has been set.

### GetMethod

`func (o *ServiceCheck) GetMethod() string`

GetMethod returns the Method field if non-nil, zero value otherwise.

### GetMethodOk

`func (o *ServiceCheck) GetMethodOk() (*string, bool)`

GetMethodOk returns a tuple with the Method field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMethod

`func (o *ServiceCheck) SetMethod(v string)`

SetMethod sets Method field to given value.

### HasMethod

`func (o *ServiceCheck) HasMethod() bool`

HasMethod returns a boolean if a field has been set.

### GetName

`func (o *ServiceCheck) GetName() string`

GetName returns the Name field if non-nil, zero value otherwise.

### GetNameOk

`func (o *ServiceCheck) GetNameOk() (*string, bool)`

GetNameOk returns a tuple with the Name field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetName

`func (o *ServiceCheck) SetName(v string)`

SetName sets Name field to given value.

### HasName

`func (o *ServiceCheck) HasName() bool`

HasName returns a boolean if a field has been set.

### GetOnUpdate

`func (o *ServiceCheck) GetOnUpdate() string`

GetOnUpdate returns the OnUpdate field if non-nil, zero value otherwise.

### GetOnUpdateOk

`func (o *ServiceCheck) GetOnUpdateOk() (*string, bool)`

GetOnUpdateOk returns a tuple with the OnUpdate field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetOnUpdate

`func (o *ServiceCheck) SetOnUpdate(v string)`

SetOnUpdate sets OnUpdate field to given value.

### HasOnUpdate

`func (o *ServiceCheck) HasOnUpdate() bool`

HasOnUpdate returns a boolean if a field has been set.

### GetPath

`func (o *ServiceCheck) GetPath() string`

GetPath returns the Path field if non-nil, zero value otherwise.

### GetPathOk

`func (o *ServiceCheck) GetPathOk() (*string, bool)`

GetPathOk returns a tuple with the Path field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPath

`func (o *ServiceCheck) SetPath(v string)`

SetPath sets Path field to given value.

### HasPath

`func (o *ServiceCheck) HasPath() bool`

HasPath returns a boolean if a field has been set.

### GetPortLabel

`func (o *ServiceCheck) GetPortLabel() string`

GetPortLabel returns the PortLabel field if non-nil, zero value otherwise.

### GetPortLabelOk

`func (o *ServiceCheck) GetPortLabelOk() (*string, bool)`

GetPortLabelOk returns a tuple with the PortLabel field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPortLabel

`func (o *ServiceCheck) SetPortLabel(v string)`

SetPortLabel sets PortLabel field to given value.

### HasPortLabel

`func (o *ServiceCheck) HasPortLabel() bool`

HasPortLabel returns a boolean if a field has been set.

### GetProtocol

`func (o *ServiceCheck) GetProtocol() string`

GetProtocol returns the Protocol field if non-nil, zero value otherwise.

### GetProtocolOk

`func (o *ServiceCheck) GetProtocolOk() (*string, bool)`

GetProtocolOk returns a tuple with the Protocol field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetProtocol

`func (o *ServiceCheck) SetProtocol(v string)`

SetProtocol sets Protocol field to given value.

### HasProtocol

`func (o *ServiceCheck) HasProtocol() bool`

HasProtocol returns a boolean if a field has been set.

### GetSuccessBeforePassing

`func (o *ServiceCheck) GetSuccessBeforePassing() int32`

GetSuccessBeforePassing returns the SuccessBeforePassing field if non-nil, zero value otherwise.

### GetSuccessBeforePassingOk

`func (o *ServiceCheck) GetSuccessBeforePassingOk() (*int32, bool)`

GetSuccessBeforePassingOk returns a tuple with the SuccessBeforePassing field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSuccessBeforePassing

`func (o *ServiceCheck) SetSuccessBeforePassing(v int32)`

SetSuccessBeforePassing sets SuccessBeforePassing field to given value.

### HasSuccessBeforePassing

`func (o *ServiceCheck) HasSuccessBeforePassing() bool`

HasSuccessBeforePassing returns a boolean if a field has been set.

### GetTLSSkipVerify

`func (o *ServiceCheck) GetTLSSkipVerify() bool`

GetTLSSkipVerify returns the TLSSkipVerify field if non-nil, zero value otherwise.

### GetTLSSkipVerifyOk

`func (o *ServiceCheck) GetTLSSkipVerifyOk() (*bool, bool)`

GetTLSSkipVerifyOk returns a tuple with the TLSSkipVerify field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTLSSkipVerify

`func (o *ServiceCheck) SetTLSSkipVerify(v bool)`

SetTLSSkipVerify sets TLSSkipVerify field to given value.

### HasTLSSkipVerify

`func (o *ServiceCheck) HasTLSSkipVerify() bool`

HasTLSSkipVerify returns a boolean if a field has been set.

### GetTaskName

`func (o *ServiceCheck) GetTaskName() string`

GetTaskName returns the TaskName field if non-nil, zero value otherwise.

### GetTaskNameOk

`func (o *ServiceCheck) GetTaskNameOk() (*string, bool)`

GetTaskNameOk returns a tuple with the TaskName field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTaskName

`func (o *ServiceCheck) SetTaskName(v string)`

SetTaskName sets TaskName field to given value.

### HasTaskName

`func (o *ServiceCheck) HasTaskName() bool`

HasTaskName returns a boolean if a field has been set.

### GetTimeout

`func (o *ServiceCheck) GetTimeout() int64`

GetTimeout returns the Timeout field if non-nil, zero value otherwise.

### GetTimeoutOk

`func (o *ServiceCheck) GetTimeoutOk() (*int64, bool)`

GetTimeoutOk returns a tuple with the Timeout field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTimeout

`func (o *ServiceCheck) SetTimeout(v int64)`

SetTimeout sets Timeout field to given value.

### HasTimeout

`func (o *ServiceCheck) HasTimeout() bool`

HasTimeout returns a boolean if a field has been set.

### GetType

`func (o *ServiceCheck) GetType() string`

GetType returns the Type field if non-nil, zero value otherwise.

### GetTypeOk

`func (o *ServiceCheck) GetTypeOk() (*string, bool)`

GetTypeOk returns a tuple with the Type field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetType

`func (o *ServiceCheck) SetType(v string)`

SetType sets Type field to given value.

### HasType

`func (o *ServiceCheck) HasType() bool`

HasType returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


