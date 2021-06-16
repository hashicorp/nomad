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

// AllocListStub AllocListStub is used to return a subset of alloc information
type AllocListStub struct {
	AllocatedResources *AllocatedResources `json:"AllocatedResources,omitempty"`
	ClientDescription *string `json:"ClientDescription,omitempty"`
	ClientStatus *string `json:"ClientStatus,omitempty"`
	CreateIndex *int32 `json:"CreateIndex,omitempty"`
	CreateTime *int64 `json:"CreateTime,omitempty"`
	DeploymentStatus *AllocDeploymentStatus `json:"DeploymentStatus,omitempty"`
	DesiredDescription *string `json:"DesiredDescription,omitempty"`
	DesiredStatus *string `json:"DesiredStatus,omitempty"`
	DesiredTransition *DesiredTransition `json:"DesiredTransition,omitempty"`
	EvalID *string `json:"EvalID,omitempty"`
	FollowupEvalID *string `json:"FollowupEvalID,omitempty"`
	ID *string `json:"ID,omitempty"`
	JobID *string `json:"JobID,omitempty"`
	JobType *string `json:"JobType,omitempty"`
	JobVersion *int32 `json:"JobVersion,omitempty"`
	ModifyIndex *int32 `json:"ModifyIndex,omitempty"`
	ModifyTime *int64 `json:"ModifyTime,omitempty"`
	Name *string `json:"Name,omitempty"`
	Namespace *string `json:"Namespace,omitempty"`
	NodeID *string `json:"NodeID,omitempty"`
	NodeName *string `json:"NodeName,omitempty"`
	PreemptedAllocations *[]string `json:"PreemptedAllocations,omitempty"`
	PreemptedByAllocation *string `json:"PreemptedByAllocation,omitempty"`
	RescheduleTracker *RescheduleTracker `json:"RescheduleTracker,omitempty"`
	TaskGroup *string `json:"TaskGroup,omitempty"`
	TaskStates *map[string]TaskState `json:"TaskStates,omitempty"`
}

// NewAllocListStub instantiates a new AllocListStub object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewAllocListStub() *AllocListStub {
	this := AllocListStub{}
	return &this
}

// NewAllocListStubWithDefaults instantiates a new AllocListStub object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewAllocListStubWithDefaults() *AllocListStub {
	this := AllocListStub{}
	return &this
}

// GetAllocatedResources returns the AllocatedResources field value if set, zero value otherwise.
func (o *AllocListStub) GetAllocatedResources() AllocatedResources {
	if o == nil || o.AllocatedResources == nil {
		var ret AllocatedResources
		return ret
	}
	return *o.AllocatedResources
}

// GetAllocatedResourcesOk returns a tuple with the AllocatedResources field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *AllocListStub) GetAllocatedResourcesOk() (*AllocatedResources, bool) {
	if o == nil || o.AllocatedResources == nil {
		return nil, false
	}
	return o.AllocatedResources, true
}

// HasAllocatedResources returns a boolean if a field has been set.
func (o *AllocListStub) HasAllocatedResources() bool {
	if o != nil && o.AllocatedResources != nil {
		return true
	}

	return false
}

// SetAllocatedResources gets a reference to the given AllocatedResources and assigns it to the AllocatedResources field.
func (o *AllocListStub) SetAllocatedResources(v AllocatedResources) {
	o.AllocatedResources = &v
}

// GetClientDescription returns the ClientDescription field value if set, zero value otherwise.
func (o *AllocListStub) GetClientDescription() string {
	if o == nil || o.ClientDescription == nil {
		var ret string
		return ret
	}
	return *o.ClientDescription
}

// GetClientDescriptionOk returns a tuple with the ClientDescription field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *AllocListStub) GetClientDescriptionOk() (*string, bool) {
	if o == nil || o.ClientDescription == nil {
		return nil, false
	}
	return o.ClientDescription, true
}

// HasClientDescription returns a boolean if a field has been set.
func (o *AllocListStub) HasClientDescription() bool {
	if o != nil && o.ClientDescription != nil {
		return true
	}

	return false
}

// SetClientDescription gets a reference to the given string and assigns it to the ClientDescription field.
func (o *AllocListStub) SetClientDescription(v string) {
	o.ClientDescription = &v
}

// GetClientStatus returns the ClientStatus field value if set, zero value otherwise.
func (o *AllocListStub) GetClientStatus() string {
	if o == nil || o.ClientStatus == nil {
		var ret string
		return ret
	}
	return *o.ClientStatus
}

// GetClientStatusOk returns a tuple with the ClientStatus field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *AllocListStub) GetClientStatusOk() (*string, bool) {
	if o == nil || o.ClientStatus == nil {
		return nil, false
	}
	return o.ClientStatus, true
}

// HasClientStatus returns a boolean if a field has been set.
func (o *AllocListStub) HasClientStatus() bool {
	if o != nil && o.ClientStatus != nil {
		return true
	}

	return false
}

// SetClientStatus gets a reference to the given string and assigns it to the ClientStatus field.
func (o *AllocListStub) SetClientStatus(v string) {
	o.ClientStatus = &v
}

// GetCreateIndex returns the CreateIndex field value if set, zero value otherwise.
func (o *AllocListStub) GetCreateIndex() int32 {
	if o == nil || o.CreateIndex == nil {
		var ret int32
		return ret
	}
	return *o.CreateIndex
}

// GetCreateIndexOk returns a tuple with the CreateIndex field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *AllocListStub) GetCreateIndexOk() (*int32, bool) {
	if o == nil || o.CreateIndex == nil {
		return nil, false
	}
	return o.CreateIndex, true
}

// HasCreateIndex returns a boolean if a field has been set.
func (o *AllocListStub) HasCreateIndex() bool {
	if o != nil && o.CreateIndex != nil {
		return true
	}

	return false
}

// SetCreateIndex gets a reference to the given int32 and assigns it to the CreateIndex field.
func (o *AllocListStub) SetCreateIndex(v int32) {
	o.CreateIndex = &v
}

// GetCreateTime returns the CreateTime field value if set, zero value otherwise.
func (o *AllocListStub) GetCreateTime() int64 {
	if o == nil || o.CreateTime == nil {
		var ret int64
		return ret
	}
	return *o.CreateTime
}

// GetCreateTimeOk returns a tuple with the CreateTime field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *AllocListStub) GetCreateTimeOk() (*int64, bool) {
	if o == nil || o.CreateTime == nil {
		return nil, false
	}
	return o.CreateTime, true
}

// HasCreateTime returns a boolean if a field has been set.
func (o *AllocListStub) HasCreateTime() bool {
	if o != nil && o.CreateTime != nil {
		return true
	}

	return false
}

// SetCreateTime gets a reference to the given int64 and assigns it to the CreateTime field.
func (o *AllocListStub) SetCreateTime(v int64) {
	o.CreateTime = &v
}

// GetDeploymentStatus returns the DeploymentStatus field value if set, zero value otherwise.
func (o *AllocListStub) GetDeploymentStatus() AllocDeploymentStatus {
	if o == nil || o.DeploymentStatus == nil {
		var ret AllocDeploymentStatus
		return ret
	}
	return *o.DeploymentStatus
}

// GetDeploymentStatusOk returns a tuple with the DeploymentStatus field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *AllocListStub) GetDeploymentStatusOk() (*AllocDeploymentStatus, bool) {
	if o == nil || o.DeploymentStatus == nil {
		return nil, false
	}
	return o.DeploymentStatus, true
}

// HasDeploymentStatus returns a boolean if a field has been set.
func (o *AllocListStub) HasDeploymentStatus() bool {
	if o != nil && o.DeploymentStatus != nil {
		return true
	}

	return false
}

// SetDeploymentStatus gets a reference to the given AllocDeploymentStatus and assigns it to the DeploymentStatus field.
func (o *AllocListStub) SetDeploymentStatus(v AllocDeploymentStatus) {
	o.DeploymentStatus = &v
}

// GetDesiredDescription returns the DesiredDescription field value if set, zero value otherwise.
func (o *AllocListStub) GetDesiredDescription() string {
	if o == nil || o.DesiredDescription == nil {
		var ret string
		return ret
	}
	return *o.DesiredDescription
}

// GetDesiredDescriptionOk returns a tuple with the DesiredDescription field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *AllocListStub) GetDesiredDescriptionOk() (*string, bool) {
	if o == nil || o.DesiredDescription == nil {
		return nil, false
	}
	return o.DesiredDescription, true
}

// HasDesiredDescription returns a boolean if a field has been set.
func (o *AllocListStub) HasDesiredDescription() bool {
	if o != nil && o.DesiredDescription != nil {
		return true
	}

	return false
}

// SetDesiredDescription gets a reference to the given string and assigns it to the DesiredDescription field.
func (o *AllocListStub) SetDesiredDescription(v string) {
	o.DesiredDescription = &v
}

// GetDesiredStatus returns the DesiredStatus field value if set, zero value otherwise.
func (o *AllocListStub) GetDesiredStatus() string {
	if o == nil || o.DesiredStatus == nil {
		var ret string
		return ret
	}
	return *o.DesiredStatus
}

// GetDesiredStatusOk returns a tuple with the DesiredStatus field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *AllocListStub) GetDesiredStatusOk() (*string, bool) {
	if o == nil || o.DesiredStatus == nil {
		return nil, false
	}
	return o.DesiredStatus, true
}

// HasDesiredStatus returns a boolean if a field has been set.
func (o *AllocListStub) HasDesiredStatus() bool {
	if o != nil && o.DesiredStatus != nil {
		return true
	}

	return false
}

// SetDesiredStatus gets a reference to the given string and assigns it to the DesiredStatus field.
func (o *AllocListStub) SetDesiredStatus(v string) {
	o.DesiredStatus = &v
}

// GetDesiredTransition returns the DesiredTransition field value if set, zero value otherwise.
func (o *AllocListStub) GetDesiredTransition() DesiredTransition {
	if o == nil || o.DesiredTransition == nil {
		var ret DesiredTransition
		return ret
	}
	return *o.DesiredTransition
}

// GetDesiredTransitionOk returns a tuple with the DesiredTransition field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *AllocListStub) GetDesiredTransitionOk() (*DesiredTransition, bool) {
	if o == nil || o.DesiredTransition == nil {
		return nil, false
	}
	return o.DesiredTransition, true
}

// HasDesiredTransition returns a boolean if a field has been set.
func (o *AllocListStub) HasDesiredTransition() bool {
	if o != nil && o.DesiredTransition != nil {
		return true
	}

	return false
}

// SetDesiredTransition gets a reference to the given DesiredTransition and assigns it to the DesiredTransition field.
func (o *AllocListStub) SetDesiredTransition(v DesiredTransition) {
	o.DesiredTransition = &v
}

// GetEvalID returns the EvalID field value if set, zero value otherwise.
func (o *AllocListStub) GetEvalID() string {
	if o == nil || o.EvalID == nil {
		var ret string
		return ret
	}
	return *o.EvalID
}

// GetEvalIDOk returns a tuple with the EvalID field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *AllocListStub) GetEvalIDOk() (*string, bool) {
	if o == nil || o.EvalID == nil {
		return nil, false
	}
	return o.EvalID, true
}

// HasEvalID returns a boolean if a field has been set.
func (o *AllocListStub) HasEvalID() bool {
	if o != nil && o.EvalID != nil {
		return true
	}

	return false
}

// SetEvalID gets a reference to the given string and assigns it to the EvalID field.
func (o *AllocListStub) SetEvalID(v string) {
	o.EvalID = &v
}

// GetFollowupEvalID returns the FollowupEvalID field value if set, zero value otherwise.
func (o *AllocListStub) GetFollowupEvalID() string {
	if o == nil || o.FollowupEvalID == nil {
		var ret string
		return ret
	}
	return *o.FollowupEvalID
}

// GetFollowupEvalIDOk returns a tuple with the FollowupEvalID field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *AllocListStub) GetFollowupEvalIDOk() (*string, bool) {
	if o == nil || o.FollowupEvalID == nil {
		return nil, false
	}
	return o.FollowupEvalID, true
}

// HasFollowupEvalID returns a boolean if a field has been set.
func (o *AllocListStub) HasFollowupEvalID() bool {
	if o != nil && o.FollowupEvalID != nil {
		return true
	}

	return false
}

// SetFollowupEvalID gets a reference to the given string and assigns it to the FollowupEvalID field.
func (o *AllocListStub) SetFollowupEvalID(v string) {
	o.FollowupEvalID = &v
}

// GetID returns the ID field value if set, zero value otherwise.
func (o *AllocListStub) GetID() string {
	if o == nil || o.ID == nil {
		var ret string
		return ret
	}
	return *o.ID
}

// GetIDOk returns a tuple with the ID field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *AllocListStub) GetIDOk() (*string, bool) {
	if o == nil || o.ID == nil {
		return nil, false
	}
	return o.ID, true
}

// HasID returns a boolean if a field has been set.
func (o *AllocListStub) HasID() bool {
	if o != nil && o.ID != nil {
		return true
	}

	return false
}

// SetID gets a reference to the given string and assigns it to the ID field.
func (o *AllocListStub) SetID(v string) {
	o.ID = &v
}

// GetJobID returns the JobID field value if set, zero value otherwise.
func (o *AllocListStub) GetJobID() string {
	if o == nil || o.JobID == nil {
		var ret string
		return ret
	}
	return *o.JobID
}

// GetJobIDOk returns a tuple with the JobID field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *AllocListStub) GetJobIDOk() (*string, bool) {
	if o == nil || o.JobID == nil {
		return nil, false
	}
	return o.JobID, true
}

// HasJobID returns a boolean if a field has been set.
func (o *AllocListStub) HasJobID() bool {
	if o != nil && o.JobID != nil {
		return true
	}

	return false
}

// SetJobID gets a reference to the given string and assigns it to the JobID field.
func (o *AllocListStub) SetJobID(v string) {
	o.JobID = &v
}

// GetJobType returns the JobType field value if set, zero value otherwise.
func (o *AllocListStub) GetJobType() string {
	if o == nil || o.JobType == nil {
		var ret string
		return ret
	}
	return *o.JobType
}

// GetJobTypeOk returns a tuple with the JobType field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *AllocListStub) GetJobTypeOk() (*string, bool) {
	if o == nil || o.JobType == nil {
		return nil, false
	}
	return o.JobType, true
}

// HasJobType returns a boolean if a field has been set.
func (o *AllocListStub) HasJobType() bool {
	if o != nil && o.JobType != nil {
		return true
	}

	return false
}

// SetJobType gets a reference to the given string and assigns it to the JobType field.
func (o *AllocListStub) SetJobType(v string) {
	o.JobType = &v
}

// GetJobVersion returns the JobVersion field value if set, zero value otherwise.
func (o *AllocListStub) GetJobVersion() int32 {
	if o == nil || o.JobVersion == nil {
		var ret int32
		return ret
	}
	return *o.JobVersion
}

// GetJobVersionOk returns a tuple with the JobVersion field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *AllocListStub) GetJobVersionOk() (*int32, bool) {
	if o == nil || o.JobVersion == nil {
		return nil, false
	}
	return o.JobVersion, true
}

// HasJobVersion returns a boolean if a field has been set.
func (o *AllocListStub) HasJobVersion() bool {
	if o != nil && o.JobVersion != nil {
		return true
	}

	return false
}

// SetJobVersion gets a reference to the given int32 and assigns it to the JobVersion field.
func (o *AllocListStub) SetJobVersion(v int32) {
	o.JobVersion = &v
}

// GetModifyIndex returns the ModifyIndex field value if set, zero value otherwise.
func (o *AllocListStub) GetModifyIndex() int32 {
	if o == nil || o.ModifyIndex == nil {
		var ret int32
		return ret
	}
	return *o.ModifyIndex
}

// GetModifyIndexOk returns a tuple with the ModifyIndex field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *AllocListStub) GetModifyIndexOk() (*int32, bool) {
	if o == nil || o.ModifyIndex == nil {
		return nil, false
	}
	return o.ModifyIndex, true
}

// HasModifyIndex returns a boolean if a field has been set.
func (o *AllocListStub) HasModifyIndex() bool {
	if o != nil && o.ModifyIndex != nil {
		return true
	}

	return false
}

// SetModifyIndex gets a reference to the given int32 and assigns it to the ModifyIndex field.
func (o *AllocListStub) SetModifyIndex(v int32) {
	o.ModifyIndex = &v
}

// GetModifyTime returns the ModifyTime field value if set, zero value otherwise.
func (o *AllocListStub) GetModifyTime() int64 {
	if o == nil || o.ModifyTime == nil {
		var ret int64
		return ret
	}
	return *o.ModifyTime
}

// GetModifyTimeOk returns a tuple with the ModifyTime field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *AllocListStub) GetModifyTimeOk() (*int64, bool) {
	if o == nil || o.ModifyTime == nil {
		return nil, false
	}
	return o.ModifyTime, true
}

// HasModifyTime returns a boolean if a field has been set.
func (o *AllocListStub) HasModifyTime() bool {
	if o != nil && o.ModifyTime != nil {
		return true
	}

	return false
}

// SetModifyTime gets a reference to the given int64 and assigns it to the ModifyTime field.
func (o *AllocListStub) SetModifyTime(v int64) {
	o.ModifyTime = &v
}

// GetName returns the Name field value if set, zero value otherwise.
func (o *AllocListStub) GetName() string {
	if o == nil || o.Name == nil {
		var ret string
		return ret
	}
	return *o.Name
}

// GetNameOk returns a tuple with the Name field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *AllocListStub) GetNameOk() (*string, bool) {
	if o == nil || o.Name == nil {
		return nil, false
	}
	return o.Name, true
}

// HasName returns a boolean if a field has been set.
func (o *AllocListStub) HasName() bool {
	if o != nil && o.Name != nil {
		return true
	}

	return false
}

// SetName gets a reference to the given string and assigns it to the Name field.
func (o *AllocListStub) SetName(v string) {
	o.Name = &v
}

// GetNamespace returns the Namespace field value if set, zero value otherwise.
func (o *AllocListStub) GetNamespace() string {
	if o == nil || o.Namespace == nil {
		var ret string
		return ret
	}
	return *o.Namespace
}

// GetNamespaceOk returns a tuple with the Namespace field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *AllocListStub) GetNamespaceOk() (*string, bool) {
	if o == nil || o.Namespace == nil {
		return nil, false
	}
	return o.Namespace, true
}

// HasNamespace returns a boolean if a field has been set.
func (o *AllocListStub) HasNamespace() bool {
	if o != nil && o.Namespace != nil {
		return true
	}

	return false
}

// SetNamespace gets a reference to the given string and assigns it to the Namespace field.
func (o *AllocListStub) SetNamespace(v string) {
	o.Namespace = &v
}

// GetNodeID returns the NodeID field value if set, zero value otherwise.
func (o *AllocListStub) GetNodeID() string {
	if o == nil || o.NodeID == nil {
		var ret string
		return ret
	}
	return *o.NodeID
}

// GetNodeIDOk returns a tuple with the NodeID field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *AllocListStub) GetNodeIDOk() (*string, bool) {
	if o == nil || o.NodeID == nil {
		return nil, false
	}
	return o.NodeID, true
}

// HasNodeID returns a boolean if a field has been set.
func (o *AllocListStub) HasNodeID() bool {
	if o != nil && o.NodeID != nil {
		return true
	}

	return false
}

// SetNodeID gets a reference to the given string and assigns it to the NodeID field.
func (o *AllocListStub) SetNodeID(v string) {
	o.NodeID = &v
}

// GetNodeName returns the NodeName field value if set, zero value otherwise.
func (o *AllocListStub) GetNodeName() string {
	if o == nil || o.NodeName == nil {
		var ret string
		return ret
	}
	return *o.NodeName
}

// GetNodeNameOk returns a tuple with the NodeName field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *AllocListStub) GetNodeNameOk() (*string, bool) {
	if o == nil || o.NodeName == nil {
		return nil, false
	}
	return o.NodeName, true
}

// HasNodeName returns a boolean if a field has been set.
func (o *AllocListStub) HasNodeName() bool {
	if o != nil && o.NodeName != nil {
		return true
	}

	return false
}

// SetNodeName gets a reference to the given string and assigns it to the NodeName field.
func (o *AllocListStub) SetNodeName(v string) {
	o.NodeName = &v
}

// GetPreemptedAllocations returns the PreemptedAllocations field value if set, zero value otherwise.
func (o *AllocListStub) GetPreemptedAllocations() []string {
	if o == nil || o.PreemptedAllocations == nil {
		var ret []string
		return ret
	}
	return *o.PreemptedAllocations
}

// GetPreemptedAllocationsOk returns a tuple with the PreemptedAllocations field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *AllocListStub) GetPreemptedAllocationsOk() (*[]string, bool) {
	if o == nil || o.PreemptedAllocations == nil {
		return nil, false
	}
	return o.PreemptedAllocations, true
}

// HasPreemptedAllocations returns a boolean if a field has been set.
func (o *AllocListStub) HasPreemptedAllocations() bool {
	if o != nil && o.PreemptedAllocations != nil {
		return true
	}

	return false
}

// SetPreemptedAllocations gets a reference to the given []string and assigns it to the PreemptedAllocations field.
func (o *AllocListStub) SetPreemptedAllocations(v []string) {
	o.PreemptedAllocations = &v
}

// GetPreemptedByAllocation returns the PreemptedByAllocation field value if set, zero value otherwise.
func (o *AllocListStub) GetPreemptedByAllocation() string {
	if o == nil || o.PreemptedByAllocation == nil {
		var ret string
		return ret
	}
	return *o.PreemptedByAllocation
}

// GetPreemptedByAllocationOk returns a tuple with the PreemptedByAllocation field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *AllocListStub) GetPreemptedByAllocationOk() (*string, bool) {
	if o == nil || o.PreemptedByAllocation == nil {
		return nil, false
	}
	return o.PreemptedByAllocation, true
}

// HasPreemptedByAllocation returns a boolean if a field has been set.
func (o *AllocListStub) HasPreemptedByAllocation() bool {
	if o != nil && o.PreemptedByAllocation != nil {
		return true
	}

	return false
}

// SetPreemptedByAllocation gets a reference to the given string and assigns it to the PreemptedByAllocation field.
func (o *AllocListStub) SetPreemptedByAllocation(v string) {
	o.PreemptedByAllocation = &v
}

// GetRescheduleTracker returns the RescheduleTracker field value if set, zero value otherwise.
func (o *AllocListStub) GetRescheduleTracker() RescheduleTracker {
	if o == nil || o.RescheduleTracker == nil {
		var ret RescheduleTracker
		return ret
	}
	return *o.RescheduleTracker
}

// GetRescheduleTrackerOk returns a tuple with the RescheduleTracker field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *AllocListStub) GetRescheduleTrackerOk() (*RescheduleTracker, bool) {
	if o == nil || o.RescheduleTracker == nil {
		return nil, false
	}
	return o.RescheduleTracker, true
}

// HasRescheduleTracker returns a boolean if a field has been set.
func (o *AllocListStub) HasRescheduleTracker() bool {
	if o != nil && o.RescheduleTracker != nil {
		return true
	}

	return false
}

// SetRescheduleTracker gets a reference to the given RescheduleTracker and assigns it to the RescheduleTracker field.
func (o *AllocListStub) SetRescheduleTracker(v RescheduleTracker) {
	o.RescheduleTracker = &v
}

// GetTaskGroup returns the TaskGroup field value if set, zero value otherwise.
func (o *AllocListStub) GetTaskGroup() string {
	if o == nil || o.TaskGroup == nil {
		var ret string
		return ret
	}
	return *o.TaskGroup
}

// GetTaskGroupOk returns a tuple with the TaskGroup field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *AllocListStub) GetTaskGroupOk() (*string, bool) {
	if o == nil || o.TaskGroup == nil {
		return nil, false
	}
	return o.TaskGroup, true
}

// HasTaskGroup returns a boolean if a field has been set.
func (o *AllocListStub) HasTaskGroup() bool {
	if o != nil && o.TaskGroup != nil {
		return true
	}

	return false
}

// SetTaskGroup gets a reference to the given string and assigns it to the TaskGroup field.
func (o *AllocListStub) SetTaskGroup(v string) {
	o.TaskGroup = &v
}

// GetTaskStates returns the TaskStates field value if set, zero value otherwise.
func (o *AllocListStub) GetTaskStates() map[string]TaskState {
	if o == nil || o.TaskStates == nil {
		var ret map[string]TaskState
		return ret
	}
	return *o.TaskStates
}

// GetTaskStatesOk returns a tuple with the TaskStates field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *AllocListStub) GetTaskStatesOk() (*map[string]TaskState, bool) {
	if o == nil || o.TaskStates == nil {
		return nil, false
	}
	return o.TaskStates, true
}

// HasTaskStates returns a boolean if a field has been set.
func (o *AllocListStub) HasTaskStates() bool {
	if o != nil && o.TaskStates != nil {
		return true
	}

	return false
}

// SetTaskStates gets a reference to the given map[string]TaskState and assigns it to the TaskStates field.
func (o *AllocListStub) SetTaskStates(v map[string]TaskState) {
	o.TaskStates = &v
}

func (o AllocListStub) MarshalJSON() ([]byte, error) {
	toSerialize := map[string]interface{}{}
	if o.AllocatedResources != nil {
		toSerialize["AllocatedResources"] = o.AllocatedResources
	}
	if o.ClientDescription != nil {
		toSerialize["ClientDescription"] = o.ClientDescription
	}
	if o.ClientStatus != nil {
		toSerialize["ClientStatus"] = o.ClientStatus
	}
	if o.CreateIndex != nil {
		toSerialize["CreateIndex"] = o.CreateIndex
	}
	if o.CreateTime != nil {
		toSerialize["CreateTime"] = o.CreateTime
	}
	if o.DeploymentStatus != nil {
		toSerialize["DeploymentStatus"] = o.DeploymentStatus
	}
	if o.DesiredDescription != nil {
		toSerialize["DesiredDescription"] = o.DesiredDescription
	}
	if o.DesiredStatus != nil {
		toSerialize["DesiredStatus"] = o.DesiredStatus
	}
	if o.DesiredTransition != nil {
		toSerialize["DesiredTransition"] = o.DesiredTransition
	}
	if o.EvalID != nil {
		toSerialize["EvalID"] = o.EvalID
	}
	if o.FollowupEvalID != nil {
		toSerialize["FollowupEvalID"] = o.FollowupEvalID
	}
	if o.ID != nil {
		toSerialize["ID"] = o.ID
	}
	if o.JobID != nil {
		toSerialize["JobID"] = o.JobID
	}
	if o.JobType != nil {
		toSerialize["JobType"] = o.JobType
	}
	if o.JobVersion != nil {
		toSerialize["JobVersion"] = o.JobVersion
	}
	if o.ModifyIndex != nil {
		toSerialize["ModifyIndex"] = o.ModifyIndex
	}
	if o.ModifyTime != nil {
		toSerialize["ModifyTime"] = o.ModifyTime
	}
	if o.Name != nil {
		toSerialize["Name"] = o.Name
	}
	if o.Namespace != nil {
		toSerialize["Namespace"] = o.Namespace
	}
	if o.NodeID != nil {
		toSerialize["NodeID"] = o.NodeID
	}
	if o.NodeName != nil {
		toSerialize["NodeName"] = o.NodeName
	}
	if o.PreemptedAllocations != nil {
		toSerialize["PreemptedAllocations"] = o.PreemptedAllocations
	}
	if o.PreemptedByAllocation != nil {
		toSerialize["PreemptedByAllocation"] = o.PreemptedByAllocation
	}
	if o.RescheduleTracker != nil {
		toSerialize["RescheduleTracker"] = o.RescheduleTracker
	}
	if o.TaskGroup != nil {
		toSerialize["TaskGroup"] = o.TaskGroup
	}
	if o.TaskStates != nil {
		toSerialize["TaskStates"] = o.TaskStates
	}
	return json.Marshal(toSerialize)
}

type NullableAllocListStub struct {
	value *AllocListStub
	isSet bool
}

func (v NullableAllocListStub) Get() *AllocListStub {
	return v.value
}

func (v *NullableAllocListStub) Set(val *AllocListStub) {
	v.value = val
	v.isSet = true
}

func (v NullableAllocListStub) IsSet() bool {
	return v.isSet
}

func (v *NullableAllocListStub) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableAllocListStub(val *AllocListStub) *NullableAllocListStub {
	return &NullableAllocListStub{value: val, isSet: true}
}

func (v NullableAllocListStub) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableAllocListStub) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}


