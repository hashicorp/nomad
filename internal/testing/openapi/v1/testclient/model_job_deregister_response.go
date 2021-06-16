/*
 * nomad
 *
 * <h1 id=\"http-api\">HTTP API</h1> <p>The main interface to Nomad is a RESTful HTTP API. The API can query the current     state of the system as well as modify the state of the system. The Nomad CLI     actually invokes Nomad&#39;s HTTP for many commands.</p> <h2 id=\"version-prefix\">Version Prefix</h2> <p>All API routes are prefixed with <code>/v1/</code>.</p> <h2 id=\"addressing-and-ports\">Addressing and Ports</h2> <p>Nomad binds to a specific set of addresses and ports. The HTTP API is served via     the <code>http</code> address and port. This <code>address:port</code> must be accessible locally. If     you bind to <code>127.0.0.1:4646</code>, the API is only available <em>from that host</em>. If you     bind to a private internal IP, the API will be available from within that     network. If you bind to a public IP, the API will be available from the public     Internet (not recommended).</p> <p>The default port for the Nomad HTTP API is <code>4646</code>. This can be overridden via     the Nomad configuration block. Here is an example curl request to query a Nomad     server with the default configuration:</p> <pre><code class=\"language-shell-session\">$ curl http://127.0.0.1:4646/v1/agent/members </code></pre> <p>The conventions used in the API documentation do not list a port and use the     standard URL <code>localhost:4646</code>. Be sure to replace this with your Nomad agent URL     when using the examples.</p> <h2 id=\"data-model-and-layout\">Data Model and Layout</h2> <p>There are five primary nouns in Nomad:</p> <ul>     <li>jobs</li>     <li>nodes</li>     <li>allocations</li>     <li>deployments</li>     <li>evaluations</li> </ul> <p><a href=\"/img/nomad-data-model.png\"><img src=\"/img/nomad-data-model.png\" alt=\"Nomad Data Model\"></a></p> <p>Jobs are submitted by users and represent a <em>desired state</em>. A job is a     declarative description of tasks to run which are bounded by constraints and     require resources. Jobs can also have affinities which are used to express placement     preferences. Nodes are the servers in the clusters that tasks can be     scheduled on. The mapping of tasks in a job to nodes is done using allocations.     An allocation is used to declare that a set of tasks in a job should be run on a     particular node. Scheduling is the process of determining the appropriate     allocations and is done as part of an evaluation. Deployments are objects to     track a rolling update of allocations between two versions of a job.</p> <p>The API is modeled closely on the underlying data model. Use the links to the     left for documentation about specific endpoints. There are also &quot;Agent&quot; APIs     which interact with a specific agent and not the broader cluster used for     administration.</p> <h2 id=\"acls\">ACLs</h2> <p>Several endpoints in Nomad use or require ACL tokens to operate. The token are used to authenticate the request and determine if the request is allowed based on the associated authorizations. Tokens are specified per-request by using the <code>X-Nomad-Token</code> request header set to the <code>SecretID</code> of an ACL Token.</p> <p>For more details about ACLs, please see the <a href=\"https://learn.hashicorp.com/collections/nomad/access-control\">ACL Guide</a>.</p> <h2 id=\"authentication\">Authentication</h2> <p>When ACLs are enabled, a Nomad token should be provided to API requests using the <code>X-Nomad-Token</code> header. When using authentication, clients should communicate via TLS.</p> <p>Here is an example using curl:</p> <pre><code class=\"language-shell-session\">$ curl \\     --header &quot;X-Nomad-Token: aa534e09-6a07-0a45-2295-a7f77063d429&quot; \\     https://localhost:4646/v1/jobs </code></pre> <h2 id=\"namespaces\">Namespaces</h2> <p>Nomad has support for namespaces, which allow jobs and their associated objects     to be segmented from each other and other users of the cluster. When using     non-default namespace, the API request must pass the target namespace as an API     query parameter. Prior to Nomad 1.0 namespaces were Enterprise-only.</p> <p>Here is an example using curl:</p> <pre><code class=\"language-shell-session\">$ curl \\     https://localhost:4646/v1/jobs?namespace=qa </code></pre> <h2 id=\"blocking-queries\">Blocking Queries</h2> <p>Many endpoints in Nomad support a feature known as &quot;blocking queries&quot;. A     blocking query is used to wait for a potential change using long polling. Not     all endpoints support blocking, but each endpoint uniquely documents its support     for blocking queries in the documentation.</p> <p>Endpoints that support blocking queries return an HTTP header named     <code>X-Nomad-Index</code>. This is a unique identifier representing the current state of     the requested resource. On a new Nomad cluster the value of this index starts at 1. </p> <p>On subsequent requests for this resource, the client can set the <code>index</code> query     string parameter to the value of <code>X-Nomad-Index</code>, indicating that the client     wishes to wait for any changes subsequent to that index.</p> <p>When this is provided, the HTTP request will &quot;hang&quot; until a change in the system     occurs, or the maximum timeout is reached. A critical note is that the return of     a blocking request is <strong>no guarantee</strong> of a change. It is possible that the     timeout was reached or that there was an idempotent write that does not affect     the result of the query.</p> <p>In addition to <code>index</code>, endpoints that support blocking will also honor a <code>wait</code>     parameter specifying a maximum duration for the blocking request. This is     limited to 10 minutes. If not set, the wait time defaults to 5 minutes. This     value can be specified in the form of &quot;10s&quot; or &quot;5m&quot; (i.e., 10 seconds or 5     minutes, respectively). A small random amount of additional wait time is added     to the supplied maximum <code>wait</code> time to spread out the wake up time of any     concurrent requests. This adds up to <code>wait / 16</code> additional time to the maximum     duration.</p> <h2 id=\"consistency-modes\">Consistency Modes</h2> <p>Most of the read query endpoints support multiple levels of consistency. Since     no policy will suit all clients&#39; needs, these consistency modes allow the user     to have the ultimate say in how to balance the trade-offs inherent in a     distributed system.</p> <p>The two read modes are:</p> <ul>     <li>         <p><code>default</code> - If not specified, the default is strongly consistent in almost all             cases. However, there is a small window in which a new leader may be elected             during which the old leader may service stale values. The trade-off is fast             reads but potentially stale values. The condition resulting in stale reads is             hard to trigger, and most clients should not need to worry about this case.             Also, note that this race condition only applies to reads, not writes.</p>     </li>     <li>         <p><code>stale</code> - This mode allows any server to service the read regardless of             whether it is the leader. This means reads can be arbitrarily stale; however,             results are generally consistent to within 50 milliseconds of the leader. The             trade-off is very fast and scalable reads with a higher likelihood of stale             values. Since this mode allows reads without a leader, a cluster that is             unavailable will still be able to respond to queries.</p>     </li> </ul> <p>To switch these modes, use the <code>stale</code> query parameter on requests.</p> <p>To support bounding the acceptable staleness of data, responses provide the     <code>X-Nomad-LastContact</code> header containing the time in milliseconds that a server     was last contacted by the leader node. The <code>X-Nomad-KnownLeader</code> header also     indicates if there is a known leader. These can be used by clients to gauge the     staleness of a result and take appropriate action. </p> <h2 id=\"cross-region-requests\">Cross-Region Requests</h2> <p>By default, any request to the HTTP API will default to the region on which the     machine is servicing the request. If the agent runs in &quot;region1&quot;, the request     will query the region &quot;region1&quot;. A target region can be explicitly request using     the <code>?region</code> query parameter. The request will be transparently forwarded and     serviced by a server in the requested region.</p> <h2 id=\"compressed-responses\">Compressed Responses</h2> <p>The HTTP API will gzip the response if the HTTP request denotes that the client     accepts gzip compression. This is achieved by passing the accept encoding:</p> <pre><code class=\"language-shell-session\">$ curl \\     --header &quot;Accept-Encoding: gzip&quot; \\     https://localhost:4646/v1/... </code></pre> <h2 id=\"formatted-json-output\">Formatted JSON Output</h2> <p>By default, the output of all HTTP API requests is minimized JSON. If the client     passes <code>pretty</code> on the query string, formatted JSON will be returned.</p> <p>In general, clients should prefer a client-side parser like <code>jq</code> instead of     server-formatted data. Asking the server to format the data takes away     processing cycles from more important tasks.</p> <pre><code class=\"language-shell-session\">$ curl https://localhost:4646/v1/page?pretty </code></pre> <h2 id=\"http-methods\">HTTP Methods</h2> <p>Nomad&#39;s API aims to be RESTful, although there are some exceptions. The API     responds to the standard HTTP verbs GET, PUT, and DELETE. Each API method will     clearly document the verb(s) it responds to and the generated response. The same     path with different verbs may trigger different behavior. For example:</p> <pre><code class=\"language-text\">PUT /v1/jobs GET /v1/jobs </code></pre> <p>Even though these share a path, the <code>PUT</code> operation creates a new job whereas     the <code>GET</code> operation reads all jobs.</p> <h2 id=\"http-response-codes\">HTTP Response Codes</h2> <p>Individual API&#39;s will contain further documentation in the case that more     specific response codes are returned but all clients should handle the following:</p> <ul>     <li>200 and 204 as success codes.</li>     <li>400 indicates a validation failure and if a parameter is modified in the         request, it could potentially succeed.</li>     <li>403 marks that the client isn&#39;t authenticated for the request.</li>     <li>404 indicates an unknown resource.</li>     <li>5xx means that the client should not expect the request to succeed if retried.</li> </ul>
 *
 * API version: 1.1.0
 * Contact: support@hashicorp.com
 */

// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

package testclient

import (
	"encoding/json"
)

// JobDeregisterResponse JobDeregisterResponse is used to respond to a job de-registration
type JobDeregisterResponse struct {
	EvalCreateIndex *int32 `json:"EvalCreateIndex,omitempty"`
	EvalID *string `json:"EvalID,omitempty"`
	JobModifyIndex *int32 `json:"JobModifyIndex,omitempty"`
	// Is there a known leader
	KnownLeader *bool `json:"KnownLeader,omitempty"`
	// A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years.
	LastContact *int64 `json:"LastContact,omitempty"`
	// LastIndex. This can be used as a WaitIndex to perform a blocking query
	LastIndex *int32 `json:"LastIndex,omitempty"`
	// A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years.
	RequestTime *int64 `json:"RequestTime,omitempty"`
}

// NewJobDeregisterResponse instantiates a new JobDeregisterResponse object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewJobDeregisterResponse() *JobDeregisterResponse {
	this := JobDeregisterResponse{}
	return &this
}

// NewJobDeregisterResponseWithDefaults instantiates a new JobDeregisterResponse object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewJobDeregisterResponseWithDefaults() *JobDeregisterResponse {
	this := JobDeregisterResponse{}
	return &this
}

// GetEvalCreateIndex returns the EvalCreateIndex field value if set, zero value otherwise.
func (o *JobDeregisterResponse) GetEvalCreateIndex() int32 {
	if o == nil || o.EvalCreateIndex == nil {
		var ret int32
		return ret
	}
	return *o.EvalCreateIndex
}

// GetEvalCreateIndexOk returns a tuple with the EvalCreateIndex field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *JobDeregisterResponse) GetEvalCreateIndexOk() (*int32, bool) {
	if o == nil || o.EvalCreateIndex == nil {
		return nil, false
	}
	return o.EvalCreateIndex, true
}

// HasEvalCreateIndex returns a boolean if a field has been set.
func (o *JobDeregisterResponse) HasEvalCreateIndex() bool {
	if o != nil && o.EvalCreateIndex != nil {
		return true
	}

	return false
}

// SetEvalCreateIndex gets a reference to the given int32 and assigns it to the EvalCreateIndex field.
func (o *JobDeregisterResponse) SetEvalCreateIndex(v int32) {
	o.EvalCreateIndex = &v
}

// GetEvalID returns the EvalID field value if set, zero value otherwise.
func (o *JobDeregisterResponse) GetEvalID() string {
	if o == nil || o.EvalID == nil {
		var ret string
		return ret
	}
	return *o.EvalID
}

// GetEvalIDOk returns a tuple with the EvalID field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *JobDeregisterResponse) GetEvalIDOk() (*string, bool) {
	if o == nil || o.EvalID == nil {
		return nil, false
	}
	return o.EvalID, true
}

// HasEvalID returns a boolean if a field has been set.
func (o *JobDeregisterResponse) HasEvalID() bool {
	if o != nil && o.EvalID != nil {
		return true
	}

	return false
}

// SetEvalID gets a reference to the given string and assigns it to the EvalID field.
func (o *JobDeregisterResponse) SetEvalID(v string) {
	o.EvalID = &v
}

// GetJobModifyIndex returns the JobModifyIndex field value if set, zero value otherwise.
func (o *JobDeregisterResponse) GetJobModifyIndex() int32 {
	if o == nil || o.JobModifyIndex == nil {
		var ret int32
		return ret
	}
	return *o.JobModifyIndex
}

// GetJobModifyIndexOk returns a tuple with the JobModifyIndex field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *JobDeregisterResponse) GetJobModifyIndexOk() (*int32, bool) {
	if o == nil || o.JobModifyIndex == nil {
		return nil, false
	}
	return o.JobModifyIndex, true
}

// HasJobModifyIndex returns a boolean if a field has been set.
func (o *JobDeregisterResponse) HasJobModifyIndex() bool {
	if o != nil && o.JobModifyIndex != nil {
		return true
	}

	return false
}

// SetJobModifyIndex gets a reference to the given int32 and assigns it to the JobModifyIndex field.
func (o *JobDeregisterResponse) SetJobModifyIndex(v int32) {
	o.JobModifyIndex = &v
}

// GetKnownLeader returns the KnownLeader field value if set, zero value otherwise.
func (o *JobDeregisterResponse) GetKnownLeader() bool {
	if o == nil || o.KnownLeader == nil {
		var ret bool
		return ret
	}
	return *o.KnownLeader
}

// GetKnownLeaderOk returns a tuple with the KnownLeader field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *JobDeregisterResponse) GetKnownLeaderOk() (*bool, bool) {
	if o == nil || o.KnownLeader == nil {
		return nil, false
	}
	return o.KnownLeader, true
}

// HasKnownLeader returns a boolean if a field has been set.
func (o *JobDeregisterResponse) HasKnownLeader() bool {
	if o != nil && o.KnownLeader != nil {
		return true
	}

	return false
}

// SetKnownLeader gets a reference to the given bool and assigns it to the KnownLeader field.
func (o *JobDeregisterResponse) SetKnownLeader(v bool) {
	o.KnownLeader = &v
}

// GetLastContact returns the LastContact field value if set, zero value otherwise.
func (o *JobDeregisterResponse) GetLastContact() int64 {
	if o == nil || o.LastContact == nil {
		var ret int64
		return ret
	}
	return *o.LastContact
}

// GetLastContactOk returns a tuple with the LastContact field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *JobDeregisterResponse) GetLastContactOk() (*int64, bool) {
	if o == nil || o.LastContact == nil {
		return nil, false
	}
	return o.LastContact, true
}

// HasLastContact returns a boolean if a field has been set.
func (o *JobDeregisterResponse) HasLastContact() bool {
	if o != nil && o.LastContact != nil {
		return true
	}

	return false
}

// SetLastContact gets a reference to the given int64 and assigns it to the LastContact field.
func (o *JobDeregisterResponse) SetLastContact(v int64) {
	o.LastContact = &v
}

// GetLastIndex returns the LastIndex field value if set, zero value otherwise.
func (o *JobDeregisterResponse) GetLastIndex() int32 {
	if o == nil || o.LastIndex == nil {
		var ret int32
		return ret
	}
	return *o.LastIndex
}

// GetLastIndexOk returns a tuple with the LastIndex field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *JobDeregisterResponse) GetLastIndexOk() (*int32, bool) {
	if o == nil || o.LastIndex == nil {
		return nil, false
	}
	return o.LastIndex, true
}

// HasLastIndex returns a boolean if a field has been set.
func (o *JobDeregisterResponse) HasLastIndex() bool {
	if o != nil && o.LastIndex != nil {
		return true
	}

	return false
}

// SetLastIndex gets a reference to the given int32 and assigns it to the LastIndex field.
func (o *JobDeregisterResponse) SetLastIndex(v int32) {
	o.LastIndex = &v
}

// GetRequestTime returns the RequestTime field value if set, zero value otherwise.
func (o *JobDeregisterResponse) GetRequestTime() int64 {
	if o == nil || o.RequestTime == nil {
		var ret int64
		return ret
	}
	return *o.RequestTime
}

// GetRequestTimeOk returns a tuple with the RequestTime field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *JobDeregisterResponse) GetRequestTimeOk() (*int64, bool) {
	if o == nil || o.RequestTime == nil {
		return nil, false
	}
	return o.RequestTime, true
}

// HasRequestTime returns a boolean if a field has been set.
func (o *JobDeregisterResponse) HasRequestTime() bool {
	if o != nil && o.RequestTime != nil {
		return true
	}

	return false
}

// SetRequestTime gets a reference to the given int64 and assigns it to the RequestTime field.
func (o *JobDeregisterResponse) SetRequestTime(v int64) {
	o.RequestTime = &v
}

func (o JobDeregisterResponse) MarshalJSON() ([]byte, error) {
	toSerialize := map[string]interface{}{}
	if o.EvalCreateIndex != nil {
		toSerialize["EvalCreateIndex"] = o.EvalCreateIndex
	}
	if o.EvalID != nil {
		toSerialize["EvalID"] = o.EvalID
	}
	if o.JobModifyIndex != nil {
		toSerialize["JobModifyIndex"] = o.JobModifyIndex
	}
	if o.KnownLeader != nil {
		toSerialize["KnownLeader"] = o.KnownLeader
	}
	if o.LastContact != nil {
		toSerialize["LastContact"] = o.LastContact
	}
	if o.LastIndex != nil {
		toSerialize["LastIndex"] = o.LastIndex
	}
	if o.RequestTime != nil {
		toSerialize["RequestTime"] = o.RequestTime
	}
	return json.Marshal(toSerialize)
}

type NullableJobDeregisterResponse struct {
	value *JobDeregisterResponse
	isSet bool
}

func (v NullableJobDeregisterResponse) Get() *JobDeregisterResponse {
	return v.value
}

func (v *NullableJobDeregisterResponse) Set(val *JobDeregisterResponse) {
	v.value = val
	v.isSet = true
}

func (v NullableJobDeregisterResponse) IsSet() bool {
	return v.isSet
}

func (v *NullableJobDeregisterResponse) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableJobDeregisterResponse(val *JobDeregisterResponse) *NullableJobDeregisterResponse {
	return &NullableJobDeregisterResponse{value: val, isSet: true}
}

func (v NullableJobDeregisterResponse) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableJobDeregisterResponse) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}


