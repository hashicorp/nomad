# ImageSummary

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Containers** | **int64** | containers | 
**Created** | **int64** | created | 
**Id** | **string** | Id | 
**Labels** | **map[string]string** | labels | 
**ParentId** | **string** | parent Id | 
**RepoDigests** | **[]string** | repo digests | 
**RepoTags** | **[]string** | repo tags | 
**SharedSize** | **int64** | shared size | 
**Size** | **int64** | size | 
**VirtualSize** | **int64** | virtual size | 

## Methods

### NewImageSummary

`func NewImageSummary(containers int64, created int64, id string, labels map[string]string, parentId string, repoDigests []string, repoTags []string, sharedSize int64, size int64, virtualSize int64, ) *ImageSummary`

NewImageSummary instantiates a new ImageSummary object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewImageSummaryWithDefaults

`func NewImageSummaryWithDefaults() *ImageSummary`

NewImageSummaryWithDefaults instantiates a new ImageSummary object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetContainers

`func (o *ImageSummary) GetContainers() int64`

GetContainers returns the Containers field if non-nil, zero value otherwise.

### GetContainersOk

`func (o *ImageSummary) GetContainersOk() (*int64, bool)`

GetContainersOk returns a tuple with the Containers field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetContainers

`func (o *ImageSummary) SetContainers(v int64)`

SetContainers sets Containers field to given value.


### GetCreated

`func (o *ImageSummary) GetCreated() int64`

GetCreated returns the Created field if non-nil, zero value otherwise.

### GetCreatedOk

`func (o *ImageSummary) GetCreatedOk() (*int64, bool)`

GetCreatedOk returns a tuple with the Created field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCreated

`func (o *ImageSummary) SetCreated(v int64)`

SetCreated sets Created field to given value.


### GetId

`func (o *ImageSummary) GetId() string`

GetId returns the Id field if non-nil, zero value otherwise.

### GetIdOk

`func (o *ImageSummary) GetIdOk() (*string, bool)`

GetIdOk returns a tuple with the Id field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetId

`func (o *ImageSummary) SetId(v string)`

SetId sets Id field to given value.


### GetLabels

`func (o *ImageSummary) GetLabels() map[string]string`

GetLabels returns the Labels field if non-nil, zero value otherwise.

### GetLabelsOk

`func (o *ImageSummary) GetLabelsOk() (*map[string]string, bool)`

GetLabelsOk returns a tuple with the Labels field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLabels

`func (o *ImageSummary) SetLabels(v map[string]string)`

SetLabels sets Labels field to given value.


### GetParentId

`func (o *ImageSummary) GetParentId() string`

GetParentId returns the ParentId field if non-nil, zero value otherwise.

### GetParentIdOk

`func (o *ImageSummary) GetParentIdOk() (*string, bool)`

GetParentIdOk returns a tuple with the ParentId field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetParentId

`func (o *ImageSummary) SetParentId(v string)`

SetParentId sets ParentId field to given value.


### GetRepoDigests

`func (o *ImageSummary) GetRepoDigests() []string`

GetRepoDigests returns the RepoDigests field if non-nil, zero value otherwise.

### GetRepoDigestsOk

`func (o *ImageSummary) GetRepoDigestsOk() (*[]string, bool)`

GetRepoDigestsOk returns a tuple with the RepoDigests field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetRepoDigests

`func (o *ImageSummary) SetRepoDigests(v []string)`

SetRepoDigests sets RepoDigests field to given value.


### GetRepoTags

`func (o *ImageSummary) GetRepoTags() []string`

GetRepoTags returns the RepoTags field if non-nil, zero value otherwise.

### GetRepoTagsOk

`func (o *ImageSummary) GetRepoTagsOk() (*[]string, bool)`

GetRepoTagsOk returns a tuple with the RepoTags field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetRepoTags

`func (o *ImageSummary) SetRepoTags(v []string)`

SetRepoTags sets RepoTags field to given value.


### GetSharedSize

`func (o *ImageSummary) GetSharedSize() int64`

GetSharedSize returns the SharedSize field if non-nil, zero value otherwise.

### GetSharedSizeOk

`func (o *ImageSummary) GetSharedSizeOk() (*int64, bool)`

GetSharedSizeOk returns a tuple with the SharedSize field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSharedSize

`func (o *ImageSummary) SetSharedSize(v int64)`

SetSharedSize sets SharedSize field to given value.


### GetSize

`func (o *ImageSummary) GetSize() int64`

GetSize returns the Size field if non-nil, zero value otherwise.

### GetSizeOk

`func (o *ImageSummary) GetSizeOk() (*int64, bool)`

GetSizeOk returns a tuple with the Size field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSize

`func (o *ImageSummary) SetSize(v int64)`

SetSize sets Size field to given value.


### GetVirtualSize

`func (o *ImageSummary) GetVirtualSize() int64`

GetVirtualSize returns the VirtualSize field if non-nil, zero value otherwise.

### GetVirtualSizeOk

`func (o *ImageSummary) GetVirtualSizeOk() (*int64, bool)`

GetVirtualSizeOk returns a tuple with the VirtualSize field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetVirtualSize

`func (o *ImageSummary) SetVirtualSize(v int64)`

SetVirtualSize sets VirtualSize field to given value.



[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


