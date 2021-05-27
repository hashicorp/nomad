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

// ServiceCheck struct for ServiceCheck
type ServiceCheck struct {
	AddressMode *string `json:"AddressMode,omitempty"`
	Args *[]string `json:"Args,omitempty"`
	Body *string `json:"Body,omitempty"`
	CheckRestart *CheckRestart `json:"CheckRestart,omitempty"`
	Command *string `json:"Command,omitempty"`
	Expose *bool `json:"Expose,omitempty"`
	FailuresBeforeCritical *int64 `json:"FailuresBeforeCritical,omitempty"`
	GRPCService *string `json:"GRPCService,omitempty"`
	GRPCUseTLS *bool `json:"GRPCUseTLS,omitempty"`
	Header *map[string][]string `json:"Header,omitempty"`
	// FIXME Id is unused. Remove?
	Id *string `json:"Id,omitempty"`
	InitialStatus *string `json:"InitialStatus,omitempty"`
	// A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years.
	Interval *int64 `json:"Interval,omitempty"`
	Method *string `json:"Method,omitempty"`
	Name *string `json:"Name,omitempty"`
	OnUpdate *string `json:"OnUpdate,omitempty"`
	Path *string `json:"Path,omitempty"`
	PortLabel *string `json:"PortLabel,omitempty"`
	Protocol *string `json:"Protocol,omitempty"`
	SuccessBeforePassing *int64 `json:"SuccessBeforePassing,omitempty"`
	TLSSkipVerify *bool `json:"TLSSkipVerify,omitempty"`
	TaskName *string `json:"TaskName,omitempty"`
	// A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years.
	Timeout *int64 `json:"Timeout,omitempty"`
	Type *string `json:"Type,omitempty"`
}

// NewServiceCheck instantiates a new ServiceCheck object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewServiceCheck() *ServiceCheck {
	this := ServiceCheck{}
	return &this
}

// NewServiceCheckWithDefaults instantiates a new ServiceCheck object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewServiceCheckWithDefaults() *ServiceCheck {
	this := ServiceCheck{}
	return &this
}

// GetAddressMode returns the AddressMode field value if set, zero value otherwise.
func (o *ServiceCheck) GetAddressMode() string {
	if o == nil || o.AddressMode == nil {
		var ret string
		return ret
	}
	return *o.AddressMode
}

// GetAddressModeOk returns a tuple with the AddressMode field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ServiceCheck) GetAddressModeOk() (*string, bool) {
	if o == nil || o.AddressMode == nil {
		return nil, false
	}
	return o.AddressMode, true
}

// HasAddressMode returns a boolean if a field has been set.
func (o *ServiceCheck) HasAddressMode() bool {
	if o != nil && o.AddressMode != nil {
		return true
	}

	return false
}

// SetAddressMode gets a reference to the given string and assigns it to the AddressMode field.
func (o *ServiceCheck) SetAddressMode(v string) {
	o.AddressMode = &v
}

// GetArgs returns the Args field value if set, zero value otherwise.
func (o *ServiceCheck) GetArgs() []string {
	if o == nil || o.Args == nil {
		var ret []string
		return ret
	}
	return *o.Args
}

// GetArgsOk returns a tuple with the Args field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ServiceCheck) GetArgsOk() (*[]string, bool) {
	if o == nil || o.Args == nil {
		return nil, false
	}
	return o.Args, true
}

// HasArgs returns a boolean if a field has been set.
func (o *ServiceCheck) HasArgs() bool {
	if o != nil && o.Args != nil {
		return true
	}

	return false
}

// SetArgs gets a reference to the given []string and assigns it to the Args field.
func (o *ServiceCheck) SetArgs(v []string) {
	o.Args = &v
}

// GetBody returns the Body field value if set, zero value otherwise.
func (o *ServiceCheck) GetBody() string {
	if o == nil || o.Body == nil {
		var ret string
		return ret
	}
	return *o.Body
}

// GetBodyOk returns a tuple with the Body field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ServiceCheck) GetBodyOk() (*string, bool) {
	if o == nil || o.Body == nil {
		return nil, false
	}
	return o.Body, true
}

// HasBody returns a boolean if a field has been set.
func (o *ServiceCheck) HasBody() bool {
	if o != nil && o.Body != nil {
		return true
	}

	return false
}

// SetBody gets a reference to the given string and assigns it to the Body field.
func (o *ServiceCheck) SetBody(v string) {
	o.Body = &v
}

// GetCheckRestart returns the CheckRestart field value if set, zero value otherwise.
func (o *ServiceCheck) GetCheckRestart() CheckRestart {
	if o == nil || o.CheckRestart == nil {
		var ret CheckRestart
		return ret
	}
	return *o.CheckRestart
}

// GetCheckRestartOk returns a tuple with the CheckRestart field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ServiceCheck) GetCheckRestartOk() (*CheckRestart, bool) {
	if o == nil || o.CheckRestart == nil {
		return nil, false
	}
	return o.CheckRestart, true
}

// HasCheckRestart returns a boolean if a field has been set.
func (o *ServiceCheck) HasCheckRestart() bool {
	if o != nil && o.CheckRestart != nil {
		return true
	}

	return false
}

// SetCheckRestart gets a reference to the given CheckRestart and assigns it to the CheckRestart field.
func (o *ServiceCheck) SetCheckRestart(v CheckRestart) {
	o.CheckRestart = &v
}

// GetCommand returns the Command field value if set, zero value otherwise.
func (o *ServiceCheck) GetCommand() string {
	if o == nil || o.Command == nil {
		var ret string
		return ret
	}
	return *o.Command
}

// GetCommandOk returns a tuple with the Command field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ServiceCheck) GetCommandOk() (*string, bool) {
	if o == nil || o.Command == nil {
		return nil, false
	}
	return o.Command, true
}

// HasCommand returns a boolean if a field has been set.
func (o *ServiceCheck) HasCommand() bool {
	if o != nil && o.Command != nil {
		return true
	}

	return false
}

// SetCommand gets a reference to the given string and assigns it to the Command field.
func (o *ServiceCheck) SetCommand(v string) {
	o.Command = &v
}

// GetExpose returns the Expose field value if set, zero value otherwise.
func (o *ServiceCheck) GetExpose() bool {
	if o == nil || o.Expose == nil {
		var ret bool
		return ret
	}
	return *o.Expose
}

// GetExposeOk returns a tuple with the Expose field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ServiceCheck) GetExposeOk() (*bool, bool) {
	if o == nil || o.Expose == nil {
		return nil, false
	}
	return o.Expose, true
}

// HasExpose returns a boolean if a field has been set.
func (o *ServiceCheck) HasExpose() bool {
	if o != nil && o.Expose != nil {
		return true
	}

	return false
}

// SetExpose gets a reference to the given bool and assigns it to the Expose field.
func (o *ServiceCheck) SetExpose(v bool) {
	o.Expose = &v
}

// GetFailuresBeforeCritical returns the FailuresBeforeCritical field value if set, zero value otherwise.
func (o *ServiceCheck) GetFailuresBeforeCritical() int64 {
	if o == nil || o.FailuresBeforeCritical == nil {
		var ret int64
		return ret
	}
	return *o.FailuresBeforeCritical
}

// GetFailuresBeforeCriticalOk returns a tuple with the FailuresBeforeCritical field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ServiceCheck) GetFailuresBeforeCriticalOk() (*int64, bool) {
	if o == nil || o.FailuresBeforeCritical == nil {
		return nil, false
	}
	return o.FailuresBeforeCritical, true
}

// HasFailuresBeforeCritical returns a boolean if a field has been set.
func (o *ServiceCheck) HasFailuresBeforeCritical() bool {
	if o != nil && o.FailuresBeforeCritical != nil {
		return true
	}

	return false
}

// SetFailuresBeforeCritical gets a reference to the given int64 and assigns it to the FailuresBeforeCritical field.
func (o *ServiceCheck) SetFailuresBeforeCritical(v int64) {
	o.FailuresBeforeCritical = &v
}

// GetGRPCService returns the GRPCService field value if set, zero value otherwise.
func (o *ServiceCheck) GetGRPCService() string {
	if o == nil || o.GRPCService == nil {
		var ret string
		return ret
	}
	return *o.GRPCService
}

// GetGRPCServiceOk returns a tuple with the GRPCService field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ServiceCheck) GetGRPCServiceOk() (*string, bool) {
	if o == nil || o.GRPCService == nil {
		return nil, false
	}
	return o.GRPCService, true
}

// HasGRPCService returns a boolean if a field has been set.
func (o *ServiceCheck) HasGRPCService() bool {
	if o != nil && o.GRPCService != nil {
		return true
	}

	return false
}

// SetGRPCService gets a reference to the given string and assigns it to the GRPCService field.
func (o *ServiceCheck) SetGRPCService(v string) {
	o.GRPCService = &v
}

// GetGRPCUseTLS returns the GRPCUseTLS field value if set, zero value otherwise.
func (o *ServiceCheck) GetGRPCUseTLS() bool {
	if o == nil || o.GRPCUseTLS == nil {
		var ret bool
		return ret
	}
	return *o.GRPCUseTLS
}

// GetGRPCUseTLSOk returns a tuple with the GRPCUseTLS field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ServiceCheck) GetGRPCUseTLSOk() (*bool, bool) {
	if o == nil || o.GRPCUseTLS == nil {
		return nil, false
	}
	return o.GRPCUseTLS, true
}

// HasGRPCUseTLS returns a boolean if a field has been set.
func (o *ServiceCheck) HasGRPCUseTLS() bool {
	if o != nil && o.GRPCUseTLS != nil {
		return true
	}

	return false
}

// SetGRPCUseTLS gets a reference to the given bool and assigns it to the GRPCUseTLS field.
func (o *ServiceCheck) SetGRPCUseTLS(v bool) {
	o.GRPCUseTLS = &v
}

// GetHeader returns the Header field value if set, zero value otherwise.
func (o *ServiceCheck) GetHeader() map[string][]string {
	if o == nil || o.Header == nil {
		var ret map[string][]string
		return ret
	}
	return *o.Header
}

// GetHeaderOk returns a tuple with the Header field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ServiceCheck) GetHeaderOk() (*map[string][]string, bool) {
	if o == nil || o.Header == nil {
		return nil, false
	}
	return o.Header, true
}

// HasHeader returns a boolean if a field has been set.
func (o *ServiceCheck) HasHeader() bool {
	if o != nil && o.Header != nil {
		return true
	}

	return false
}

// SetHeader gets a reference to the given map[string][]string and assigns it to the Header field.
func (o *ServiceCheck) SetHeader(v map[string][]string) {
	o.Header = &v
}

// GetId returns the Id field value if set, zero value otherwise.
func (o *ServiceCheck) GetId() string {
	if o == nil || o.Id == nil {
		var ret string
		return ret
	}
	return *o.Id
}

// GetIdOk returns a tuple with the Id field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ServiceCheck) GetIdOk() (*string, bool) {
	if o == nil || o.Id == nil {
		return nil, false
	}
	return o.Id, true
}

// HasId returns a boolean if a field has been set.
func (o *ServiceCheck) HasId() bool {
	if o != nil && o.Id != nil {
		return true
	}

	return false
}

// SetId gets a reference to the given string and assigns it to the Id field.
func (o *ServiceCheck) SetId(v string) {
	o.Id = &v
}

// GetInitialStatus returns the InitialStatus field value if set, zero value otherwise.
func (o *ServiceCheck) GetInitialStatus() string {
	if o == nil || o.InitialStatus == nil {
		var ret string
		return ret
	}
	return *o.InitialStatus
}

// GetInitialStatusOk returns a tuple with the InitialStatus field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ServiceCheck) GetInitialStatusOk() (*string, bool) {
	if o == nil || o.InitialStatus == nil {
		return nil, false
	}
	return o.InitialStatus, true
}

// HasInitialStatus returns a boolean if a field has been set.
func (o *ServiceCheck) HasInitialStatus() bool {
	if o != nil && o.InitialStatus != nil {
		return true
	}

	return false
}

// SetInitialStatus gets a reference to the given string and assigns it to the InitialStatus field.
func (o *ServiceCheck) SetInitialStatus(v string) {
	o.InitialStatus = &v
}

// GetInterval returns the Interval field value if set, zero value otherwise.
func (o *ServiceCheck) GetInterval() int64 {
	if o == nil || o.Interval == nil {
		var ret int64
		return ret
	}
	return *o.Interval
}

// GetIntervalOk returns a tuple with the Interval field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ServiceCheck) GetIntervalOk() (*int64, bool) {
	if o == nil || o.Interval == nil {
		return nil, false
	}
	return o.Interval, true
}

// HasInterval returns a boolean if a field has been set.
func (o *ServiceCheck) HasInterval() bool {
	if o != nil && o.Interval != nil {
		return true
	}

	return false
}

// SetInterval gets a reference to the given int64 and assigns it to the Interval field.
func (o *ServiceCheck) SetInterval(v int64) {
	o.Interval = &v
}

// GetMethod returns the Method field value if set, zero value otherwise.
func (o *ServiceCheck) GetMethod() string {
	if o == nil || o.Method == nil {
		var ret string
		return ret
	}
	return *o.Method
}

// GetMethodOk returns a tuple with the Method field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ServiceCheck) GetMethodOk() (*string, bool) {
	if o == nil || o.Method == nil {
		return nil, false
	}
	return o.Method, true
}

// HasMethod returns a boolean if a field has been set.
func (o *ServiceCheck) HasMethod() bool {
	if o != nil && o.Method != nil {
		return true
	}

	return false
}

// SetMethod gets a reference to the given string and assigns it to the Method field.
func (o *ServiceCheck) SetMethod(v string) {
	o.Method = &v
}

// GetName returns the Name field value if set, zero value otherwise.
func (o *ServiceCheck) GetName() string {
	if o == nil || o.Name == nil {
		var ret string
		return ret
	}
	return *o.Name
}

// GetNameOk returns a tuple with the Name field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ServiceCheck) GetNameOk() (*string, bool) {
	if o == nil || o.Name == nil {
		return nil, false
	}
	return o.Name, true
}

// HasName returns a boolean if a field has been set.
func (o *ServiceCheck) HasName() bool {
	if o != nil && o.Name != nil {
		return true
	}

	return false
}

// SetName gets a reference to the given string and assigns it to the Name field.
func (o *ServiceCheck) SetName(v string) {
	o.Name = &v
}

// GetOnUpdate returns the OnUpdate field value if set, zero value otherwise.
func (o *ServiceCheck) GetOnUpdate() string {
	if o == nil || o.OnUpdate == nil {
		var ret string
		return ret
	}
	return *o.OnUpdate
}

// GetOnUpdateOk returns a tuple with the OnUpdate field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ServiceCheck) GetOnUpdateOk() (*string, bool) {
	if o == nil || o.OnUpdate == nil {
		return nil, false
	}
	return o.OnUpdate, true
}

// HasOnUpdate returns a boolean if a field has been set.
func (o *ServiceCheck) HasOnUpdate() bool {
	if o != nil && o.OnUpdate != nil {
		return true
	}

	return false
}

// SetOnUpdate gets a reference to the given string and assigns it to the OnUpdate field.
func (o *ServiceCheck) SetOnUpdate(v string) {
	o.OnUpdate = &v
}

// GetPath returns the Path field value if set, zero value otherwise.
func (o *ServiceCheck) GetPath() string {
	if o == nil || o.Path == nil {
		var ret string
		return ret
	}
	return *o.Path
}

// GetPathOk returns a tuple with the Path field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ServiceCheck) GetPathOk() (*string, bool) {
	if o == nil || o.Path == nil {
		return nil, false
	}
	return o.Path, true
}

// HasPath returns a boolean if a field has been set.
func (o *ServiceCheck) HasPath() bool {
	if o != nil && o.Path != nil {
		return true
	}

	return false
}

// SetPath gets a reference to the given string and assigns it to the Path field.
func (o *ServiceCheck) SetPath(v string) {
	o.Path = &v
}

// GetPortLabel returns the PortLabel field value if set, zero value otherwise.
func (o *ServiceCheck) GetPortLabel() string {
	if o == nil || o.PortLabel == nil {
		var ret string
		return ret
	}
	return *o.PortLabel
}

// GetPortLabelOk returns a tuple with the PortLabel field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ServiceCheck) GetPortLabelOk() (*string, bool) {
	if o == nil || o.PortLabel == nil {
		return nil, false
	}
	return o.PortLabel, true
}

// HasPortLabel returns a boolean if a field has been set.
func (o *ServiceCheck) HasPortLabel() bool {
	if o != nil && o.PortLabel != nil {
		return true
	}

	return false
}

// SetPortLabel gets a reference to the given string and assigns it to the PortLabel field.
func (o *ServiceCheck) SetPortLabel(v string) {
	o.PortLabel = &v
}

// GetProtocol returns the Protocol field value if set, zero value otherwise.
func (o *ServiceCheck) GetProtocol() string {
	if o == nil || o.Protocol == nil {
		var ret string
		return ret
	}
	return *o.Protocol
}

// GetProtocolOk returns a tuple with the Protocol field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ServiceCheck) GetProtocolOk() (*string, bool) {
	if o == nil || o.Protocol == nil {
		return nil, false
	}
	return o.Protocol, true
}

// HasProtocol returns a boolean if a field has been set.
func (o *ServiceCheck) HasProtocol() bool {
	if o != nil && o.Protocol != nil {
		return true
	}

	return false
}

// SetProtocol gets a reference to the given string and assigns it to the Protocol field.
func (o *ServiceCheck) SetProtocol(v string) {
	o.Protocol = &v
}

// GetSuccessBeforePassing returns the SuccessBeforePassing field value if set, zero value otherwise.
func (o *ServiceCheck) GetSuccessBeforePassing() int64 {
	if o == nil || o.SuccessBeforePassing == nil {
		var ret int64
		return ret
	}
	return *o.SuccessBeforePassing
}

// GetSuccessBeforePassingOk returns a tuple with the SuccessBeforePassing field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ServiceCheck) GetSuccessBeforePassingOk() (*int64, bool) {
	if o == nil || o.SuccessBeforePassing == nil {
		return nil, false
	}
	return o.SuccessBeforePassing, true
}

// HasSuccessBeforePassing returns a boolean if a field has been set.
func (o *ServiceCheck) HasSuccessBeforePassing() bool {
	if o != nil && o.SuccessBeforePassing != nil {
		return true
	}

	return false
}

// SetSuccessBeforePassing gets a reference to the given int64 and assigns it to the SuccessBeforePassing field.
func (o *ServiceCheck) SetSuccessBeforePassing(v int64) {
	o.SuccessBeforePassing = &v
}

// GetTLSSkipVerify returns the TLSSkipVerify field value if set, zero value otherwise.
func (o *ServiceCheck) GetTLSSkipVerify() bool {
	if o == nil || o.TLSSkipVerify == nil {
		var ret bool
		return ret
	}
	return *o.TLSSkipVerify
}

// GetTLSSkipVerifyOk returns a tuple with the TLSSkipVerify field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ServiceCheck) GetTLSSkipVerifyOk() (*bool, bool) {
	if o == nil || o.TLSSkipVerify == nil {
		return nil, false
	}
	return o.TLSSkipVerify, true
}

// HasTLSSkipVerify returns a boolean if a field has been set.
func (o *ServiceCheck) HasTLSSkipVerify() bool {
	if o != nil && o.TLSSkipVerify != nil {
		return true
	}

	return false
}

// SetTLSSkipVerify gets a reference to the given bool and assigns it to the TLSSkipVerify field.
func (o *ServiceCheck) SetTLSSkipVerify(v bool) {
	o.TLSSkipVerify = &v
}

// GetTaskName returns the TaskName field value if set, zero value otherwise.
func (o *ServiceCheck) GetTaskName() string {
	if o == nil || o.TaskName == nil {
		var ret string
		return ret
	}
	return *o.TaskName
}

// GetTaskNameOk returns a tuple with the TaskName field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ServiceCheck) GetTaskNameOk() (*string, bool) {
	if o == nil || o.TaskName == nil {
		return nil, false
	}
	return o.TaskName, true
}

// HasTaskName returns a boolean if a field has been set.
func (o *ServiceCheck) HasTaskName() bool {
	if o != nil && o.TaskName != nil {
		return true
	}

	return false
}

// SetTaskName gets a reference to the given string and assigns it to the TaskName field.
func (o *ServiceCheck) SetTaskName(v string) {
	o.TaskName = &v
}

// GetTimeout returns the Timeout field value if set, zero value otherwise.
func (o *ServiceCheck) GetTimeout() int64 {
	if o == nil || o.Timeout == nil {
		var ret int64
		return ret
	}
	return *o.Timeout
}

// GetTimeoutOk returns a tuple with the Timeout field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ServiceCheck) GetTimeoutOk() (*int64, bool) {
	if o == nil || o.Timeout == nil {
		return nil, false
	}
	return o.Timeout, true
}

// HasTimeout returns a boolean if a field has been set.
func (o *ServiceCheck) HasTimeout() bool {
	if o != nil && o.Timeout != nil {
		return true
	}

	return false
}

// SetTimeout gets a reference to the given int64 and assigns it to the Timeout field.
func (o *ServiceCheck) SetTimeout(v int64) {
	o.Timeout = &v
}

// GetType returns the Type field value if set, zero value otherwise.
func (o *ServiceCheck) GetType() string {
	if o == nil || o.Type == nil {
		var ret string
		return ret
	}
	return *o.Type
}

// GetTypeOk returns a tuple with the Type field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ServiceCheck) GetTypeOk() (*string, bool) {
	if o == nil || o.Type == nil {
		return nil, false
	}
	return o.Type, true
}

// HasType returns a boolean if a field has been set.
func (o *ServiceCheck) HasType() bool {
	if o != nil && o.Type != nil {
		return true
	}

	return false
}

// SetType gets a reference to the given string and assigns it to the Type field.
func (o *ServiceCheck) SetType(v string) {
	o.Type = &v
}

func (o ServiceCheck) MarshalJSON() ([]byte, error) {
	toSerialize := map[string]interface{}{}
	if o.AddressMode != nil {
		toSerialize["AddressMode"] = o.AddressMode
	}
	if o.Args != nil {
		toSerialize["Args"] = o.Args
	}
	if o.Body != nil {
		toSerialize["Body"] = o.Body
	}
	if o.CheckRestart != nil {
		toSerialize["CheckRestart"] = o.CheckRestart
	}
	if o.Command != nil {
		toSerialize["Command"] = o.Command
	}
	if o.Expose != nil {
		toSerialize["Expose"] = o.Expose
	}
	if o.FailuresBeforeCritical != nil {
		toSerialize["FailuresBeforeCritical"] = o.FailuresBeforeCritical
	}
	if o.GRPCService != nil {
		toSerialize["GRPCService"] = o.GRPCService
	}
	if o.GRPCUseTLS != nil {
		toSerialize["GRPCUseTLS"] = o.GRPCUseTLS
	}
	if o.Header != nil {
		toSerialize["Header"] = o.Header
	}
	if o.Id != nil {
		toSerialize["Id"] = o.Id
	}
	if o.InitialStatus != nil {
		toSerialize["InitialStatus"] = o.InitialStatus
	}
	if o.Interval != nil {
		toSerialize["Interval"] = o.Interval
	}
	if o.Method != nil {
		toSerialize["Method"] = o.Method
	}
	if o.Name != nil {
		toSerialize["Name"] = o.Name
	}
	if o.OnUpdate != nil {
		toSerialize["OnUpdate"] = o.OnUpdate
	}
	if o.Path != nil {
		toSerialize["Path"] = o.Path
	}
	if o.PortLabel != nil {
		toSerialize["PortLabel"] = o.PortLabel
	}
	if o.Protocol != nil {
		toSerialize["Protocol"] = o.Protocol
	}
	if o.SuccessBeforePassing != nil {
		toSerialize["SuccessBeforePassing"] = o.SuccessBeforePassing
	}
	if o.TLSSkipVerify != nil {
		toSerialize["TLSSkipVerify"] = o.TLSSkipVerify
	}
	if o.TaskName != nil {
		toSerialize["TaskName"] = o.TaskName
	}
	if o.Timeout != nil {
		toSerialize["Timeout"] = o.Timeout
	}
	if o.Type != nil {
		toSerialize["Type"] = o.Type
	}
	return json.Marshal(toSerialize)
}

type NullableServiceCheck struct {
	value *ServiceCheck
	isSet bool
}

func (v NullableServiceCheck) Get() *ServiceCheck {
	return v.value
}

func (v *NullableServiceCheck) Set(val *ServiceCheck) {
	v.value = val
	v.isSet = true
}

func (v NullableServiceCheck) IsSet() bool {
	return v.isSet
}

func (v *NullableServiceCheck) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableServiceCheck(val *ServiceCheck) *NullableServiceCheck {
	return &NullableServiceCheck{value: val, isSet: true}
}

func (v NullableServiceCheck) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableServiceCheck) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}


