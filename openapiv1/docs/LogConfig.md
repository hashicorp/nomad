# LogConfig

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**MaxFileSizeMB** | Pointer to **int64** |  | [optional] 
**MaxFiles** | Pointer to **int64** |  | [optional] 

## Methods

### NewLogConfig

`func NewLogConfig() *LogConfig`

NewLogConfig instantiates a new LogConfig object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewLogConfigWithDefaults

`func NewLogConfigWithDefaults() *LogConfig`

NewLogConfigWithDefaults instantiates a new LogConfig object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetMaxFileSizeMB

`func (o *LogConfig) GetMaxFileSizeMB() int64`

GetMaxFileSizeMB returns the MaxFileSizeMB field if non-nil, zero value otherwise.

### GetMaxFileSizeMBOk

`func (o *LogConfig) GetMaxFileSizeMBOk() (*int64, bool)`

GetMaxFileSizeMBOk returns a tuple with the MaxFileSizeMB field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMaxFileSizeMB

`func (o *LogConfig) SetMaxFileSizeMB(v int64)`

SetMaxFileSizeMB sets MaxFileSizeMB field to given value.

### HasMaxFileSizeMB

`func (o *LogConfig) HasMaxFileSizeMB() bool`

HasMaxFileSizeMB returns a boolean if a field has been set.

### GetMaxFiles

`func (o *LogConfig) GetMaxFiles() int64`

GetMaxFiles returns the MaxFiles field if non-nil, zero value otherwise.

### GetMaxFilesOk

`func (o *LogConfig) GetMaxFilesOk() (*int64, bool)`

GetMaxFilesOk returns a tuple with the MaxFiles field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMaxFiles

`func (o *LogConfig) SetMaxFiles(v int64)`

SetMaxFiles sets MaxFiles field to given value.

### HasMaxFiles

`func (o *LogConfig) HasMaxFiles() bool`

HasMaxFiles returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


