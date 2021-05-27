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

// ImageSummary ImageSummary image summary
type ImageSummary struct {
	// containers
	Containers int64 `json:"Containers"`
	// created
	Created int64 `json:"Created"`
	// Id
	Id string `json:"Id"`
	// labels
	Labels map[string]string `json:"Labels"`
	// parent Id
	ParentId string `json:"ParentId"`
	// repo digests
	RepoDigests []string `json:"RepoDigests"`
	// repo tags
	RepoTags []string `json:"RepoTags"`
	// shared size
	SharedSize int64 `json:"SharedSize"`
	// size
	Size int64 `json:"Size"`
	// virtual size
	VirtualSize int64 `json:"VirtualSize"`
}

// NewImageSummary instantiates a new ImageSummary object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewImageSummary(containers int64, created int64, id string, labels map[string]string, parentId string, repoDigests []string, repoTags []string, sharedSize int64, size int64, virtualSize int64) *ImageSummary {
	this := ImageSummary{}
	this.Containers = containers
	this.Created = created
	this.Id = id
	this.Labels = labels
	this.ParentId = parentId
	this.RepoDigests = repoDigests
	this.RepoTags = repoTags
	this.SharedSize = sharedSize
	this.Size = size
	this.VirtualSize = virtualSize
	return &this
}

// NewImageSummaryWithDefaults instantiates a new ImageSummary object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewImageSummaryWithDefaults() *ImageSummary {
	this := ImageSummary{}
	return &this
}

// GetContainers returns the Containers field value
func (o *ImageSummary) GetContainers() int64 {
	if o == nil {
		var ret int64
		return ret
	}

	return o.Containers
}

// GetContainersOk returns a tuple with the Containers field value
// and a boolean to check if the value has been set.
func (o *ImageSummary) GetContainersOk() (*int64, bool) {
	if o == nil  {
		return nil, false
	}
	return &o.Containers, true
}

// SetContainers sets field value
func (o *ImageSummary) SetContainers(v int64) {
	o.Containers = v
}

// GetCreated returns the Created field value
func (o *ImageSummary) GetCreated() int64 {
	if o == nil {
		var ret int64
		return ret
	}

	return o.Created
}

// GetCreatedOk returns a tuple with the Created field value
// and a boolean to check if the value has been set.
func (o *ImageSummary) GetCreatedOk() (*int64, bool) {
	if o == nil  {
		return nil, false
	}
	return &o.Created, true
}

// SetCreated sets field value
func (o *ImageSummary) SetCreated(v int64) {
	o.Created = v
}

// GetId returns the Id field value
func (o *ImageSummary) GetId() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.Id
}

// GetIdOk returns a tuple with the Id field value
// and a boolean to check if the value has been set.
func (o *ImageSummary) GetIdOk() (*string, bool) {
	if o == nil  {
		return nil, false
	}
	return &o.Id, true
}

// SetId sets field value
func (o *ImageSummary) SetId(v string) {
	o.Id = v
}

// GetLabels returns the Labels field value
func (o *ImageSummary) GetLabels() map[string]string {
	if o == nil {
		var ret map[string]string
		return ret
	}

	return o.Labels
}

// GetLabelsOk returns a tuple with the Labels field value
// and a boolean to check if the value has been set.
func (o *ImageSummary) GetLabelsOk() (*map[string]string, bool) {
	if o == nil  {
		return nil, false
	}
	return &o.Labels, true
}

// SetLabels sets field value
func (o *ImageSummary) SetLabels(v map[string]string) {
	o.Labels = v
}

// GetParentId returns the ParentId field value
func (o *ImageSummary) GetParentId() string {
	if o == nil {
		var ret string
		return ret
	}

	return o.ParentId
}

// GetParentIdOk returns a tuple with the ParentId field value
// and a boolean to check if the value has been set.
func (o *ImageSummary) GetParentIdOk() (*string, bool) {
	if o == nil  {
		return nil, false
	}
	return &o.ParentId, true
}

// SetParentId sets field value
func (o *ImageSummary) SetParentId(v string) {
	o.ParentId = v
}

// GetRepoDigests returns the RepoDigests field value
func (o *ImageSummary) GetRepoDigests() []string {
	if o == nil {
		var ret []string
		return ret
	}

	return o.RepoDigests
}

// GetRepoDigestsOk returns a tuple with the RepoDigests field value
// and a boolean to check if the value has been set.
func (o *ImageSummary) GetRepoDigestsOk() (*[]string, bool) {
	if o == nil  {
		return nil, false
	}
	return &o.RepoDigests, true
}

// SetRepoDigests sets field value
func (o *ImageSummary) SetRepoDigests(v []string) {
	o.RepoDigests = v
}

// GetRepoTags returns the RepoTags field value
func (o *ImageSummary) GetRepoTags() []string {
	if o == nil {
		var ret []string
		return ret
	}

	return o.RepoTags
}

// GetRepoTagsOk returns a tuple with the RepoTags field value
// and a boolean to check if the value has been set.
func (o *ImageSummary) GetRepoTagsOk() (*[]string, bool) {
	if o == nil  {
		return nil, false
	}
	return &o.RepoTags, true
}

// SetRepoTags sets field value
func (o *ImageSummary) SetRepoTags(v []string) {
	o.RepoTags = v
}

// GetSharedSize returns the SharedSize field value
func (o *ImageSummary) GetSharedSize() int64 {
	if o == nil {
		var ret int64
		return ret
	}

	return o.SharedSize
}

// GetSharedSizeOk returns a tuple with the SharedSize field value
// and a boolean to check if the value has been set.
func (o *ImageSummary) GetSharedSizeOk() (*int64, bool) {
	if o == nil  {
		return nil, false
	}
	return &o.SharedSize, true
}

// SetSharedSize sets field value
func (o *ImageSummary) SetSharedSize(v int64) {
	o.SharedSize = v
}

// GetSize returns the Size field value
func (o *ImageSummary) GetSize() int64 {
	if o == nil {
		var ret int64
		return ret
	}

	return o.Size
}

// GetSizeOk returns a tuple with the Size field value
// and a boolean to check if the value has been set.
func (o *ImageSummary) GetSizeOk() (*int64, bool) {
	if o == nil  {
		return nil, false
	}
	return &o.Size, true
}

// SetSize sets field value
func (o *ImageSummary) SetSize(v int64) {
	o.Size = v
}

// GetVirtualSize returns the VirtualSize field value
func (o *ImageSummary) GetVirtualSize() int64 {
	if o == nil {
		var ret int64
		return ret
	}

	return o.VirtualSize
}

// GetVirtualSizeOk returns a tuple with the VirtualSize field value
// and a boolean to check if the value has been set.
func (o *ImageSummary) GetVirtualSizeOk() (*int64, bool) {
	if o == nil  {
		return nil, false
	}
	return &o.VirtualSize, true
}

// SetVirtualSize sets field value
func (o *ImageSummary) SetVirtualSize(v int64) {
	o.VirtualSize = v
}

func (o ImageSummary) MarshalJSON() ([]byte, error) {
	toSerialize := map[string]interface{}{}
	if true {
		toSerialize["Containers"] = o.Containers
	}
	if true {
		toSerialize["Created"] = o.Created
	}
	if true {
		toSerialize["Id"] = o.Id
	}
	if true {
		toSerialize["Labels"] = o.Labels
	}
	if true {
		toSerialize["ParentId"] = o.ParentId
	}
	if true {
		toSerialize["RepoDigests"] = o.RepoDigests
	}
	if true {
		toSerialize["RepoTags"] = o.RepoTags
	}
	if true {
		toSerialize["SharedSize"] = o.SharedSize
	}
	if true {
		toSerialize["Size"] = o.Size
	}
	if true {
		toSerialize["VirtualSize"] = o.VirtualSize
	}
	return json.Marshal(toSerialize)
}

type NullableImageSummary struct {
	value *ImageSummary
	isSet bool
}

func (v NullableImageSummary) Get() *ImageSummary {
	return v.value
}

func (v *NullableImageSummary) Set(val *ImageSummary) {
	v.value = val
	v.isSet = true
}

func (v NullableImageSummary) IsSet() bool {
	return v.isSet
}

func (v *NullableImageSummary) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableImageSummary(val *ImageSummary) *NullableImageSummary {
	return &NullableImageSummary{value: val, isSet: true}
}

func (v NullableImageSummary) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableImageSummary) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}


