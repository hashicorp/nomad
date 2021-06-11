# ContainerTopOKBody

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Processes** | **[][]string** | Each process running in the container, where each is process is an array of values corresponding to the titles | 
**Titles** | **[]string** | The ps column titles | 

## Methods

### NewContainerTopOKBody

`func NewContainerTopOKBody(processes [][]string, titles []string, ) *ContainerTopOKBody`

NewContainerTopOKBody instantiates a new ContainerTopOKBody object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewContainerTopOKBodyWithDefaults

`func NewContainerTopOKBodyWithDefaults() *ContainerTopOKBody`

NewContainerTopOKBodyWithDefaults instantiates a new ContainerTopOKBody object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetProcesses

`func (o *ContainerTopOKBody) GetProcesses() [][]string`

GetProcesses returns the Processes field if non-nil, zero value otherwise.

### GetProcessesOk

`func (o *ContainerTopOKBody) GetProcessesOk() (*[][]string, bool)`

GetProcessesOk returns a tuple with the Processes field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetProcesses

`func (o *ContainerTopOKBody) SetProcesses(v [][]string)`

SetProcesses sets Processes field to given value.


### GetTitles

`func (o *ContainerTopOKBody) GetTitles() []string`

GetTitles returns the Titles field if non-nil, zero value otherwise.

### GetTitlesOk

`func (o *ContainerTopOKBody) GetTitlesOk() (*[]string, bool)`

GetTitlesOk returns a tuple with the Titles field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTitles

`func (o *ContainerTopOKBody) SetTitles(v []string)`

SetTitles sets Titles field to given value.



[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


