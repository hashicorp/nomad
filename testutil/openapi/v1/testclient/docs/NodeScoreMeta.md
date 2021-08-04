# NodeScoreMeta

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**NodeID** | Pointer to **string** |  | [optional] 
**NormScore** | Pointer to **float64** |  | [optional] 
**Scores** | Pointer to **map[string]float64** |  | [optional] 

## Methods

### NewNodeScoreMeta

`func NewNodeScoreMeta() *NodeScoreMeta`

NewNodeScoreMeta instantiates a new NodeScoreMeta object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewNodeScoreMetaWithDefaults

`func NewNodeScoreMetaWithDefaults() *NodeScoreMeta`

NewNodeScoreMetaWithDefaults instantiates a new NodeScoreMeta object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetNodeID

`func (o *NodeScoreMeta) GetNodeID() string`

GetNodeID returns the NodeID field if non-nil, zero value otherwise.

### GetNodeIDOk

`func (o *NodeScoreMeta) GetNodeIDOk() (*string, bool)`

GetNodeIDOk returns a tuple with the NodeID field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNodeID

`func (o *NodeScoreMeta) SetNodeID(v string)`

SetNodeID sets NodeID field to given value.

### HasNodeID

`func (o *NodeScoreMeta) HasNodeID() bool`

HasNodeID returns a boolean if a field has been set.

### GetNormScore

`func (o *NodeScoreMeta) GetNormScore() float64`

GetNormScore returns the NormScore field if non-nil, zero value otherwise.

### GetNormScoreOk

`func (o *NodeScoreMeta) GetNormScoreOk() (*float64, bool)`

GetNormScoreOk returns a tuple with the NormScore field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNormScore

`func (o *NodeScoreMeta) SetNormScore(v float64)`

SetNormScore sets NormScore field to given value.

### HasNormScore

`func (o *NodeScoreMeta) HasNormScore() bool`

HasNormScore returns a boolean if a field has been set.

### GetScores

`func (o *NodeScoreMeta) GetScores() map[string]float64`

GetScores returns the Scores field if non-nil, zero value otherwise.

### GetScoresOk

`func (o *NodeScoreMeta) GetScoresOk() (*map[string]float64, bool)`

GetScoresOk returns a tuple with the Scores field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetScores

`func (o *NodeScoreMeta) SetScores(v map[string]float64)`

SetScores sets Scores field to given value.

### HasScores

`func (o *NodeScoreMeta) HasScores() bool`

HasScores returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


