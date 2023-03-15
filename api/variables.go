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
	ErrVariableNotFound     = "variable not found"
	ErrVariableMissingItems = "variable missing Items field"
)

// Variables is used to access variables.
type Variables struct {
	client *Client
}

// Variables returns a new handle on the variables.
func (c *Client) Variables() *Variables {
	return &Variables{client: c}
}

// Create is used to create a variable.
func (sv *Variables) Create(v *Variable, qo *WriteOptions) (*Variable, *WriteMeta, error) {

	v.Path = cleanPathString(v.Path)
	var out Variable
	wm, err := sv.client.write("/v1/var/"+v.Path, v, &out, qo)
	if err != nil {
		return nil, wm, err
	}
	return &out, wm, nil
}

// CheckedCreate is used to create a variable if it doesn't exist
// already. If it does, it will return a ErrCASConflict that can be unwrapped
// for more details.
func (sv *Variables) CheckedCreate(v *Variable, qo *WriteOptions) (*Variable, *WriteMeta, error) {

	v.Path = cleanPathString(v.Path)
	var out Variable
	wm, err := sv.writeChecked("/v1/var/"+v.Path+"?cas=0", v, &out, qo)
	if err != nil {
		return nil, wm, err
	}

	return &out, wm, nil
}

// Read is used to query a single variable by path. This will error
// if the variable is not found.
func (sv *Variables) Read(path string, qo *QueryOptions) (*Variable, *QueryMeta, error) {

	path = cleanPathString(path)
	var svar = new(Variable)
	qm, err := sv.readInternal("/v1/var/"+path, &svar, qo)
	if err != nil {
		return nil, nil, err
	}
	if svar == nil {
		return nil, qm, errors.New(ErrVariableNotFound)
	}
	return svar, qm, nil
}

// Peek is used to query a single variable by path, but does not error
// when the variable is not found
func (sv *Variables) Peek(path string, qo *QueryOptions) (*Variable, *QueryMeta, error) {

	path = cleanPathString(path)
	var svar = new(Variable)
	qm, err := sv.readInternal("/v1/var/"+path, &svar, qo)
	if err != nil {
		return nil, nil, err
	}
	return svar, qm, nil
}

// Update is used to update a variable.
func (sv *Variables) Update(v *Variable, qo *WriteOptions) (*Variable, *WriteMeta, error) {

	v.Path = cleanPathString(v.Path)
	var out Variable

	wm, err := sv.client.write("/v1/var/"+v.Path, v, &out, qo)
	if err != nil {
		return nil, wm, err
	}
	return &out, wm, nil
}

// CheckedUpdate is used to updated a variable if the modify index
// matches the one on the server.  If it does not, it will return an
// ErrCASConflict that can be unwrapped for more details.
func (sv *Variables) CheckedUpdate(v *Variable, qo *WriteOptions) (*Variable, *WriteMeta, error) {

	v.Path = cleanPathString(v.Path)
	var out Variable
	wm, err := sv.writeChecked("/v1/var/"+v.Path+"?cas="+fmt.Sprint(v.ModifyIndex), v, &out, qo)
	if err != nil {
		return nil, wm, err
	}

	return &out, wm, nil
}

// Delete is used to delete a variable
func (sv *Variables) Delete(path string, qo *WriteOptions) (*WriteMeta, error) {

	path = cleanPathString(path)
	wm, err := sv.deleteInternal(path, qo)
	if err != nil {
		return nil, err
	}
	return wm, nil
}

// CheckedDelete is used to conditionally delete a variable. If the
// existing variable does not match the provided checkIndex, it will return an
// ErrCASConflict that can be unwrapped for more details.
func (sv *Variables) CheckedDelete(path string, checkIndex uint64, qo *WriteOptions) (*WriteMeta, error) {

	path = cleanPathString(path)
	wm, err := sv.deleteChecked(path, checkIndex, qo)
	if err != nil {
		return nil, err
	}

	return wm, nil
}

// List is used to dump all of the variables, can be used to pass prefix
// via QueryOptions rather than as a parameter
func (sv *Variables) List(qo *QueryOptions) ([]*VariableMetadata, *QueryMeta, error) {

	var resp []*VariableMetadata
	qm, err := sv.client.query("/v1/vars", &resp, qo)
	if err != nil {
		return nil, nil, err
	}
	return resp, qm, nil
}

// PrefixList is used to do a prefix List search over variables.
func (sv *Variables) PrefixList(prefix string, qo *QueryOptions) ([]*VariableMetadata, *QueryMeta, error) {

	if qo == nil {
		qo = &QueryOptions{Prefix: prefix}
	} else {
		qo.Prefix = prefix
	}

	return sv.List(qo)
}

// GetItems returns the inner Items collection from a variable at a
// given path
func (sv *Variables) GetItems(path string, qo *QueryOptions) (*VariableItems, *QueryMeta, error) {

	path = cleanPathString(path)
	svar := new(Variable)

	qm, err := sv.readInternal("/v1/var/"+path, &svar, qo)
	if err != nil {
		return nil, nil, err
	}

	return &svar.Items, qm, nil
}

// readInternal exists because the API's higher-level read method requires
// the status code to be 200 (OK). For Peek(), we do not consider 403 (Permission
// Denied or 404 (Not Found) an error, this function just returns a nil in those
// cases.
func (sv *Variables) readInternal(endpoint string, out **Variable, q *QueryOptions) (*QueryMeta, error) {

	r, err := sv.client.newRequest("GET", endpoint)
	if err != nil {
		return nil, err
	}
	r.setQueryOptions(q)

	checkFn := requireStatusIn(http.StatusOK, http.StatusNotFound, http.StatusForbidden)
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

	if resp.StatusCode == http.StatusForbidden {
		*out = nil
		resp.Body.Close()
		// On a 403, there is no QueryMeta to parse, but consul-template--the
		// main consumer of the Peek() func that calls this method needs the
		// value to be non-zero; so set them to a reasonable but artificial
		// value. Index 1 doesn't say anything about the cluster, and there
		// has to be a KnownLeader to get a 403.
		qm.LastIndex = 1
		qm.KnownLeader = true
		return qm, nil
	}

	defer resp.Body.Close()
	if err := decodeBody(resp, out); err != nil {
		return nil, err
	}

	return qm, nil
}

// deleteInternal exists because the API's higher-level delete method requires
// the status code to be 200 (OK). The SV HTTP API returns a 204 (No Content)
// on success.
func (sv *Variables) deleteInternal(path string, q *WriteOptions) (*WriteMeta, error) {

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
func (sv *Variables) deleteChecked(path string, checkIndex uint64, q *WriteOptions) (*WriteMeta, error) {

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

		conflict := new(Variable)
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
func (sv *Variables) writeChecked(endpoint string, in *Variable, out *Variable, q *WriteOptions) (*WriteMeta, error) {

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
	defer resp.Body.Close()

	wm := &WriteMeta{RequestTime: rtt}
	parseWriteMeta(resp, wm)

	if resp.StatusCode == http.StatusConflict {

		conflict := new(Variable)
		if err := decodeBody(resp, &conflict); err != nil {
			return nil, err
		}
		return nil, ErrCASConflict{
			Conflict:   conflict,
			CheckIndex: in.ModifyIndex,
		}
	}
	if out != nil {
		if err := decodeBody(resp, &out); err != nil {
			return nil, err
		}
	}
	return wm, nil
}

// Variable specifies the metadata and contents to be stored in the
// encrypted Nomad backend.
type Variable struct {
	// Namespace is the Nomad namespace associated with the variable
	Namespace string `hcl:"namespace"`
	// Path is the path to the variable
	Path string `hcl:"path"`

	// Raft indexes to track creation and modification
	CreateIndex uint64 `hcl:"create_index"`
	ModifyIndex uint64 `hcl:"modify_index"`

	// Times provided as a convenience for operators expressed time.UnixNanos
	CreateTime int64 `hcl:"create_time"`
	ModifyTime int64 `hcl:"modify_time"`

	Items VariableItems `hcl:"items"`
}

// VariableMetadata specifies the metadata for a variable and
// is used as the list object
type VariableMetadata struct {
	// Namespace is the Nomad namespace associated with the variable
	Namespace string `hcl:"namespace"`
	// Path is the path to the variable
	Path string `hcl:"path"`

	// Raft indexes to track creation and modification
	CreateIndex uint64 `hcl:"create_index"`
	ModifyIndex uint64 `hcl:"modify_index"`

	// Times provided as a convenience for operators expressed time.UnixNanos
	CreateTime int64 `hcl:"create_time"`
	ModifyTime int64 `hcl:"modify_time"`
}

type VariableItems map[string]string

// NewVariable is a convenience method to more easily create a
// ready-to-use variable
func NewVariable(path string) *Variable {

	return &Variable{
		Path:  path,
		Items: make(VariableItems),
	}
}

// Copy returns a new deep copy of this Variable
func (sv1 *Variable) Copy() *Variable {

	var out Variable = *sv1
	out.Items = make(VariableItems)
	for k, v := range sv1.Items {
		out.Items[k] = v
	}
	return &out
}

// Metadata returns the VariableMetadata component of
// a Variable. This can be useful for comparing against
// a List result.
func (sv *Variable) Metadata() *VariableMetadata {

	return &VariableMetadata{
		Namespace:   sv.Namespace,
		Path:        sv.Path,
		CreateIndex: sv.CreateIndex,
		ModifyIndex: sv.ModifyIndex,
		CreateTime:  sv.CreateTime,
		ModifyTime:  sv.ModifyTime,
	}
}

// IsZeroValue can be used to test if a Variable has been changed
// from the default values it gets at creation
func (sv *Variable) IsZeroValue() bool {
	return *sv.Metadata() == VariableMetadata{} && sv.Items == nil
}

// cleanPathString removes leading and trailing slashes since they
// would trigger go's path cleaning/redirection behavior in the
// standard HTTP router
func cleanPathString(path string) string {
	return strings.Trim(path, " /")
}

// AsJSON returns the Variable as a JSON-formatted string
func (sv Variable) AsJSON() string {
	var b []byte
	b, _ = json.Marshal(sv)
	return string(b)
}

// AsPrettyJSON returns the Variable as a JSON-formatted string with
// indentation
func (sv Variable) AsPrettyJSON() string {
	var b []byte
	b, _ = json.MarshalIndent(sv, "", "  ")
	return string(b)
}

type ErrCASConflict struct {
	CheckIndex uint64
	Conflict   *Variable
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
// HTTP response code when accessing the variable's HTTP API
func generateUnexpectedResponseCodeError(resp *http.Response) error {
	var buf bytes.Buffer
	io.Copy(&buf, resp.Body)
	resp.Body.Close()
	return fmt.Errorf("Unexpected response code: %d (%s)", resp.StatusCode, buf.Bytes())
}
