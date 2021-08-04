# DNSConfig

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Options** | Pointer to **[]string** |  | [optional] 
**Searches** | Pointer to **[]string** |  | [optional] 
**Servers** | Pointer to **[]string** |  | [optional] 

## Methods

### NewDNSConfig

`func NewDNSConfig() *DNSConfig`

NewDNSConfig instantiates a new DNSConfig object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewDNSConfigWithDefaults

`func NewDNSConfigWithDefaults() *DNSConfig`

NewDNSConfigWithDefaults instantiates a new DNSConfig object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetOptions

`func (o *DNSConfig) GetOptions() []string`

GetOptions returns the Options field if non-nil, zero value otherwise.

### GetOptionsOk

`func (o *DNSConfig) GetOptionsOk() (*[]string, bool)`

GetOptionsOk returns a tuple with the Options field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetOptions

`func (o *DNSConfig) SetOptions(v []string)`

SetOptions sets Options field to given value.

### HasOptions

`func (o *DNSConfig) HasOptions() bool`

HasOptions returns a boolean if a field has been set.

### GetSearches

`func (o *DNSConfig) GetSearches() []string`

GetSearches returns the Searches field if non-nil, zero value otherwise.

### GetSearchesOk

`func (o *DNSConfig) GetSearchesOk() (*[]string, bool)`

GetSearchesOk returns a tuple with the Searches field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSearches

`func (o *DNSConfig) SetSearches(v []string)`

SetSearches sets Searches field to given value.

### HasSearches

`func (o *DNSConfig) HasSearches() bool`

HasSearches returns a boolean if a field has been set.

### GetServers

`func (o *DNSConfig) GetServers() []string`

GetServers returns the Servers field if non-nil, zero value otherwise.

### GetServersOk

`func (o *DNSConfig) GetServersOk() (*[]string, bool)`

GetServersOk returns a tuple with the Servers field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetServers

`func (o *DNSConfig) SetServers(v []string)`

SetServers sets Servers field to given value.

### HasServers

`func (o *DNSConfig) HasServers() bool`

HasServers returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


