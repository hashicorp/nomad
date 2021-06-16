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

// TaskGroup struct for TaskGroup
type TaskGroup struct {
	Affinities *[]Affinity `json:"Affinities,omitempty"`
	Constraints *[]Constraint `json:"Constraints,omitempty"`
	Consul *Consul `json:"Consul,omitempty"`
	Count *int64 `json:"Count,omitempty"`
	EphemeralDisk *EphemeralDisk `json:"EphemeralDisk,omitempty"`
	Meta *map[string]string `json:"Meta,omitempty"`
	Migrate *MigrateStrategy `json:"Migrate,omitempty"`
	Name *string `json:"Name,omitempty"`
	Networks *[]NetworkResource `json:"Networks,omitempty"`
	ReschedulePolicy *ReschedulePolicy `json:"ReschedulePolicy,omitempty"`
	RestartPolicy *RestartPolicy `json:"RestartPolicy,omitempty"`
	Scaling *ScalingPolicy `json:"Scaling,omitempty"`
	Services *[]Service `json:"Services,omitempty"`
	// A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years.
	ShutdownDelay *int64 `json:"ShutdownDelay,omitempty"`
	Spreads *[]Spread `json:"Spreads,omitempty"`
	// A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years.
	StopAfterClientDisconnect *int64 `json:"StopAfterClientDisconnect,omitempty"`
	Tasks *[]Task `json:"Tasks,omitempty"`
	Update *UpdateStrategy `json:"Update,omitempty"`
	Volumes *map[string]VolumeRequest `json:"Volumes,omitempty"`
}

// NewTaskGroup instantiates a new TaskGroup object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewTaskGroup() *TaskGroup {
	this := TaskGroup{}
	return &this
}

// NewTaskGroupWithDefaults instantiates a new TaskGroup object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewTaskGroupWithDefaults() *TaskGroup {
	this := TaskGroup{}
	return &this
}

// GetAffinities returns the Affinities field value if set, zero value otherwise.
func (o *TaskGroup) GetAffinities() []Affinity {
	if o == nil || o.Affinities == nil {
		var ret []Affinity
		return ret
	}
	return *o.Affinities
}

// GetAffinitiesOk returns a tuple with the Affinities field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *TaskGroup) GetAffinitiesOk() (*[]Affinity, bool) {
	if o == nil || o.Affinities == nil {
		return nil, false
	}
	return o.Affinities, true
}

// HasAffinities returns a boolean if a field has been set.
func (o *TaskGroup) HasAffinities() bool {
	if o != nil && o.Affinities != nil {
		return true
	}

	return false
}

// SetAffinities gets a reference to the given []Affinity and assigns it to the Affinities field.
func (o *TaskGroup) SetAffinities(v []Affinity) {
	o.Affinities = &v
}

// GetConstraints returns the Constraints field value if set, zero value otherwise.
func (o *TaskGroup) GetConstraints() []Constraint {
	if o == nil || o.Constraints == nil {
		var ret []Constraint
		return ret
	}
	return *o.Constraints
}

// GetConstraintsOk returns a tuple with the Constraints field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *TaskGroup) GetConstraintsOk() (*[]Constraint, bool) {
	if o == nil || o.Constraints == nil {
		return nil, false
	}
	return o.Constraints, true
}

// HasConstraints returns a boolean if a field has been set.
func (o *TaskGroup) HasConstraints() bool {
	if o != nil && o.Constraints != nil {
		return true
	}

	return false
}

// SetConstraints gets a reference to the given []Constraint and assigns it to the Constraints field.
func (o *TaskGroup) SetConstraints(v []Constraint) {
	o.Constraints = &v
}

// GetConsul returns the Consul field value if set, zero value otherwise.
func (o *TaskGroup) GetConsul() Consul {
	if o == nil || o.Consul == nil {
		var ret Consul
		return ret
	}
	return *o.Consul
}

// GetConsulOk returns a tuple with the Consul field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *TaskGroup) GetConsulOk() (*Consul, bool) {
	if o == nil || o.Consul == nil {
		return nil, false
	}
	return o.Consul, true
}

// HasConsul returns a boolean if a field has been set.
func (o *TaskGroup) HasConsul() bool {
	if o != nil && o.Consul != nil {
		return true
	}

	return false
}

// SetConsul gets a reference to the given Consul and assigns it to the Consul field.
func (o *TaskGroup) SetConsul(v Consul) {
	o.Consul = &v
}

// GetCount returns the Count field value if set, zero value otherwise.
func (o *TaskGroup) GetCount() int64 {
	if o == nil || o.Count == nil {
		var ret int64
		return ret
	}
	return *o.Count
}

// GetCountOk returns a tuple with the Count field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *TaskGroup) GetCountOk() (*int64, bool) {
	if o == nil || o.Count == nil {
		return nil, false
	}
	return o.Count, true
}

// HasCount returns a boolean if a field has been set.
func (o *TaskGroup) HasCount() bool {
	if o != nil && o.Count != nil {
		return true
	}

	return false
}

// SetCount gets a reference to the given int64 and assigns it to the Count field.
func (o *TaskGroup) SetCount(v int64) {
	o.Count = &v
}

// GetEphemeralDisk returns the EphemeralDisk field value if set, zero value otherwise.
func (o *TaskGroup) GetEphemeralDisk() EphemeralDisk {
	if o == nil || o.EphemeralDisk == nil {
		var ret EphemeralDisk
		return ret
	}
	return *o.EphemeralDisk
}

// GetEphemeralDiskOk returns a tuple with the EphemeralDisk field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *TaskGroup) GetEphemeralDiskOk() (*EphemeralDisk, bool) {
	if o == nil || o.EphemeralDisk == nil {
		return nil, false
	}
	return o.EphemeralDisk, true
}

// HasEphemeralDisk returns a boolean if a field has been set.
func (o *TaskGroup) HasEphemeralDisk() bool {
	if o != nil && o.EphemeralDisk != nil {
		return true
	}

	return false
}

// SetEphemeralDisk gets a reference to the given EphemeralDisk and assigns it to the EphemeralDisk field.
func (o *TaskGroup) SetEphemeralDisk(v EphemeralDisk) {
	o.EphemeralDisk = &v
}

// GetMeta returns the Meta field value if set, zero value otherwise.
func (o *TaskGroup) GetMeta() map[string]string {
	if o == nil || o.Meta == nil {
		var ret map[string]string
		return ret
	}
	return *o.Meta
}

// GetMetaOk returns a tuple with the Meta field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *TaskGroup) GetMetaOk() (*map[string]string, bool) {
	if o == nil || o.Meta == nil {
		return nil, false
	}
	return o.Meta, true
}

// HasMeta returns a boolean if a field has been set.
func (o *TaskGroup) HasMeta() bool {
	if o != nil && o.Meta != nil {
		return true
	}

	return false
}

// SetMeta gets a reference to the given map[string]string and assigns it to the Meta field.
func (o *TaskGroup) SetMeta(v map[string]string) {
	o.Meta = &v
}

// GetMigrate returns the Migrate field value if set, zero value otherwise.
func (o *TaskGroup) GetMigrate() MigrateStrategy {
	if o == nil || o.Migrate == nil {
		var ret MigrateStrategy
		return ret
	}
	return *o.Migrate
}

// GetMigrateOk returns a tuple with the Migrate field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *TaskGroup) GetMigrateOk() (*MigrateStrategy, bool) {
	if o == nil || o.Migrate == nil {
		return nil, false
	}
	return o.Migrate, true
}

// HasMigrate returns a boolean if a field has been set.
func (o *TaskGroup) HasMigrate() bool {
	if o != nil && o.Migrate != nil {
		return true
	}

	return false
}

// SetMigrate gets a reference to the given MigrateStrategy and assigns it to the Migrate field.
func (o *TaskGroup) SetMigrate(v MigrateStrategy) {
	o.Migrate = &v
}

// GetName returns the Name field value if set, zero value otherwise.
func (o *TaskGroup) GetName() string {
	if o == nil || o.Name == nil {
		var ret string
		return ret
	}
	return *o.Name
}

// GetNameOk returns a tuple with the Name field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *TaskGroup) GetNameOk() (*string, bool) {
	if o == nil || o.Name == nil {
		return nil, false
	}
	return o.Name, true
}

// HasName returns a boolean if a field has been set.
func (o *TaskGroup) HasName() bool {
	if o != nil && o.Name != nil {
		return true
	}

	return false
}

// SetName gets a reference to the given string and assigns it to the Name field.
func (o *TaskGroup) SetName(v string) {
	o.Name = &v
}

// GetNetworks returns the Networks field value if set, zero value otherwise.
func (o *TaskGroup) GetNetworks() []NetworkResource {
	if o == nil || o.Networks == nil {
		var ret []NetworkResource
		return ret
	}
	return *o.Networks
}

// GetNetworksOk returns a tuple with the Networks field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *TaskGroup) GetNetworksOk() (*[]NetworkResource, bool) {
	if o == nil || o.Networks == nil {
		return nil, false
	}
	return o.Networks, true
}

// HasNetworks returns a boolean if a field has been set.
func (o *TaskGroup) HasNetworks() bool {
	if o != nil && o.Networks != nil {
		return true
	}

	return false
}

// SetNetworks gets a reference to the given []NetworkResource and assigns it to the Networks field.
func (o *TaskGroup) SetNetworks(v []NetworkResource) {
	o.Networks = &v
}

// GetReschedulePolicy returns the ReschedulePolicy field value if set, zero value otherwise.
func (o *TaskGroup) GetReschedulePolicy() ReschedulePolicy {
	if o == nil || o.ReschedulePolicy == nil {
		var ret ReschedulePolicy
		return ret
	}
	return *o.ReschedulePolicy
}

// GetReschedulePolicyOk returns a tuple with the ReschedulePolicy field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *TaskGroup) GetReschedulePolicyOk() (*ReschedulePolicy, bool) {
	if o == nil || o.ReschedulePolicy == nil {
		return nil, false
	}
	return o.ReschedulePolicy, true
}

// HasReschedulePolicy returns a boolean if a field has been set.
func (o *TaskGroup) HasReschedulePolicy() bool {
	if o != nil && o.ReschedulePolicy != nil {
		return true
	}

	return false
}

// SetReschedulePolicy gets a reference to the given ReschedulePolicy and assigns it to the ReschedulePolicy field.
func (o *TaskGroup) SetReschedulePolicy(v ReschedulePolicy) {
	o.ReschedulePolicy = &v
}

// GetRestartPolicy returns the RestartPolicy field value if set, zero value otherwise.
func (o *TaskGroup) GetRestartPolicy() RestartPolicy {
	if o == nil || o.RestartPolicy == nil {
		var ret RestartPolicy
		return ret
	}
	return *o.RestartPolicy
}

// GetRestartPolicyOk returns a tuple with the RestartPolicy field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *TaskGroup) GetRestartPolicyOk() (*RestartPolicy, bool) {
	if o == nil || o.RestartPolicy == nil {
		return nil, false
	}
	return o.RestartPolicy, true
}

// HasRestartPolicy returns a boolean if a field has been set.
func (o *TaskGroup) HasRestartPolicy() bool {
	if o != nil && o.RestartPolicy != nil {
		return true
	}

	return false
}

// SetRestartPolicy gets a reference to the given RestartPolicy and assigns it to the RestartPolicy field.
func (o *TaskGroup) SetRestartPolicy(v RestartPolicy) {
	o.RestartPolicy = &v
}

// GetScaling returns the Scaling field value if set, zero value otherwise.
func (o *TaskGroup) GetScaling() ScalingPolicy {
	if o == nil || o.Scaling == nil {
		var ret ScalingPolicy
		return ret
	}
	return *o.Scaling
}

// GetScalingOk returns a tuple with the Scaling field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *TaskGroup) GetScalingOk() (*ScalingPolicy, bool) {
	if o == nil || o.Scaling == nil {
		return nil, false
	}
	return o.Scaling, true
}

// HasScaling returns a boolean if a field has been set.
func (o *TaskGroup) HasScaling() bool {
	if o != nil && o.Scaling != nil {
		return true
	}

	return false
}

// SetScaling gets a reference to the given ScalingPolicy and assigns it to the Scaling field.
func (o *TaskGroup) SetScaling(v ScalingPolicy) {
	o.Scaling = &v
}

// GetServices returns the Services field value if set, zero value otherwise.
func (o *TaskGroup) GetServices() []Service {
	if o == nil || o.Services == nil {
		var ret []Service
		return ret
	}
	return *o.Services
}

// GetServicesOk returns a tuple with the Services field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *TaskGroup) GetServicesOk() (*[]Service, bool) {
	if o == nil || o.Services == nil {
		return nil, false
	}
	return o.Services, true
}

// HasServices returns a boolean if a field has been set.
func (o *TaskGroup) HasServices() bool {
	if o != nil && o.Services != nil {
		return true
	}

	return false
}

// SetServices gets a reference to the given []Service and assigns it to the Services field.
func (o *TaskGroup) SetServices(v []Service) {
	o.Services = &v
}

// GetShutdownDelay returns the ShutdownDelay field value if set, zero value otherwise.
func (o *TaskGroup) GetShutdownDelay() int64 {
	if o == nil || o.ShutdownDelay == nil {
		var ret int64
		return ret
	}
	return *o.ShutdownDelay
}

// GetShutdownDelayOk returns a tuple with the ShutdownDelay field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *TaskGroup) GetShutdownDelayOk() (*int64, bool) {
	if o == nil || o.ShutdownDelay == nil {
		return nil, false
	}
	return o.ShutdownDelay, true
}

// HasShutdownDelay returns a boolean if a field has been set.
func (o *TaskGroup) HasShutdownDelay() bool {
	if o != nil && o.ShutdownDelay != nil {
		return true
	}

	return false
}

// SetShutdownDelay gets a reference to the given int64 and assigns it to the ShutdownDelay field.
func (o *TaskGroup) SetShutdownDelay(v int64) {
	o.ShutdownDelay = &v
}

// GetSpreads returns the Spreads field value if set, zero value otherwise.
func (o *TaskGroup) GetSpreads() []Spread {
	if o == nil || o.Spreads == nil {
		var ret []Spread
		return ret
	}
	return *o.Spreads
}

// GetSpreadsOk returns a tuple with the Spreads field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *TaskGroup) GetSpreadsOk() (*[]Spread, bool) {
	if o == nil || o.Spreads == nil {
		return nil, false
	}
	return o.Spreads, true
}

// HasSpreads returns a boolean if a field has been set.
func (o *TaskGroup) HasSpreads() bool {
	if o != nil && o.Spreads != nil {
		return true
	}

	return false
}

// SetSpreads gets a reference to the given []Spread and assigns it to the Spreads field.
func (o *TaskGroup) SetSpreads(v []Spread) {
	o.Spreads = &v
}

// GetStopAfterClientDisconnect returns the StopAfterClientDisconnect field value if set, zero value otherwise.
func (o *TaskGroup) GetStopAfterClientDisconnect() int64 {
	if o == nil || o.StopAfterClientDisconnect == nil {
		var ret int64
		return ret
	}
	return *o.StopAfterClientDisconnect
}

// GetStopAfterClientDisconnectOk returns a tuple with the StopAfterClientDisconnect field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *TaskGroup) GetStopAfterClientDisconnectOk() (*int64, bool) {
	if o == nil || o.StopAfterClientDisconnect == nil {
		return nil, false
	}
	return o.StopAfterClientDisconnect, true
}

// HasStopAfterClientDisconnect returns a boolean if a field has been set.
func (o *TaskGroup) HasStopAfterClientDisconnect() bool {
	if o != nil && o.StopAfterClientDisconnect != nil {
		return true
	}

	return false
}

// SetStopAfterClientDisconnect gets a reference to the given int64 and assigns it to the StopAfterClientDisconnect field.
func (o *TaskGroup) SetStopAfterClientDisconnect(v int64) {
	o.StopAfterClientDisconnect = &v
}

// GetTasks returns the Tasks field value if set, zero value otherwise.
func (o *TaskGroup) GetTasks() []Task {
	if o == nil || o.Tasks == nil {
		var ret []Task
		return ret
	}
	return *o.Tasks
}

// GetTasksOk returns a tuple with the Tasks field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *TaskGroup) GetTasksOk() (*[]Task, bool) {
	if o == nil || o.Tasks == nil {
		return nil, false
	}
	return o.Tasks, true
}

// HasTasks returns a boolean if a field has been set.
func (o *TaskGroup) HasTasks() bool {
	if o != nil && o.Tasks != nil {
		return true
	}

	return false
}

// SetTasks gets a reference to the given []Task and assigns it to the Tasks field.
func (o *TaskGroup) SetTasks(v []Task) {
	o.Tasks = &v
}

// GetUpdate returns the Update field value if set, zero value otherwise.
func (o *TaskGroup) GetUpdate() UpdateStrategy {
	if o == nil || o.Update == nil {
		var ret UpdateStrategy
		return ret
	}
	return *o.Update
}

// GetUpdateOk returns a tuple with the Update field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *TaskGroup) GetUpdateOk() (*UpdateStrategy, bool) {
	if o == nil || o.Update == nil {
		return nil, false
	}
	return o.Update, true
}

// HasUpdate returns a boolean if a field has been set.
func (o *TaskGroup) HasUpdate() bool {
	if o != nil && o.Update != nil {
		return true
	}

	return false
}

// SetUpdate gets a reference to the given UpdateStrategy and assigns it to the Update field.
func (o *TaskGroup) SetUpdate(v UpdateStrategy) {
	o.Update = &v
}

// GetVolumes returns the Volumes field value if set, zero value otherwise.
func (o *TaskGroup) GetVolumes() map[string]VolumeRequest {
	if o == nil || o.Volumes == nil {
		var ret map[string]VolumeRequest
		return ret
	}
	return *o.Volumes
}

// GetVolumesOk returns a tuple with the Volumes field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *TaskGroup) GetVolumesOk() (*map[string]VolumeRequest, bool) {
	if o == nil || o.Volumes == nil {
		return nil, false
	}
	return o.Volumes, true
}

// HasVolumes returns a boolean if a field has been set.
func (o *TaskGroup) HasVolumes() bool {
	if o != nil && o.Volumes != nil {
		return true
	}

	return false
}

// SetVolumes gets a reference to the given map[string]VolumeRequest and assigns it to the Volumes field.
func (o *TaskGroup) SetVolumes(v map[string]VolumeRequest) {
	o.Volumes = &v
}

func (o TaskGroup) MarshalJSON() ([]byte, error) {
	toSerialize := map[string]interface{}{}
	if o.Affinities != nil {
		toSerialize["Affinities"] = o.Affinities
	}
	if o.Constraints != nil {
		toSerialize["Constraints"] = o.Constraints
	}
	if o.Consul != nil {
		toSerialize["Consul"] = o.Consul
	}
	if o.Count != nil {
		toSerialize["Count"] = o.Count
	}
	if o.EphemeralDisk != nil {
		toSerialize["EphemeralDisk"] = o.EphemeralDisk
	}
	if o.Meta != nil {
		toSerialize["Meta"] = o.Meta
	}
	if o.Migrate != nil {
		toSerialize["Migrate"] = o.Migrate
	}
	if o.Name != nil {
		toSerialize["Name"] = o.Name
	}
	if o.Networks != nil {
		toSerialize["Networks"] = o.Networks
	}
	if o.ReschedulePolicy != nil {
		toSerialize["ReschedulePolicy"] = o.ReschedulePolicy
	}
	if o.RestartPolicy != nil {
		toSerialize["RestartPolicy"] = o.RestartPolicy
	}
	if o.Scaling != nil {
		toSerialize["Scaling"] = o.Scaling
	}
	if o.Services != nil {
		toSerialize["Services"] = o.Services
	}
	if o.ShutdownDelay != nil {
		toSerialize["ShutdownDelay"] = o.ShutdownDelay
	}
	if o.Spreads != nil {
		toSerialize["Spreads"] = o.Spreads
	}
	if o.StopAfterClientDisconnect != nil {
		toSerialize["StopAfterClientDisconnect"] = o.StopAfterClientDisconnect
	}
	if o.Tasks != nil {
		toSerialize["Tasks"] = o.Tasks
	}
	if o.Update != nil {
		toSerialize["Update"] = o.Update
	}
	if o.Volumes != nil {
		toSerialize["Volumes"] = o.Volumes
	}
	return json.Marshal(toSerialize)
}

type NullableTaskGroup struct {
	value *TaskGroup
	isSet bool
}

func (v NullableTaskGroup) Get() *TaskGroup {
	return v.value
}

func (v *NullableTaskGroup) Set(val *TaskGroup) {
	v.value = val
	v.isSet = true
}

func (v NullableTaskGroup) IsSet() bool {
	return v.isSet
}

func (v *NullableTaskGroup) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableTaskGroup(val *TaskGroup) *NullableTaskGroup {
	return &NullableTaskGroup{value: val, isSet: true}
}

func (v NullableTaskGroup) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableTaskGroup) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}


