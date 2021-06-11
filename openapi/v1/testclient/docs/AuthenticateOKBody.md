# AuthenticateOKBody

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**IdentityToken** | **string** | An opaque token used to authenticate a user after a successful login | 
**Status** | **string** | The status of the authentication | 

## Methods

### NewAuthenticateOKBody

`func NewAuthenticateOKBody(identityToken string, status string, ) *AuthenticateOKBody`

NewAuthenticateOKBody instantiates a new AuthenticateOKBody object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewAuthenticateOKBodyWithDefaults

`func NewAuthenticateOKBodyWithDefaults() *AuthenticateOKBody`

NewAuthenticateOKBodyWithDefaults instantiates a new AuthenticateOKBody object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetIdentityToken

`func (o *AuthenticateOKBody) GetIdentityToken() string`

GetIdentityToken returns the IdentityToken field if non-nil, zero value otherwise.

### GetIdentityTokenOk

`func (o *AuthenticateOKBody) GetIdentityTokenOk() (*string, bool)`

GetIdentityTokenOk returns a tuple with the IdentityToken field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetIdentityToken

`func (o *AuthenticateOKBody) SetIdentityToken(v string)`

SetIdentityToken sets IdentityToken field to given value.


### GetStatus

`func (o *AuthenticateOKBody) GetStatus() string`

GetStatus returns the Status field if non-nil, zero value otherwise.

### GetStatusOk

`func (o *AuthenticateOKBody) GetStatusOk() (*string, bool)`

GetStatusOk returns a tuple with the Status field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetStatus

`func (o *AuthenticateOKBody) SetStatus(v string)`

SetStatus sets Status field to given value.



[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


