package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	ErrVariableNotFound     = "secure variable not found"
	ErrVariableMissingItems = "secure variable missing Items field"
)

// SecureVariables is used to access secure variables.
type SecureVariables struct {
	client *Client
}

// SecureVariables returns a new handle on the secure variables.
func (c *Client) SecureVariables() *SecureVariables {
	return &SecureVariables{client: c}
}

// Create is used to create a secure variable.
func (sv *SecureVariables) Create(v *SecureVariable, qo *WriteOptions) (*WriteMeta, error) {

	v.Path = cleanPathString(v.Path)
	wm, err := sv.client.write("/v1/var/"+v.Path, v, nil, qo)
	if err != nil {
		return nil, err
	}
	return wm, nil
}

// CheckedCreate is used to create a secure variable if it doesn't exist
// already. If it does, it will return a ErrCASConflict that can be unwrapped
// for more details.
func (sv *SecureVariables) CheckedCreate(v *SecureVariable, qo *WriteOptions) (*WriteMeta, error) {

	v.Path = cleanPathString(v.Path)
	wm, err := sv.writeChecked("/v1/var/"+v.Path+"?cas=0", v, nil, qo)
	if err != nil {
		return nil, err
	}

	return wm, nil
}

// Read is used to query a single secure variable by path. This will error
// if the variable is not found.
func (sv *SecureVariables) Read(path string, qo *QueryOptions) (*SecureVariable, *QueryMeta, error) {

	path = cleanPathString(path)
	var svar = new(SecureVariable)
	qm, err := sv.readInternal("/v1/var/"+path, &svar, qo)
	if err != nil {
		return nil, nil, err
	}
	if svar == nil {
		return nil, qm, errors.New(ErrVariableNotFound)
	}
	return svar, qm, nil
}

// Peek is used to query a single secure variable by path, but does not error
// when the variable is not found
func (sv *SecureVariables) Peek(path string, qo *QueryOptions) (*SecureVariable, *QueryMeta, error) {

	path = cleanPathString(path)
	var svar = new(SecureVariable)
	qm, err := sv.readInternal("/v1/var/"+path, &svar, qo)
	if err != nil {
		return nil, nil, err
	}
	return svar, qm, nil
}

// Update is used to update a secure variable.
func (sv *SecureVariables) Update(v *SecureVariable, qo *WriteOptions) (*WriteMeta, error) {

	v.Path = cleanPathString(v.Path)
	wm, err := sv.client.write("/v1/var/"+v.Path, v, nil, qo)
	if err != nil {
		return nil, err
	}
	return wm, nil
}

// CheckedUpdate is used to updated a secure variable if the modify index
// matches the one on the server.  If it does not, it will return an
// ErrCASConflict that can be unwrapped for more details.
func (sv *SecureVariables) CheckedUpdate(v *SecureVariable, qo *WriteOptions) (*WriteMeta, error) {

	v.Path = cleanPathString(v.Path)
	wm, err := sv.writeChecked("/v1/var/"+v.Path+"?cas="+fmt.Sprint(v.ModifyIndex), v, nil, qo)
	if err != nil {
		return nil, err
	}

	return wm, nil
}

// Delete is used to delete a secure variable
func (sv *SecureVariables) Delete(path string, qo *WriteOptions) (*WriteMeta, error) {

	path = cleanPathString(path)
	wm, err := sv.deleteInternal(path, qo)
	if err != nil {
		return nil, err
	}
	return wm, nil
}

// CheckedDelete is used to conditionally delete a secure variable. If the
// existing variable does not match the provided checkIndex, it will return an
// ErrCASConflict that can be unwrapped for more details.
func (sv *SecureVariables) CheckedDelete(path string, checkIndex uint64, qo *WriteOptions) (*WriteMeta, error) {

	path = cleanPathString(path)
	wm, err := sv.deleteChecked(path, checkIndex, qo)
	if err != nil {
		return nil, err
	}

	return wm, nil
}

// List is used to dump all of the secure variables, can be used to pass prefix
// via QueryOptions rather than as a parameter
func (sv *SecureVariables) List(qo *QueryOptions) ([]*SecureVariableMetadata, *QueryMeta, error) {

	var resp []*SecureVariableMetadata
	qm, err := sv.client.query("/v1/vars", &resp, qo)
	if err != nil {
		return nil, nil, err
	}
	return resp, qm, nil
}

// PrefixList is used to do a prefix List search over secure variables.
func (sv *SecureVariables) PrefixList(prefix string, qo *QueryOptions) ([]*SecureVariableMetadata, *QueryMeta, error) {

	if qo == nil {
		qo = &QueryOptions{Prefix: prefix}
	} else {
		qo.Prefix = prefix
	}

	return sv.List(qo)
}

// GetItems returns the inner Items collection from a secure variable at a
// given path
func (sv *SecureVariables) GetItems(path string, qo *QueryOptions) (*SecureVariableItems, *QueryMeta, error) {

	path = cleanPathString(path)
	svar := new(SecureVariable)

	qm, err := sv.readInternal("/v1/var/"+path, &svar, qo)
	if err != nil {
		return nil, nil, err
	}

	return &svar.Items, qm, nil
}

// readInternal exists because the API's higher-level read method requires
// the status code to be 200 (OK). For Peek(), we do not consider 404
// (Not Found) an error.
func (sv *SecureVariables) readInternal(endpoint string, out **SecureVariable, q *QueryOptions) (*QueryMeta, error) {

	r, err := sv.client.newRequest("GET", endpoint)
	if err != nil {
		return nil, err
	}
	r.setQueryOptions(q)

	checkFn := requireStatusIn(http.StatusOK, http.StatusNotFound)
	rtt, resp, err := checkFn(sv.client.doRequest(r))
	if err != nil {
		return nil, err
	}

	qm := &QueryMeta{}
	parseQueryMeta(resp, qm)
	qm.RequestTime = rtt

	if resp.StatusCode == http.StatusNotFound {
		*out = nil
		resp.Body.Close()
		return qm, nil
	}

	defer resp.Body.Close()
	if err := decodeBody(resp, out); err != nil {
		return nil, err
	}

	return qm, nil
}

// readInternal exists because the API's higher-level delete method requires
// the status code to be 200 (OK). The SV HTTP API returns a 204 (No Content)
// on success.
func (sv *SecureVariables) deleteInternal(path string, q *WriteOptions) (*WriteMeta, error) {

	r, err := sv.client.newRequest("DELETE", fmt.Sprintf("/v1/var/%s", path))
	if err != nil {
		return nil, err
	}
	r.setWriteOptions(q)

	checkFn := requireStatusIn(http.StatusOK, http.StatusNoContent)
	rtt, resp, err := checkFn(sv.client.doRequest(r))

	if err != nil {
		return nil, err
	}

	wm := &WriteMeta{RequestTime: rtt}
	parseWriteMeta(resp, wm)
	return wm, nil
}

// deleteChecked exists because the API's higher-level delete method requires
// the status code to be OK. The SV HTTP API returns a 204 (No Content) on
// success and a 409 (Conflict) on a CAS error.
func (sv *SecureVariables) deleteChecked(path string, checkIndex uint64, q *WriteOptions) (*WriteMeta, error) {

	r, err := sv.client.newRequest("DELETE", fmt.Sprintf("/v1/var/%s?cas=%v", path, checkIndex))
	if err != nil {
		return nil, err
	}
	r.setWriteOptions(q)
	checkFn := requireStatusIn(http.StatusOK, http.StatusNoContent, http.StatusConflict)
	rtt, resp, err := checkFn(sv.client.doRequest(r))
	if err != nil {
		return nil, err
	}

	wm := &WriteMeta{RequestTime: rtt}
	parseWriteMeta(resp, wm)

	// The only reason we should decode the response body is if
	// it is a conflict response. Otherwise, there won't be one.
	if resp.StatusCode == http.StatusConflict {

		conflict := new(SecureVariable)
		if err := decodeBody(resp, &conflict); err != nil {
			return nil, err
		}
		return nil, ErrCASConflict{
			Conflict:   conflict,
			CheckIndex: checkIndex,
		}
	}
	return wm, nil
}

// writeChecked exists because the API's higher-level write method requires
// the status code to be OK. The SV HTTP API returns a 200 (OK) on
// success and a 409 (Conflict) on a CAS error.
func (sv *SecureVariables) writeChecked(endpoint string, in *SecureVariable, out *SecureVariable, q *WriteOptions) (*WriteMeta, error) {

	r, err := sv.client.newRequest("PUT", endpoint)
	if err != nil {
		return nil, err
	}
	r.setWriteOptions(q)
	r.obj = in

	checkFn := requireStatusIn(http.StatusOK, http.StatusNoContent, http.StatusConflict)
	rtt, resp, err := checkFn(sv.client.doRequest(r))

	if err != nil {
		return nil, err
	}

	wm := &WriteMeta{RequestTime: rtt}
	parseWriteMeta(resp, wm)

	if resp.StatusCode == http.StatusConflict {

		conflict := new(SecureVariable)
		if err := decodeBody(resp, &conflict); err != nil {
			return nil, err
		}
		return nil, ErrCASConflict{
			Conflict:   conflict,
			CheckIndex: in.ModifyIndex,
		}
	}
	return wm, nil
}

// SecureVariable specifies the metadata and contents to be stored in the
// encrypted Nomad backend.
type SecureVariable struct {
	// Namespace is the Nomad namespace associated with the secure variable
	Namespace string
	// Path is the path to the secure variable
	Path string

	// Raft indexes to track creation and modification
	CreateIndex uint64
	ModifyIndex uint64

	// Times provided as a convenience for operators expressed time.UnixNanos
	CreateTime int64
	ModifyTime int64

	Items SecureVariableItems
}

// SecureVariableMetadata specifies the metadata for a secure variable and
// is used as the list object
type SecureVariableMetadata struct {
	// Namespace is the Nomad namespace associated with the secure variable
	Namespace string
	// Path is the path to the secure variable
	Path string

	// Raft indexes to track creation and modification
	CreateIndex uint64
	ModifyIndex uint64

	// Times provided as a convenience for operators expressed time.UnixNanos
	CreateTime int64
	ModifyTime int64
}

type SecureVariableItems map[string]string

// NewSecureVariable is a convenience method to more easily create a
// ready-to-use secure variable
func NewSecureVariable(path string) *SecureVariable {

	return &SecureVariable{
		Path:  path,
		Items: make(SecureVariableItems),
	}
}

// Copy returns a new deep copy of this SecureVariable
func (sv1 *SecureVariable) Copy() *SecureVariable {

	var out SecureVariable = *sv1
	out.Items = make(SecureVariableItems)
	for k, v := range sv1.Items {
		out.Items[k] = v
	}
	return &out
}

// Metadata returns the SecureVariableMetadata component of
// a SecureVariable. This can be useful for comparing against
// a List result.
func (sv *SecureVariable) Metadata() *SecureVariableMetadata {

	return &SecureVariableMetadata{
		Namespace:   sv.Namespace,
		Path:        sv.Path,
		CreateIndex: sv.CreateIndex,
		ModifyIndex: sv.ModifyIndex,
		CreateTime:  sv.CreateTime,
		ModifyTime:  sv.ModifyTime,
	}
}

// IsZeroValue can be used to test if a SecureVariable has been changed
// from the default values it gets at creation
func (sv *SecureVariable) IsZeroValue() bool {
	return *sv.Metadata() == SecureVariableMetadata{} && sv.Items == nil
}

// cleanPathString removes leading and trailing slashes since they
// would trigger go's path cleaning/redirection behavior in the
// standard HTTP router
func cleanPathString(path string) string {
	return strings.Trim(path, " /")
}

// AsJSON returns the SecureVariable as a JSON-formatted string
func (sv SecureVariable) AsJSON() string {
	var b []byte
	b, _ = json.Marshal(sv)
	return string(b)
}

// AsPrettyJSON returns the SecureVariable as a JSON-formatted string with
// indentation
func (sv SecureVariable) AsPrettyJSON() string {
	var b []byte
	b, _ = json.MarshalIndent(sv, "", "  ")
	return string(b)
}

type ErrCASConflict struct {
	CheckIndex uint64
	Conflict   *SecureVariable
}

func (e ErrCASConflict) Error() string {
	return fmt.Sprintf("cas conflict: expected ModifyIndex %v; found %v", e.CheckIndex, e.Conflict.ModifyIndex)
}

// doRequestWrapper is a function that wraps the client's doRequest method
// and can be used to provide error and response handling
type doRequestWrapper = func(time.Duration, *http.Response, error) (time.Duration, *http.Response, error)

// requireStatusIn is a doRequestWrapper generator that takes expected HTTP
// response codes and validates that the received response code is among them
func requireStatusIn(statuses ...int) doRequestWrapper {
	fn := func(d time.Duration, resp *http.Response, e error) (time.Duration, *http.Response, error) {
		statuses := statuses
		if e != nil {
			if resp != nil {
				resp.Body.Close()
			}
			return d, nil, e
		}

		for _, status := range statuses {
			if resp.StatusCode == status {
				return d, resp, nil
			}
		}

		return d, nil, generateUnexpectedResponseCodeError(resp)
	}
	return fn
}

// generateUnexpectedResponseCodeError creates a standardized error
// when the the API client's newRequest method receives an unexpected
// HTTP response code when accessing the secure variable's HTTP API
func generateUnexpectedResponseCodeError(resp *http.Response) error {
	var buf bytes.Buffer
	io.Copy(&buf, resp.Body)
	resp.Body.Close()
	return fmt.Errorf("Unexpected response code: %d (%s)", resp.StatusCode, buf.Bytes())
}
