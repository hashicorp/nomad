# JobValidateResponse

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**DriverConfigValidated** | Pointer to **bool** | DriverConfigValidated indicates whether the agent validated the driver config | [optional] 
**Error** | Pointer to **string** | Error is a string version of any error that may have occurred | [optional] 
**ValidationErrors** | Pointer to **[]string** | ValidationErrors is a list of validation errors | [optional] 
**Warnings** | Pointer to **string** | Warnings contains any warnings about the given job. These may include deprecation warnings. | [optional] 

## Methods

### NewJobValidateResponse

`func NewJobValidateResponse() *JobValidateResponse`

NewJobValidateResponse instantiates a new JobValidateResponse object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewJobValidateResponseWithDefaults

`func NewJobValidateResponseWithDefaults() *JobValidateResponse`

NewJobValidateResponseWithDefaults instantiates a new JobValidateResponse object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetDriverConfigValidated

`func (o *JobValidateResponse) GetDriverConfigValidated() bool`

GetDriverConfigValidated returns the DriverConfigValidated field if non-nil, zero value otherwise.

### GetDriverConfigValidatedOk

`func (o *JobValidateResponse) GetDriverConfigValidatedOk() (*bool, bool)`

GetDriverConfigValidatedOk returns a tuple with the DriverConfigValidated field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDriverConfigValidated

`func (o *JobValidateResponse) SetDriverConfigValidated(v bool)`

SetDriverConfigValidated sets DriverConfigValidated field to given value.

### HasDriverConfigValidated

`func (o *JobValidateResponse) HasDriverConfigValidated() bool`

HasDriverConfigValidated returns a boolean if a field has been set.

### GetError

`func (o *JobValidateResponse) GetError() string`

GetError returns the Error field if non-nil, zero value otherwise.

### GetErrorOk

`func (o *JobValidateResponse) GetErrorOk() (*string, bool)`

GetErrorOk returns a tuple with the Error field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetError

`func (o *JobValidateResponse) SetError(v string)`

SetError sets Error field to given value.

### HasError

`func (o *JobValidateResponse) HasError() bool`

HasError returns a boolean if a field has been set.

### GetValidationErrors

`func (o *JobValidateResponse) GetValidationErrors() []string`

GetValidationErrors returns the ValidationErrors field if non-nil, zero value otherwise.

### GetValidationErrorsOk

`func (o *JobValidateResponse) GetValidationErrorsOk() (*[]string, bool)`

GetValidationErrorsOk returns a tuple with the ValidationErrors field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetValidationErrors

`func (o *JobValidateResponse) SetValidationErrors(v []string)`

SetValidationErrors sets ValidationErrors field to given value.

### HasValidationErrors

`func (o *JobValidateResponse) HasValidationErrors() bool`

HasValidationErrors returns a boolean if a field has been set.

### GetWarnings

`func (o *JobValidateResponse) GetWarnings() string`

GetWarnings returns the Warnings field if non-nil, zero value otherwise.

### GetWarningsOk

`func (o *JobValidateResponse) GetWarningsOk() (*string, bool)`

GetWarningsOk returns a tuple with the Warnings field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetWarnings

`func (o *JobValidateResponse) SetWarnings(v string)`

SetWarnings sets Warnings field to given value.

### HasWarnings

`func (o *JobValidateResponse) HasWarnings() bool`

HasWarnings returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


