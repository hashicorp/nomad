# Template

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**ChangeMode** | Pointer to **string** |  | [optional] 
**ChangeSignal** | Pointer to **string** |  | [optional] 
**DestPath** | Pointer to **string** |  | [optional] 
**EmbeddedTmpl** | Pointer to **string** |  | [optional] 
**Envvars** | Pointer to **bool** |  | [optional] 
**LeftDelim** | Pointer to **string** |  | [optional] 
**Perms** | Pointer to **string** |  | [optional] 
**RightDelim** | Pointer to **string** |  | [optional] 
**SourcePath** | Pointer to **string** |  | [optional] 
**Splay** | Pointer to **int64** |  | [optional] 
**VaultGrace** | Pointer to **int64** |  | [optional] 

## Methods

### NewTemplate

`func NewTemplate() *Template`

NewTemplate instantiates a new Template object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewTemplateWithDefaults

`func NewTemplateWithDefaults() *Template`

NewTemplateWithDefaults instantiates a new Template object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetChangeMode

`func (o *Template) GetChangeMode() string`

GetChangeMode returns the ChangeMode field if non-nil, zero value otherwise.

### GetChangeModeOk

`func (o *Template) GetChangeModeOk() (*string, bool)`

GetChangeModeOk returns a tuple with the ChangeMode field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetChangeMode

`func (o *Template) SetChangeMode(v string)`

SetChangeMode sets ChangeMode field to given value.

### HasChangeMode

`func (o *Template) HasChangeMode() bool`

HasChangeMode returns a boolean if a field has been set.

### GetChangeSignal

`func (o *Template) GetChangeSignal() string`

GetChangeSignal returns the ChangeSignal field if non-nil, zero value otherwise.

### GetChangeSignalOk

`func (o *Template) GetChangeSignalOk() (*string, bool)`

GetChangeSignalOk returns a tuple with the ChangeSignal field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetChangeSignal

`func (o *Template) SetChangeSignal(v string)`

SetChangeSignal sets ChangeSignal field to given value.

### HasChangeSignal

`func (o *Template) HasChangeSignal() bool`

HasChangeSignal returns a boolean if a field has been set.

### GetDestPath

`func (o *Template) GetDestPath() string`

GetDestPath returns the DestPath field if non-nil, zero value otherwise.

### GetDestPathOk

`func (o *Template) GetDestPathOk() (*string, bool)`

GetDestPathOk returns a tuple with the DestPath field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDestPath

`func (o *Template) SetDestPath(v string)`

SetDestPath sets DestPath field to given value.

### HasDestPath

`func (o *Template) HasDestPath() bool`

HasDestPath returns a boolean if a field has been set.

### GetEmbeddedTmpl

`func (o *Template) GetEmbeddedTmpl() string`

GetEmbeddedTmpl returns the EmbeddedTmpl field if non-nil, zero value otherwise.

### GetEmbeddedTmplOk

`func (o *Template) GetEmbeddedTmplOk() (*string, bool)`

GetEmbeddedTmplOk returns a tuple with the EmbeddedTmpl field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEmbeddedTmpl

`func (o *Template) SetEmbeddedTmpl(v string)`

SetEmbeddedTmpl sets EmbeddedTmpl field to given value.

### HasEmbeddedTmpl

`func (o *Template) HasEmbeddedTmpl() bool`

HasEmbeddedTmpl returns a boolean if a field has been set.

### GetEnvvars

`func (o *Template) GetEnvvars() bool`

GetEnvvars returns the Envvars field if non-nil, zero value otherwise.

### GetEnvvarsOk

`func (o *Template) GetEnvvarsOk() (*bool, bool)`

GetEnvvarsOk returns a tuple with the Envvars field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEnvvars

`func (o *Template) SetEnvvars(v bool)`

SetEnvvars sets Envvars field to given value.

### HasEnvvars

`func (o *Template) HasEnvvars() bool`

HasEnvvars returns a boolean if a field has been set.

### GetLeftDelim

`func (o *Template) GetLeftDelim() string`

GetLeftDelim returns the LeftDelim field if non-nil, zero value otherwise.

### GetLeftDelimOk

`func (o *Template) GetLeftDelimOk() (*string, bool)`

GetLeftDelimOk returns a tuple with the LeftDelim field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetLeftDelim

`func (o *Template) SetLeftDelim(v string)`

SetLeftDelim sets LeftDelim field to given value.

### HasLeftDelim

`func (o *Template) HasLeftDelim() bool`

HasLeftDelim returns a boolean if a field has been set.

### GetPerms

`func (o *Template) GetPerms() string`

GetPerms returns the Perms field if non-nil, zero value otherwise.

### GetPermsOk

`func (o *Template) GetPermsOk() (*string, bool)`

GetPermsOk returns a tuple with the Perms field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPerms

`func (o *Template) SetPerms(v string)`

SetPerms sets Perms field to given value.

### HasPerms

`func (o *Template) HasPerms() bool`

HasPerms returns a boolean if a field has been set.

### GetRightDelim

`func (o *Template) GetRightDelim() string`

GetRightDelim returns the RightDelim field if non-nil, zero value otherwise.

### GetRightDelimOk

`func (o *Template) GetRightDelimOk() (*string, bool)`

GetRightDelimOk returns a tuple with the RightDelim field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetRightDelim

`func (o *Template) SetRightDelim(v string)`

SetRightDelim sets RightDelim field to given value.

### HasRightDelim

`func (o *Template) HasRightDelim() bool`

HasRightDelim returns a boolean if a field has been set.

### GetSourcePath

`func (o *Template) GetSourcePath() string`

GetSourcePath returns the SourcePath field if non-nil, zero value otherwise.

### GetSourcePathOk

`func (o *Template) GetSourcePathOk() (*string, bool)`

GetSourcePathOk returns a tuple with the SourcePath field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSourcePath

`func (o *Template) SetSourcePath(v string)`

SetSourcePath sets SourcePath field to given value.

### HasSourcePath

`func (o *Template) HasSourcePath() bool`

HasSourcePath returns a boolean if a field has been set.

### GetSplay

`func (o *Template) GetSplay() int64`

GetSplay returns the Splay field if non-nil, zero value otherwise.

### GetSplayOk

`func (o *Template) GetSplayOk() (*int64, bool)`

GetSplayOk returns a tuple with the Splay field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSplay

`func (o *Template) SetSplay(v int64)`

SetSplay sets Splay field to given value.

### HasSplay

`func (o *Template) HasSplay() bool`

HasSplay returns a boolean if a field has been set.

### GetVaultGrace

`func (o *Template) GetVaultGrace() int64`

GetVaultGrace returns the VaultGrace field if non-nil, zero value otherwise.

### GetVaultGraceOk

`func (o *Template) GetVaultGraceOk() (*int64, bool)`

GetVaultGraceOk returns a tuple with the VaultGrace field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetVaultGrace

`func (o *Template) SetVaultGrace(v int64)`

SetVaultGrace sets VaultGrace field to given value.

### HasVaultGrace

`func (o *Template) HasVaultGrace() bool`

HasVaultGrace returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


