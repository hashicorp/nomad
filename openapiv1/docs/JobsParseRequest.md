# JobsParseRequest

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Canonicalize** | Pointer to **bool** | Canonicalize is a flag as to if the server should return default values for unset fields | [optional] 
**JobHCL** | Pointer to **string** | JobHCL is an hcl jobspec | [optional] 
**Hclv1** | Pointer to **bool** | HCLv1 indicates whether the JobHCL should be parsed with the hcl v1 parser | [optional] 

## Methods

### NewJobsParseRequest

`func NewJobsParseRequest() *JobsParseRequest`

NewJobsParseRequest instantiates a new JobsParseRequest object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewJobsParseRequestWithDefaults

`func NewJobsParseRequestWithDefaults() *JobsParseRequest`

NewJobsParseRequestWithDefaults instantiates a new JobsParseRequest object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetCanonicalize

`func (o *JobsParseRequest) GetCanonicalize() bool`

GetCanonicalize returns the Canonicalize field if non-nil, zero value otherwise.

### GetCanonicalizeOk

`func (o *JobsParseRequest) GetCanonicalizeOk() (*bool, bool)`

GetCanonicalizeOk returns a tuple with the Canonicalize field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCanonicalize

`func (o *JobsParseRequest) SetCanonicalize(v bool)`

SetCanonicalize sets Canonicalize field to given value.

### HasCanonicalize

`func (o *JobsParseRequest) HasCanonicalize() bool`

HasCanonicalize returns a boolean if a field has been set.

### GetJobHCL

`func (o *JobsParseRequest) GetJobHCL() string`

GetJobHCL returns the JobHCL field if non-nil, zero value otherwise.

### GetJobHCLOk

`func (o *JobsParseRequest) GetJobHCLOk() (*string, bool)`

GetJobHCLOk returns a tuple with the JobHCL field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetJobHCL

`func (o *JobsParseRequest) SetJobHCL(v string)`

SetJobHCL sets JobHCL field to given value.

### HasJobHCL

`func (o *JobsParseRequest) HasJobHCL() bool`

HasJobHCL returns a boolean if a field has been set.

### GetHclv1

`func (o *JobsParseRequest) GetHclv1() bool`

GetHclv1 returns the Hclv1 field if non-nil, zero value otherwise.

### GetHclv1Ok

`func (o *JobsParseRequest) GetHclv1Ok() (*bool, bool)`

GetHclv1Ok returns a tuple with the Hclv1 field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetHclv1

`func (o *JobsParseRequest) SetHclv1(v bool)`

SetHclv1 sets Hclv1 field to given value.

### HasHclv1

`func (o *JobsParseRequest) HasHclv1() bool`

HasHclv1 returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


