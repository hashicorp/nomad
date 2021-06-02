# ContainerCreateCreatedBody

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Id** | **string** | The ID of the created container | 
**Warnings** | **[]string** | Warnings encountered when creating the container | 

## Methods

### NewContainerCreateCreatedBody

`func NewContainerCreateCreatedBody(id string, warnings []string, ) *ContainerCreateCreatedBody`

NewContainerCreateCreatedBody instantiates a new ContainerCreateCreatedBody object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewContainerCreateCreatedBodyWithDefaults

`func NewContainerCreateCreatedBodyWithDefaults() *ContainerCreateCreatedBody`

NewContainerCreateCreatedBodyWithDefaults instantiates a new ContainerCreateCreatedBody object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetId

`func (o *ContainerCreateCreatedBody) GetId() string`

GetId returns the Id field if non-nil, zero value otherwise.

### GetIdOk

`func (o *ContainerCreateCreatedBody) GetIdOk() (*string, bool)`

GetIdOk returns a tuple with the Id field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetId

`func (o *ContainerCreateCreatedBody) SetId(v string)`

SetId sets Id field to given value.


### GetWarnings

`func (o *ContainerCreateCreatedBody) GetWarnings() []string`

GetWarnings returns the Warnings field if non-nil, zero value otherwise.

### GetWarningsOk

`func (o *ContainerCreateCreatedBody) GetWarningsOk() (*[]string, bool)`

GetWarningsOk returns a tuple with the Warnings field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetWarnings

`func (o *ContainerCreateCreatedBody) SetWarnings(v []string)`

SetWarnings sets Warnings field to given value.



[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


