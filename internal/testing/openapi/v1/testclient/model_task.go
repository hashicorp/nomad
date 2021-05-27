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

// Task struct for Task
type Task struct {
	Affinities *[]Affinity `json:"Affinities,omitempty"`
	Artifacts *[]TaskArtifact `json:"Artifacts,omitempty"`
	CSIPluginConfig *TaskCSIPluginConfig `json:"CSIPluginConfig,omitempty"`
	Config *map[string]map[string]interface{} `json:"Config,omitempty"`
	Constraints *[]Constraint `json:"Constraints,omitempty"`
	DispatchPayload *DispatchPayloadConfig `json:"DispatchPayload,omitempty"`
	Driver *string `json:"Driver,omitempty"`
	Env *map[string]string `json:"Env,omitempty"`
	KillSignal *string `json:"KillSignal,omitempty"`
	// A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years.
	KillTimeout *int64 `json:"KillTimeout,omitempty"`
	Kind *string `json:"Kind,omitempty"`
	Leader *bool `json:"Leader,omitempty"`
	Lifecycle *TaskLifecycle `json:"Lifecycle,omitempty"`
	LogConfig *LogConfig `json:"LogConfig,omitempty"`
	Meta *map[string]string `json:"Meta,omitempty"`
	Name *string `json:"Name,omitempty"`
	Resources *Resources `json:"Resources,omitempty"`
	RestartPolicy *RestartPolicy `json:"RestartPolicy,omitempty"`
	ScalingPolicies *[]ScalingPolicy `json:"ScalingPolicies,omitempty"`
	Services *[]Service `json:"Services,omitempty"`
	// A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years.
	ShutdownDelay *int64 `json:"ShutdownDelay,omitempty"`
	Templates *[]Template `json:"Templates,omitempty"`
	User *string `json:"User,omitempty"`
	Vault *Vault `json:"Vault,omitempty"`
	VolumeMounts *[]VolumeMount `json:"VolumeMounts,omitempty"`
}

// NewTask instantiates a new Task object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewTask() *Task {
	this := Task{}
	return &this
}

// NewTaskWithDefaults instantiates a new Task object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewTaskWithDefaults() *Task {
	this := Task{}
	return &this
}

// GetAffinities returns the Affinities field value if set, zero value otherwise.
func (o *Task) GetAffinities() []Affinity {
	if o == nil || o.Affinities == nil {
		var ret []Affinity
		return ret
	}
	return *o.Affinities
}

// GetAffinitiesOk returns a tuple with the Affinities field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Task) GetAffinitiesOk() (*[]Affinity, bool) {
	if o == nil || o.Affinities == nil {
		return nil, false
	}
	return o.Affinities, true
}

// HasAffinities returns a boolean if a field has been set.
func (o *Task) HasAffinities() bool {
	if o != nil && o.Affinities != nil {
		return true
	}

	return false
}

// SetAffinities gets a reference to the given []Affinity and assigns it to the Affinities field.
func (o *Task) SetAffinities(v []Affinity) {
	o.Affinities = &v
}

// GetArtifacts returns the Artifacts field value if set, zero value otherwise.
func (o *Task) GetArtifacts() []TaskArtifact {
	if o == nil || o.Artifacts == nil {
		var ret []TaskArtifact
		return ret
	}
	return *o.Artifacts
}

// GetArtifactsOk returns a tuple with the Artifacts field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Task) GetArtifactsOk() (*[]TaskArtifact, bool) {
	if o == nil || o.Artifacts == nil {
		return nil, false
	}
	return o.Artifacts, true
}

// HasArtifacts returns a boolean if a field has been set.
func (o *Task) HasArtifacts() bool {
	if o != nil && o.Artifacts != nil {
		return true
	}

	return false
}

// SetArtifacts gets a reference to the given []TaskArtifact and assigns it to the Artifacts field.
func (o *Task) SetArtifacts(v []TaskArtifact) {
	o.Artifacts = &v
}

// GetCSIPluginConfig returns the CSIPluginConfig field value if set, zero value otherwise.
func (o *Task) GetCSIPluginConfig() TaskCSIPluginConfig {
	if o == nil || o.CSIPluginConfig == nil {
		var ret TaskCSIPluginConfig
		return ret
	}
	return *o.CSIPluginConfig
}

// GetCSIPluginConfigOk returns a tuple with the CSIPluginConfig field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Task) GetCSIPluginConfigOk() (*TaskCSIPluginConfig, bool) {
	if o == nil || o.CSIPluginConfig == nil {
		return nil, false
	}
	return o.CSIPluginConfig, true
}

// HasCSIPluginConfig returns a boolean if a field has been set.
func (o *Task) HasCSIPluginConfig() bool {
	if o != nil && o.CSIPluginConfig != nil {
		return true
	}

	return false
}

// SetCSIPluginConfig gets a reference to the given TaskCSIPluginConfig and assigns it to the CSIPluginConfig field.
func (o *Task) SetCSIPluginConfig(v TaskCSIPluginConfig) {
	o.CSIPluginConfig = &v
}

// GetConfig returns the Config field value if set, zero value otherwise.
func (o *Task) GetConfig() map[string]map[string]interface{} {
	if o == nil || o.Config == nil {
		var ret map[string]map[string]interface{}
		return ret
	}
	return *o.Config
}

// GetConfigOk returns a tuple with the Config field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Task) GetConfigOk() (*map[string]map[string]interface{}, bool) {
	if o == nil || o.Config == nil {
		return nil, false
	}
	return o.Config, true
}

// HasConfig returns a boolean if a field has been set.
func (o *Task) HasConfig() bool {
	if o != nil && o.Config != nil {
		return true
	}

	return false
}

// SetConfig gets a reference to the given map[string]map[string]interface{} and assigns it to the Config field.
func (o *Task) SetConfig(v map[string]map[string]interface{}) {
	o.Config = &v
}

// GetConstraints returns the Constraints field value if set, zero value otherwise.
func (o *Task) GetConstraints() []Constraint {
	if o == nil || o.Constraints == nil {
		var ret []Constraint
		return ret
	}
	return *o.Constraints
}

// GetConstraintsOk returns a tuple with the Constraints field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Task) GetConstraintsOk() (*[]Constraint, bool) {
	if o == nil || o.Constraints == nil {
		return nil, false
	}
	return o.Constraints, true
}

// HasConstraints returns a boolean if a field has been set.
func (o *Task) HasConstraints() bool {
	if o != nil && o.Constraints != nil {
		return true
	}

	return false
}

// SetConstraints gets a reference to the given []Constraint and assigns it to the Constraints field.
func (o *Task) SetConstraints(v []Constraint) {
	o.Constraints = &v
}

// GetDispatchPayload returns the DispatchPayload field value if set, zero value otherwise.
func (o *Task) GetDispatchPayload() DispatchPayloadConfig {
	if o == nil || o.DispatchPayload == nil {
		var ret DispatchPayloadConfig
		return ret
	}
	return *o.DispatchPayload
}

// GetDispatchPayloadOk returns a tuple with the DispatchPayload field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Task) GetDispatchPayloadOk() (*DispatchPayloadConfig, bool) {
	if o == nil || o.DispatchPayload == nil {
		return nil, false
	}
	return o.DispatchPayload, true
}

// HasDispatchPayload returns a boolean if a field has been set.
func (o *Task) HasDispatchPayload() bool {
	if o != nil && o.DispatchPayload != nil {
		return true
	}

	return false
}

// SetDispatchPayload gets a reference to the given DispatchPayloadConfig and assigns it to the DispatchPayload field.
func (o *Task) SetDispatchPayload(v DispatchPayloadConfig) {
	o.DispatchPayload = &v
}

// GetDriver returns the Driver field value if set, zero value otherwise.
func (o *Task) GetDriver() string {
	if o == nil || o.Driver == nil {
		var ret string
		return ret
	}
	return *o.Driver
}

// GetDriverOk returns a tuple with the Driver field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Task) GetDriverOk() (*string, bool) {
	if o == nil || o.Driver == nil {
		return nil, false
	}
	return o.Driver, true
}

// HasDriver returns a boolean if a field has been set.
func (o *Task) HasDriver() bool {
	if o != nil && o.Driver != nil {
		return true
	}

	return false
}

// SetDriver gets a reference to the given string and assigns it to the Driver field.
func (o *Task) SetDriver(v string) {
	o.Driver = &v
}

// GetEnv returns the Env field value if set, zero value otherwise.
func (o *Task) GetEnv() map[string]string {
	if o == nil || o.Env == nil {
		var ret map[string]string
		return ret
	}
	return *o.Env
}

// GetEnvOk returns a tuple with the Env field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Task) GetEnvOk() (*map[string]string, bool) {
	if o == nil || o.Env == nil {
		return nil, false
	}
	return o.Env, true
}

// HasEnv returns a boolean if a field has been set.
func (o *Task) HasEnv() bool {
	if o != nil && o.Env != nil {
		return true
	}

	return false
}

// SetEnv gets a reference to the given map[string]string and assigns it to the Env field.
func (o *Task) SetEnv(v map[string]string) {
	o.Env = &v
}

// GetKillSignal returns the KillSignal field value if set, zero value otherwise.
func (o *Task) GetKillSignal() string {
	if o == nil || o.KillSignal == nil {
		var ret string
		return ret
	}
	return *o.KillSignal
}

// GetKillSignalOk returns a tuple with the KillSignal field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Task) GetKillSignalOk() (*string, bool) {
	if o == nil || o.KillSignal == nil {
		return nil, false
	}
	return o.KillSignal, true
}

// HasKillSignal returns a boolean if a field has been set.
func (o *Task) HasKillSignal() bool {
	if o != nil && o.KillSignal != nil {
		return true
	}

	return false
}

// SetKillSignal gets a reference to the given string and assigns it to the KillSignal field.
func (o *Task) SetKillSignal(v string) {
	o.KillSignal = &v
}

// GetKillTimeout returns the KillTimeout field value if set, zero value otherwise.
func (o *Task) GetKillTimeout() int64 {
	if o == nil || o.KillTimeout == nil {
		var ret int64
		return ret
	}
	return *o.KillTimeout
}

// GetKillTimeoutOk returns a tuple with the KillTimeout field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Task) GetKillTimeoutOk() (*int64, bool) {
	if o == nil || o.KillTimeout == nil {
		return nil, false
	}
	return o.KillTimeout, true
}

// HasKillTimeout returns a boolean if a field has been set.
func (o *Task) HasKillTimeout() bool {
	if o != nil && o.KillTimeout != nil {
		return true
	}

	return false
}

// SetKillTimeout gets a reference to the given int64 and assigns it to the KillTimeout field.
func (o *Task) SetKillTimeout(v int64) {
	o.KillTimeout = &v
}

// GetKind returns the Kind field value if set, zero value otherwise.
func (o *Task) GetKind() string {
	if o == nil || o.Kind == nil {
		var ret string
		return ret
	}
	return *o.Kind
}

// GetKindOk returns a tuple with the Kind field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Task) GetKindOk() (*string, bool) {
	if o == nil || o.Kind == nil {
		return nil, false
	}
	return o.Kind, true
}

// HasKind returns a boolean if a field has been set.
func (o *Task) HasKind() bool {
	if o != nil && o.Kind != nil {
		return true
	}

	return false
}

// SetKind gets a reference to the given string and assigns it to the Kind field.
func (o *Task) SetKind(v string) {
	o.Kind = &v
}

// GetLeader returns the Leader field value if set, zero value otherwise.
func (o *Task) GetLeader() bool {
	if o == nil || o.Leader == nil {
		var ret bool
		return ret
	}
	return *o.Leader
}

// GetLeaderOk returns a tuple with the Leader field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Task) GetLeaderOk() (*bool, bool) {
	if o == nil || o.Leader == nil {
		return nil, false
	}
	return o.Leader, true
}

// HasLeader returns a boolean if a field has been set.
func (o *Task) HasLeader() bool {
	if o != nil && o.Leader != nil {
		return true
	}

	return false
}

// SetLeader gets a reference to the given bool and assigns it to the Leader field.
func (o *Task) SetLeader(v bool) {
	o.Leader = &v
}

// GetLifecycle returns the Lifecycle field value if set, zero value otherwise.
func (o *Task) GetLifecycle() TaskLifecycle {
	if o == nil || o.Lifecycle == nil {
		var ret TaskLifecycle
		return ret
	}
	return *o.Lifecycle
}

// GetLifecycleOk returns a tuple with the Lifecycle field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Task) GetLifecycleOk() (*TaskLifecycle, bool) {
	if o == nil || o.Lifecycle == nil {
		return nil, false
	}
	return o.Lifecycle, true
}

// HasLifecycle returns a boolean if a field has been set.
func (o *Task) HasLifecycle() bool {
	if o != nil && o.Lifecycle != nil {
		return true
	}

	return false
}

// SetLifecycle gets a reference to the given TaskLifecycle and assigns it to the Lifecycle field.
func (o *Task) SetLifecycle(v TaskLifecycle) {
	o.Lifecycle = &v
}

// GetLogConfig returns the LogConfig field value if set, zero value otherwise.
func (o *Task) GetLogConfig() LogConfig {
	if o == nil || o.LogConfig == nil {
		var ret LogConfig
		return ret
	}
	return *o.LogConfig
}

// GetLogConfigOk returns a tuple with the LogConfig field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Task) GetLogConfigOk() (*LogConfig, bool) {
	if o == nil || o.LogConfig == nil {
		return nil, false
	}
	return o.LogConfig, true
}

// HasLogConfig returns a boolean if a field has been set.
func (o *Task) HasLogConfig() bool {
	if o != nil && o.LogConfig != nil {
		return true
	}

	return false
}

// SetLogConfig gets a reference to the given LogConfig and assigns it to the LogConfig field.
func (o *Task) SetLogConfig(v LogConfig) {
	o.LogConfig = &v
}

// GetMeta returns the Meta field value if set, zero value otherwise.
func (o *Task) GetMeta() map[string]string {
	if o == nil || o.Meta == nil {
		var ret map[string]string
		return ret
	}
	return *o.Meta
}

// GetMetaOk returns a tuple with the Meta field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Task) GetMetaOk() (*map[string]string, bool) {
	if o == nil || o.Meta == nil {
		return nil, false
	}
	return o.Meta, true
}

// HasMeta returns a boolean if a field has been set.
func (o *Task) HasMeta() bool {
	if o != nil && o.Meta != nil {
		return true
	}

	return false
}

// SetMeta gets a reference to the given map[string]string and assigns it to the Meta field.
func (o *Task) SetMeta(v map[string]string) {
	o.Meta = &v
}

// GetName returns the Name field value if set, zero value otherwise.
func (o *Task) GetName() string {
	if o == nil || o.Name == nil {
		var ret string
		return ret
	}
	return *o.Name
}

// GetNameOk returns a tuple with the Name field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Task) GetNameOk() (*string, bool) {
	if o == nil || o.Name == nil {
		return nil, false
	}
	return o.Name, true
}

// HasName returns a boolean if a field has been set.
func (o *Task) HasName() bool {
	if o != nil && o.Name != nil {
		return true
	}

	return false
}

// SetName gets a reference to the given string and assigns it to the Name field.
func (o *Task) SetName(v string) {
	o.Name = &v
}

// GetResources returns the Resources field value if set, zero value otherwise.
func (o *Task) GetResources() Resources {
	if o == nil || o.Resources == nil {
		var ret Resources
		return ret
	}
	return *o.Resources
}

// GetResourcesOk returns a tuple with the Resources field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Task) GetResourcesOk() (*Resources, bool) {
	if o == nil || o.Resources == nil {
		return nil, false
	}
	return o.Resources, true
}

// HasResources returns a boolean if a field has been set.
func (o *Task) HasResources() bool {
	if o != nil && o.Resources != nil {
		return true
	}

	return false
}

// SetResources gets a reference to the given Resources and assigns it to the Resources field.
func (o *Task) SetResources(v Resources) {
	o.Resources = &v
}

// GetRestartPolicy returns the RestartPolicy field value if set, zero value otherwise.
func (o *Task) GetRestartPolicy() RestartPolicy {
	if o == nil || o.RestartPolicy == nil {
		var ret RestartPolicy
		return ret
	}
	return *o.RestartPolicy
}

// GetRestartPolicyOk returns a tuple with the RestartPolicy field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Task) GetRestartPolicyOk() (*RestartPolicy, bool) {
	if o == nil || o.RestartPolicy == nil {
		return nil, false
	}
	return o.RestartPolicy, true
}

// HasRestartPolicy returns a boolean if a field has been set.
func (o *Task) HasRestartPolicy() bool {
	if o != nil && o.RestartPolicy != nil {
		return true
	}

	return false
}

// SetRestartPolicy gets a reference to the given RestartPolicy and assigns it to the RestartPolicy field.
func (o *Task) SetRestartPolicy(v RestartPolicy) {
	o.RestartPolicy = &v
}

// GetScalingPolicies returns the ScalingPolicies field value if set, zero value otherwise.
func (o *Task) GetScalingPolicies() []ScalingPolicy {
	if o == nil || o.ScalingPolicies == nil {
		var ret []ScalingPolicy
		return ret
	}
	return *o.ScalingPolicies
}

// GetScalingPoliciesOk returns a tuple with the ScalingPolicies field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Task) GetScalingPoliciesOk() (*[]ScalingPolicy, bool) {
	if o == nil || o.ScalingPolicies == nil {
		return nil, false
	}
	return o.ScalingPolicies, true
}

// HasScalingPolicies returns a boolean if a field has been set.
func (o *Task) HasScalingPolicies() bool {
	if o != nil && o.ScalingPolicies != nil {
		return true
	}

	return false
}

// SetScalingPolicies gets a reference to the given []ScalingPolicy and assigns it to the ScalingPolicies field.
func (o *Task) SetScalingPolicies(v []ScalingPolicy) {
	o.ScalingPolicies = &v
}

// GetServices returns the Services field value if set, zero value otherwise.
func (o *Task) GetServices() []Service {
	if o == nil || o.Services == nil {
		var ret []Service
		return ret
	}
	return *o.Services
}

// GetServicesOk returns a tuple with the Services field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Task) GetServicesOk() (*[]Service, bool) {
	if o == nil || o.Services == nil {
		return nil, false
	}
	return o.Services, true
}

// HasServices returns a boolean if a field has been set.
func (o *Task) HasServices() bool {
	if o != nil && o.Services != nil {
		return true
	}

	return false
}

// SetServices gets a reference to the given []Service and assigns it to the Services field.
func (o *Task) SetServices(v []Service) {
	o.Services = &v
}

// GetShutdownDelay returns the ShutdownDelay field value if set, zero value otherwise.
func (o *Task) GetShutdownDelay() int64 {
	if o == nil || o.ShutdownDelay == nil {
		var ret int64
		return ret
	}
	return *o.ShutdownDelay
}

// GetShutdownDelayOk returns a tuple with the ShutdownDelay field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Task) GetShutdownDelayOk() (*int64, bool) {
	if o == nil || o.ShutdownDelay == nil {
		return nil, false
	}
	return o.ShutdownDelay, true
}

// HasShutdownDelay returns a boolean if a field has been set.
func (o *Task) HasShutdownDelay() bool {
	if o != nil && o.ShutdownDelay != nil {
		return true
	}

	return false
}

// SetShutdownDelay gets a reference to the given int64 and assigns it to the ShutdownDelay field.
func (o *Task) SetShutdownDelay(v int64) {
	o.ShutdownDelay = &v
}

// GetTemplates returns the Templates field value if set, zero value otherwise.
func (o *Task) GetTemplates() []Template {
	if o == nil || o.Templates == nil {
		var ret []Template
		return ret
	}
	return *o.Templates
}

// GetTemplatesOk returns a tuple with the Templates field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Task) GetTemplatesOk() (*[]Template, bool) {
	if o == nil || o.Templates == nil {
		return nil, false
	}
	return o.Templates, true
}

// HasTemplates returns a boolean if a field has been set.
func (o *Task) HasTemplates() bool {
	if o != nil && o.Templates != nil {
		return true
	}

	return false
}

// SetTemplates gets a reference to the given []Template and assigns it to the Templates field.
func (o *Task) SetTemplates(v []Template) {
	o.Templates = &v
}

// GetUser returns the User field value if set, zero value otherwise.
func (o *Task) GetUser() string {
	if o == nil || o.User == nil {
		var ret string
		return ret
	}
	return *o.User
}

// GetUserOk returns a tuple with the User field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Task) GetUserOk() (*string, bool) {
	if o == nil || o.User == nil {
		return nil, false
	}
	return o.User, true
}

// HasUser returns a boolean if a field has been set.
func (o *Task) HasUser() bool {
	if o != nil && o.User != nil {
		return true
	}

	return false
}

// SetUser gets a reference to the given string and assigns it to the User field.
func (o *Task) SetUser(v string) {
	o.User = &v
}

// GetVault returns the Vault field value if set, zero value otherwise.
func (o *Task) GetVault() Vault {
	if o == nil || o.Vault == nil {
		var ret Vault
		return ret
	}
	return *o.Vault
}

// GetVaultOk returns a tuple with the Vault field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Task) GetVaultOk() (*Vault, bool) {
	if o == nil || o.Vault == nil {
		return nil, false
	}
	return o.Vault, true
}

// HasVault returns a boolean if a field has been set.
func (o *Task) HasVault() bool {
	if o != nil && o.Vault != nil {
		return true
	}

	return false
}

// SetVault gets a reference to the given Vault and assigns it to the Vault field.
func (o *Task) SetVault(v Vault) {
	o.Vault = &v
}

// GetVolumeMounts returns the VolumeMounts field value if set, zero value otherwise.
func (o *Task) GetVolumeMounts() []VolumeMount {
	if o == nil || o.VolumeMounts == nil {
		var ret []VolumeMount
		return ret
	}
	return *o.VolumeMounts
}

// GetVolumeMountsOk returns a tuple with the VolumeMounts field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Task) GetVolumeMountsOk() (*[]VolumeMount, bool) {
	if o == nil || o.VolumeMounts == nil {
		return nil, false
	}
	return o.VolumeMounts, true
}

// HasVolumeMounts returns a boolean if a field has been set.
func (o *Task) HasVolumeMounts() bool {
	if o != nil && o.VolumeMounts != nil {
		return true
	}

	return false
}

// SetVolumeMounts gets a reference to the given []VolumeMount and assigns it to the VolumeMounts field.
func (o *Task) SetVolumeMounts(v []VolumeMount) {
	o.VolumeMounts = &v
}

func (o Task) MarshalJSON() ([]byte, error) {
	toSerialize := map[string]interface{}{}
	if o.Affinities != nil {
		toSerialize["Affinities"] = o.Affinities
	}
	if o.Artifacts != nil {
		toSerialize["Artifacts"] = o.Artifacts
	}
	if o.CSIPluginConfig != nil {
		toSerialize["CSIPluginConfig"] = o.CSIPluginConfig
	}
	if o.Config != nil {
		toSerialize["Config"] = o.Config
	}
	if o.Constraints != nil {
		toSerialize["Constraints"] = o.Constraints
	}
	if o.DispatchPayload != nil {
		toSerialize["DispatchPayload"] = o.DispatchPayload
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
	if o.Kind != nil {
		toSerialize["Kind"] = o.Kind
	}
	if o.Leader != nil {
		toSerialize["Leader"] = o.Leader
	}
	if o.Lifecycle != nil {
		toSerialize["Lifecycle"] = o.Lifecycle
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
	if o.RestartPolicy != nil {
		toSerialize["RestartPolicy"] = o.RestartPolicy
	}
	if o.ScalingPolicies != nil {
		toSerialize["ScalingPolicies"] = o.ScalingPolicies
	}
	if o.Services != nil {
		toSerialize["Services"] = o.Services
	}
	if o.ShutdownDelay != nil {
		toSerialize["ShutdownDelay"] = o.ShutdownDelay
	}
	if o.Templates != nil {
		toSerialize["Templates"] = o.Templates
	}
	if o.User != nil {
		toSerialize["User"] = o.User
	}
	if o.Vault != nil {
		toSerialize["Vault"] = o.Vault
	}
	if o.VolumeMounts != nil {
		toSerialize["VolumeMounts"] = o.VolumeMounts
	}
	return json.Marshal(toSerialize)
}

type NullableTask struct {
	value *Task
	isSet bool
}

func (v NullableTask) Get() *Task {
	return v.value
}

func (v *NullableTask) Set(val *Task) {
	v.value = val
	v.isSet = true
}

func (v NullableTask) IsSet() bool {
	return v.isSet
}

func (v *NullableTask) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableTask(val *Task) *NullableTask {
	return &NullableTask{value: val, isSet: true}
}

func (v NullableTask) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableTask) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}


