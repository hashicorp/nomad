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

// SidecarTask SidecarTask represents a subset of Task fields that can be set to override the fields of the Task generated for the sidecar
type SidecarTask struct {
	Config *map[string]map[string]interface{} `json:"Config,omitempty"`
	Driver *string `json:"Driver,omitempty"`
	Env *map[string]string `json:"Env,omitempty"`
	KillSignal *string `json:"KillSignal,omitempty"`
	// A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years.
	KillTimeout *int64 `json:"KillTimeout,omitempty"`
	LogConfig *LogConfig `json:"LogConfig,omitempty"`
	Meta *map[string]string `json:"Meta,omitempty"`
	Name *string `json:"Name,omitempty"`
	Resources *Resources `json:"Resources,omitempty"`
	// A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years.
	ShutdownDelay *int64 `json:"ShutdownDelay,omitempty"`
	User *string `json:"User,omitempty"`
}

// NewSidecarTask instantiates a new SidecarTask object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewSidecarTask() *SidecarTask {
	this := SidecarTask{}
	return &this
}

// NewSidecarTaskWithDefaults instantiates a new SidecarTask object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewSidecarTaskWithDefaults() *SidecarTask {
	this := SidecarTask{}
	return &this
}

// GetConfig returns the Config field value if set, zero value otherwise.
func (o *SidecarTask) GetConfig() map[string]map[string]interface{} {
	if o == nil || o.Config == nil {
		var ret map[string]map[string]interface{}
		return ret
	}
	return *o.Config
}

// GetConfigOk returns a tuple with the Config field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *SidecarTask) GetConfigOk() (*map[string]map[string]interface{}, bool) {
	if o == nil || o.Config == nil {
		return nil, false
	}
	return o.Config, true
}

// HasConfig returns a boolean if a field has been set.
func (o *SidecarTask) HasConfig() bool {
	if o != nil && o.Config != nil {
		return true
	}

	return false
}

// SetConfig gets a reference to the given map[string]map[string]interface{} and assigns it to the Config field.
func (o *SidecarTask) SetConfig(v map[string]map[string]interface{}) {
	o.Config = &v
}

// GetDriver returns the Driver field value if set, zero value otherwise.
func (o *SidecarTask) GetDriver() string {
	if o == nil || o.Driver == nil {
		var ret string
		return ret
	}
	return *o.Driver
}

// GetDriverOk returns a tuple with the Driver field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *SidecarTask) GetDriverOk() (*string, bool) {
	if o == nil || o.Driver == nil {
		return nil, false
	}
	return o.Driver, true
}

// HasDriver returns a boolean if a field has been set.
func (o *SidecarTask) HasDriver() bool {
	if o != nil && o.Driver != nil {
		return true
	}

	return false
}

// SetDriver gets a reference to the given string and assigns it to the Driver field.
func (o *SidecarTask) SetDriver(v string) {
	o.Driver = &v
}

// GetEnv returns the Env field value if set, zero value otherwise.
func (o *SidecarTask) GetEnv() map[string]string {
	if o == nil || o.Env == nil {
		var ret map[string]string
		return ret
	}
	return *o.Env
}

// GetEnvOk returns a tuple with the Env field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *SidecarTask) GetEnvOk() (*map[string]string, bool) {
	if o == nil || o.Env == nil {
		return nil, false
	}
	return o.Env, true
}

// HasEnv returns a boolean if a field has been set.
func (o *SidecarTask) HasEnv() bool {
	if o != nil && o.Env != nil {
		return true
	}

	return false
}

// SetEnv gets a reference to the given map[string]string and assigns it to the Env field.
func (o *SidecarTask) SetEnv(v map[string]string) {
	o.Env = &v
}

// GetKillSignal returns the KillSignal field value if set, zero value otherwise.
func (o *SidecarTask) GetKillSignal() string {
	if o == nil || o.KillSignal == nil {
		var ret string
		return ret
	}
	return *o.KillSignal
}

// GetKillSignalOk returns a tuple with the KillSignal field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *SidecarTask) GetKillSignalOk() (*string, bool) {
	if o == nil || o.KillSignal == nil {
		return nil, false
	}
	return o.KillSignal, true
}

// HasKillSignal returns a boolean if a field has been set.
func (o *SidecarTask) HasKillSignal() bool {
	if o != nil && o.KillSignal != nil {
		return true
	}

	return false
}

// SetKillSignal gets a reference to the given string and assigns it to the KillSignal field.
func (o *SidecarTask) SetKillSignal(v string) {
	o.KillSignal = &v
}

// GetKillTimeout returns the KillTimeout field value if set, zero value otherwise.
func (o *SidecarTask) GetKillTimeout() int64 {
	if o == nil || o.KillTimeout == nil {
		var ret int64
		return ret
	}
	return *o.KillTimeout
}

// GetKillTimeoutOk returns a tuple with the KillTimeout field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *SidecarTask) GetKillTimeoutOk() (*int64, bool) {
	if o == nil || o.KillTimeout == nil {
		return nil, false
	}
	return o.KillTimeout, true
}

// HasKillTimeout returns a boolean if a field has been set.
func (o *SidecarTask) HasKillTimeout() bool {
	if o != nil && o.KillTimeout != nil {
		return true
	}

	return false
}

// SetKillTimeout gets a reference to the given int64 and assigns it to the KillTimeout field.
func (o *SidecarTask) SetKillTimeout(v int64) {
	o.KillTimeout = &v
}

// GetLogConfig returns the LogConfig field value if set, zero value otherwise.
func (o *SidecarTask) GetLogConfig() LogConfig {
	if o == nil || o.LogConfig == nil {
		var ret LogConfig
		return ret
	}
	return *o.LogConfig
}

// GetLogConfigOk returns a tuple with the LogConfig field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *SidecarTask) GetLogConfigOk() (*LogConfig, bool) {
	if o == nil || o.LogConfig == nil {
		return nil, false
	}
	return o.LogConfig, true
}

// HasLogConfig returns a boolean if a field has been set.
func (o *SidecarTask) HasLogConfig() bool {
	if o != nil && o.LogConfig != nil {
		return true
	}

	return false
}

// SetLogConfig gets a reference to the given LogConfig and assigns it to the LogConfig field.
func (o *SidecarTask) SetLogConfig(v LogConfig) {
	o.LogConfig = &v
}

// GetMeta returns the Meta field value if set, zero value otherwise.
func (o *SidecarTask) GetMeta() map[string]string {
	if o == nil || o.Meta == nil {
		var ret map[string]string
		return ret
	}
	return *o.Meta
}

// GetMetaOk returns a tuple with the Meta field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *SidecarTask) GetMetaOk() (*map[string]string, bool) {
	if o == nil || o.Meta == nil {
		return nil, false
	}
	return o.Meta, true
}

// HasMeta returns a boolean if a field has been set.
func (o *SidecarTask) HasMeta() bool {
	if o != nil && o.Meta != nil {
		return true
	}

	return false
}

// SetMeta gets a reference to the given map[string]string and assigns it to the Meta field.
func (o *SidecarTask) SetMeta(v map[string]string) {
	o.Meta = &v
}

// GetName returns the Name field value if set, zero value otherwise.
func (o *SidecarTask) GetName() string {
	if o == nil || o.Name == nil {
		var ret string
		return ret
	}
	return *o.Name
}

// GetNameOk returns a tuple with the Name field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *SidecarTask) GetNameOk() (*string, bool) {
	if o == nil || o.Name == nil {
		return nil, false
	}
	return o.Name, true
}

// HasName returns a boolean if a field has been set.
func (o *SidecarTask) HasName() bool {
	if o != nil && o.Name != nil {
		return true
	}

	return false
}

// SetName gets a reference to the given string and assigns it to the Name field.
func (o *SidecarTask) SetName(v string) {
	o.Name = &v
}

// GetResources returns the Resources field value if set, zero value otherwise.
func (o *SidecarTask) GetResources() Resources {
	if o == nil || o.Resources == nil {
		var ret Resources
		return ret
	}
	return *o.Resources
}

// GetResourcesOk returns a tuple with the Resources field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *SidecarTask) GetResourcesOk() (*Resources, bool) {
	if o == nil || o.Resources == nil {
		return nil, false
	}
	return o.Resources, true
}

// HasResources returns a boolean if a field has been set.
func (o *SidecarTask) HasResources() bool {
	if o != nil && o.Resources != nil {
		return true
	}

	return false
}

// SetResources gets a reference to the given Resources and assigns it to the Resources field.
func (o *SidecarTask) SetResources(v Resources) {
	o.Resources = &v
}

// GetShutdownDelay returns the ShutdownDelay field value if set, zero value otherwise.
func (o *SidecarTask) GetShutdownDelay() int64 {
	if o == nil || o.ShutdownDelay == nil {
		var ret int64
		return ret
	}
	return *o.ShutdownDelay
}

// GetShutdownDelayOk returns a tuple with the ShutdownDelay field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *SidecarTask) GetShutdownDelayOk() (*int64, bool) {
	if o == nil || o.ShutdownDelay == nil {
		return nil, false
	}
	return o.ShutdownDelay, true
}

// HasShutdownDelay returns a boolean if a field has been set.
func (o *SidecarTask) HasShutdownDelay() bool {
	if o != nil && o.ShutdownDelay != nil {
		return true
	}

	return false
}

// SetShutdownDelay gets a reference to the given int64 and assigns it to the ShutdownDelay field.
func (o *SidecarTask) SetShutdownDelay(v int64) {
	o.ShutdownDelay = &v
}

// GetUser returns the User field value if set, zero value otherwise.
func (o *SidecarTask) GetUser() string {
	if o == nil || o.User == nil {
		var ret string
		return ret
	}
	return *o.User
}

// GetUserOk returns a tuple with the User field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *SidecarTask) GetUserOk() (*string, bool) {
	if o == nil || o.User == nil {
		return nil, false
	}
	return o.User, true
}

// HasUser returns a boolean if a field has been set.
func (o *SidecarTask) HasUser() bool {
	if o != nil && o.User != nil {
		return true
	}

	return false
}

// SetUser gets a reference to the given string and assigns it to the User field.
func (o *SidecarTask) SetUser(v string) {
	o.User = &v
}

func (o SidecarTask) MarshalJSON() ([]byte, error) {
	toSerialize := map[string]interface{}{}
	if o.Config != nil {
		toSerialize["Config"] = o.Config
	}
	if o.Driver != nil {
		toSerialize["Driver"] = o.Driver
	}
	if o.Env != nil {
		toSerialize["Env"] = o.Env
	}
	if o.KillSignal != nil {
		toSerialize["KillSignal"] = o.KillSignal
	}
	if o.KillTimeout != nil {
		toSerialize["KillTimeout"] = o.KillTimeout
	}
	if o.LogConfig != nil {
		toSerialize["LogConfig"] = o.LogConfig
	}
	if o.Meta != nil {
		toSerialize["Meta"] = o.Meta
	}
	if o.Name != nil {
		toSerialize["Name"] = o.Name
	}
	if o.Resources != nil {
		toSerialize["Resources"] = o.Resources
	}
	if o.ShutdownDelay != nil {
		toSerialize["ShutdownDelay"] = o.ShutdownDelay
	}
	if o.User != nil {
		toSerialize["User"] = o.User
	}
	return json.Marshal(toSerialize)
}

type NullableSidecarTask struct {
	value *SidecarTask
	isSet bool
}

func (v NullableSidecarTask) Get() *SidecarTask {
	return v.value
}

func (v *NullableSidecarTask) Set(val *SidecarTask) {
	v.value = val
	v.isSet = true
}

func (v NullableSidecarTask) IsSet() bool {
	return v.isSet
}

func (v *NullableSidecarTask) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableSidecarTask(val *SidecarTask) *NullableSidecarTask {
	return &NullableSidecarTask{value: val, isSet: true}
}

func (v NullableSidecarTask) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableSidecarTask) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}


