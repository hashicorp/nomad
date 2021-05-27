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
	"bytes"
	_context "context"
	_ioutil "io/ioutil"
	_nethttp "net/http"
	_neturl "net/url"
	"strings"
)

// Linger please
var (
	_ _context.Context
)

// JobsApiService JobsApi service
type JobsApiService service

type ApiGetJobAllocationsRequest struct {
	ctx _context.Context
	ApiService *JobsApiService
	jobName string
	all *int32
	region *string
	stale *string
	prefix *string
	namespace *string
	perPage *int32
	nextToken *string
	index *int32
	wait *int32
	xNomadToken *string
}

func (r ApiGetJobAllocationsRequest) All(all int32) ApiGetJobAllocationsRequest {
	r.all = &all
	return r
}
func (r ApiGetJobAllocationsRequest) Region(region string) ApiGetJobAllocationsRequest {
	r.region = &region
	return r
}
func (r ApiGetJobAllocationsRequest) Stale(stale string) ApiGetJobAllocationsRequest {
	r.stale = &stale
	return r
}
func (r ApiGetJobAllocationsRequest) Prefix(prefix string) ApiGetJobAllocationsRequest {
	r.prefix = &prefix
	return r
}
func (r ApiGetJobAllocationsRequest) Namespace(namespace string) ApiGetJobAllocationsRequest {
	r.namespace = &namespace
	return r
}
func (r ApiGetJobAllocationsRequest) PerPage(perPage int32) ApiGetJobAllocationsRequest {
	r.perPage = &perPage
	return r
}
func (r ApiGetJobAllocationsRequest) NextToken(nextToken string) ApiGetJobAllocationsRequest {
	r.nextToken = &nextToken
	return r
}
func (r ApiGetJobAllocationsRequest) Index(index int32) ApiGetJobAllocationsRequest {
	r.index = &index
	return r
}
func (r ApiGetJobAllocationsRequest) Wait(wait int32) ApiGetJobAllocationsRequest {
	r.wait = &wait
	return r
}
func (r ApiGetJobAllocationsRequest) XNomadToken(xNomadToken string) ApiGetJobAllocationsRequest {
	r.xNomadToken = &xNomadToken
	return r
}

func (r ApiGetJobAllocationsRequest) Execute() ([]AllocListStub, *_nethttp.Response, error) {
	return r.ApiService.GetJobAllocationsExecute(r)
}

/*
 * GetJobAllocations Gets information about a single job's allocations. See https://www.nomadproject.io/api-docs/allocations#list-allocations. 
 * @param ctx _context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
 * @param jobName The job identifier.
 * @return ApiGetJobAllocationsRequest
 */
func (a *JobsApiService) GetJobAllocations(ctx _context.Context, jobName string) ApiGetJobAllocationsRequest {
	return ApiGetJobAllocationsRequest{
		ApiService: a,
		ctx: ctx,
		jobName: jobName,
	}
}

/*
 * Execute executes the request
 * @return []AllocListStub
 */
func (a *JobsApiService) GetJobAllocationsExecute(r ApiGetJobAllocationsRequest) ([]AllocListStub, *_nethttp.Response, error) {
	var (
		localVarHTTPMethod   = _nethttp.MethodGet
		localVarPostBody     interface{}
		localVarFormFileName string
		localVarFileName     string
		localVarFileBytes    []byte
		localVarReturnValue  []AllocListStub
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "JobsApiService.GetJobAllocations")
	if err != nil {
		return localVarReturnValue, nil, GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/job/{jobName}/allocations"
	localVarPath = strings.Replace(localVarPath, "{"+"jobName"+"}", _neturl.PathEscape(parameterToString(r.jobName, "")), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := _neturl.Values{}
	localVarFormParams := _neturl.Values{}

	if r.all != nil {
		localVarQueryParams.Add("all", parameterToString(*r.all, ""))
	}
	if r.region != nil {
		localVarQueryParams.Add("region", parameterToString(*r.region, ""))
	}
	if r.stale != nil {
		localVarQueryParams.Add("stale", parameterToString(*r.stale, ""))
	}
	if r.prefix != nil {
		localVarQueryParams.Add("prefix", parameterToString(*r.prefix, ""))
	}
	if r.namespace != nil {
		localVarQueryParams.Add("namespace", parameterToString(*r.namespace, ""))
	}
	if r.perPage != nil {
		localVarQueryParams.Add("per_page", parameterToString(*r.perPage, ""))
	}
	if r.nextToken != nil {
		localVarQueryParams.Add("next_token", parameterToString(*r.nextToken, ""))
	}
	if r.index != nil {
		localVarQueryParams.Add("index", parameterToString(*r.index, ""))
	}
	if r.wait != nil {
		localVarQueryParams.Add("wait", parameterToString(*r.wait, ""))
	}
	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{"application/json"}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	if r.xNomadToken != nil {
		localVarHeaderParams["X-Nomad-Token"] = parameterToString(*r.xNomadToken, "")
	}
	req, err := a.client.prepareRequest(r.ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, localVarFormFileName, localVarFileName, localVarFileBytes)
	if err != nil {
		return localVarReturnValue, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(req)
	if err != nil || localVarHTTPResponse == nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	localVarBody, err := _ioutil.ReadAll(localVarHTTPResponse.Body)
	localVarHTTPResponse.Body.Close()
	localVarHTTPResponse.Body = _ioutil.NopCloser(bytes.NewBuffer(localVarBody))
	if err != nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	if localVarHTTPResponse.StatusCode >= 300 {
		newErr := GenericOpenAPIError{
			body:  localVarBody,
			error: localVarHTTPResponse.Status,
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	err = a.client.decode(&localVarReturnValue, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
	if err != nil {
		newErr := GenericOpenAPIError{
			body:  localVarBody,
			error: err.Error(),
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	return localVarReturnValue, localVarHTTPResponse, nil
}

type ApiGetJobEvaluationsRequest struct {
	ctx _context.Context
	ApiService *JobsApiService
	jobName string
	region *string
	stale *string
	prefix *string
	namespace *string
	perPage *int32
	nextToken *string
	index *int32
	wait *int32
	xNomadToken *string
}

func (r ApiGetJobEvaluationsRequest) Region(region string) ApiGetJobEvaluationsRequest {
	r.region = &region
	return r
}
func (r ApiGetJobEvaluationsRequest) Stale(stale string) ApiGetJobEvaluationsRequest {
	r.stale = &stale
	return r
}
func (r ApiGetJobEvaluationsRequest) Prefix(prefix string) ApiGetJobEvaluationsRequest {
	r.prefix = &prefix
	return r
}
func (r ApiGetJobEvaluationsRequest) Namespace(namespace string) ApiGetJobEvaluationsRequest {
	r.namespace = &namespace
	return r
}
func (r ApiGetJobEvaluationsRequest) PerPage(perPage int32) ApiGetJobEvaluationsRequest {
	r.perPage = &perPage
	return r
}
func (r ApiGetJobEvaluationsRequest) NextToken(nextToken string) ApiGetJobEvaluationsRequest {
	r.nextToken = &nextToken
	return r
}
func (r ApiGetJobEvaluationsRequest) Index(index int32) ApiGetJobEvaluationsRequest {
	r.index = &index
	return r
}
func (r ApiGetJobEvaluationsRequest) Wait(wait int32) ApiGetJobEvaluationsRequest {
	r.wait = &wait
	return r
}
func (r ApiGetJobEvaluationsRequest) XNomadToken(xNomadToken string) ApiGetJobEvaluationsRequest {
	r.xNomadToken = &xNomadToken
	return r
}

func (r ApiGetJobEvaluationsRequest) Execute() ([]Evaluation, *_nethttp.Response, error) {
	return r.ApiService.GetJobEvaluationsExecute(r)
}

/*
 * GetJobEvaluations Gets information about a single job's evaluations. See [documentation](https://www.nomadproject.io/api-docs/evaluations#list-evaluations). 
 * @param ctx _context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
 * @param jobName The job identifier.
 * @return ApiGetJobEvaluationsRequest
 */
func (a *JobsApiService) GetJobEvaluations(ctx _context.Context, jobName string) ApiGetJobEvaluationsRequest {
	return ApiGetJobEvaluationsRequest{
		ApiService: a,
		ctx: ctx,
		jobName: jobName,
	}
}

/*
 * Execute executes the request
 * @return []Evaluation
 */
func (a *JobsApiService) GetJobEvaluationsExecute(r ApiGetJobEvaluationsRequest) ([]Evaluation, *_nethttp.Response, error) {
	var (
		localVarHTTPMethod   = _nethttp.MethodGet
		localVarPostBody     interface{}
		localVarFormFileName string
		localVarFileName     string
		localVarFileBytes    []byte
		localVarReturnValue  []Evaluation
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "JobsApiService.GetJobEvaluations")
	if err != nil {
		return localVarReturnValue, nil, GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/job/{jobName}/evaluations"
	localVarPath = strings.Replace(localVarPath, "{"+"jobName"+"}", _neturl.PathEscape(parameterToString(r.jobName, "")), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := _neturl.Values{}
	localVarFormParams := _neturl.Values{}

	if r.region != nil {
		localVarQueryParams.Add("region", parameterToString(*r.region, ""))
	}
	if r.stale != nil {
		localVarQueryParams.Add("stale", parameterToString(*r.stale, ""))
	}
	if r.prefix != nil {
		localVarQueryParams.Add("prefix", parameterToString(*r.prefix, ""))
	}
	if r.namespace != nil {
		localVarQueryParams.Add("namespace", parameterToString(*r.namespace, ""))
	}
	if r.perPage != nil {
		localVarQueryParams.Add("per_page", parameterToString(*r.perPage, ""))
	}
	if r.nextToken != nil {
		localVarQueryParams.Add("next_token", parameterToString(*r.nextToken, ""))
	}
	if r.index != nil {
		localVarQueryParams.Add("index", parameterToString(*r.index, ""))
	}
	if r.wait != nil {
		localVarQueryParams.Add("wait", parameterToString(*r.wait, ""))
	}
	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{"application/json"}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	if r.xNomadToken != nil {
		localVarHeaderParams["X-Nomad-Token"] = parameterToString(*r.xNomadToken, "")
	}
	req, err := a.client.prepareRequest(r.ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, localVarFormFileName, localVarFileName, localVarFileBytes)
	if err != nil {
		return localVarReturnValue, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(req)
	if err != nil || localVarHTTPResponse == nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	localVarBody, err := _ioutil.ReadAll(localVarHTTPResponse.Body)
	localVarHTTPResponse.Body.Close()
	localVarHTTPResponse.Body = _ioutil.NopCloser(bytes.NewBuffer(localVarBody))
	if err != nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	if localVarHTTPResponse.StatusCode >= 300 {
		newErr := GenericOpenAPIError{
			body:  localVarBody,
			error: localVarHTTPResponse.Status,
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	err = a.client.decode(&localVarReturnValue, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
	if err != nil {
		newErr := GenericOpenAPIError{
			body:  localVarBody,
			error: err.Error(),
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	return localVarReturnValue, localVarHTTPResponse, nil
}

type ApiGetJobSummaryRequest struct {
	ctx _context.Context
	ApiService *JobsApiService
	jobName string
	region *string
	stale *string
	prefix *string
	namespace *string
	perPage *int32
	nextToken *string
	index *int32
	wait *int32
	xNomadToken *string
}

func (r ApiGetJobSummaryRequest) Region(region string) ApiGetJobSummaryRequest {
	r.region = &region
	return r
}
func (r ApiGetJobSummaryRequest) Stale(stale string) ApiGetJobSummaryRequest {
	r.stale = &stale
	return r
}
func (r ApiGetJobSummaryRequest) Prefix(prefix string) ApiGetJobSummaryRequest {
	r.prefix = &prefix
	return r
}
func (r ApiGetJobSummaryRequest) Namespace(namespace string) ApiGetJobSummaryRequest {
	r.namespace = &namespace
	return r
}
func (r ApiGetJobSummaryRequest) PerPage(perPage int32) ApiGetJobSummaryRequest {
	r.perPage = &perPage
	return r
}
func (r ApiGetJobSummaryRequest) NextToken(nextToken string) ApiGetJobSummaryRequest {
	r.nextToken = &nextToken
	return r
}
func (r ApiGetJobSummaryRequest) Index(index int32) ApiGetJobSummaryRequest {
	r.index = &index
	return r
}
func (r ApiGetJobSummaryRequest) Wait(wait int32) ApiGetJobSummaryRequest {
	r.wait = &wait
	return r
}
func (r ApiGetJobSummaryRequest) XNomadToken(xNomadToken string) ApiGetJobSummaryRequest {
	r.xNomadToken = &xNomadToken
	return r
}

func (r ApiGetJobSummaryRequest) Execute() (JobSummary, *_nethttp.Response, error) {
	return r.ApiService.GetJobSummaryExecute(r)
}

/*
 * GetJobSummary This endpoint reads summary information about a job. https://www.nomadproject.io/api-docs/jobs#read-job-summary.
 * @param ctx _context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
 * @param jobName The job identifier.
 * @return ApiGetJobSummaryRequest
 */
func (a *JobsApiService) GetJobSummary(ctx _context.Context, jobName string) ApiGetJobSummaryRequest {
	return ApiGetJobSummaryRequest{
		ApiService: a,
		ctx: ctx,
		jobName: jobName,
	}
}

/*
 * Execute executes the request
 * @return JobSummary
 */
func (a *JobsApiService) GetJobSummaryExecute(r ApiGetJobSummaryRequest) (JobSummary, *_nethttp.Response, error) {
	var (
		localVarHTTPMethod   = _nethttp.MethodGet
		localVarPostBody     interface{}
		localVarFormFileName string
		localVarFileName     string
		localVarFileBytes    []byte
		localVarReturnValue  JobSummary
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "JobsApiService.GetJobSummary")
	if err != nil {
		return localVarReturnValue, nil, GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/job/{jobName}/summary"
	localVarPath = strings.Replace(localVarPath, "{"+"jobName"+"}", _neturl.PathEscape(parameterToString(r.jobName, "")), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := _neturl.Values{}
	localVarFormParams := _neturl.Values{}

	if r.region != nil {
		localVarQueryParams.Add("region", parameterToString(*r.region, ""))
	}
	if r.stale != nil {
		localVarQueryParams.Add("stale", parameterToString(*r.stale, ""))
	}
	if r.prefix != nil {
		localVarQueryParams.Add("prefix", parameterToString(*r.prefix, ""))
	}
	if r.namespace != nil {
		localVarQueryParams.Add("namespace", parameterToString(*r.namespace, ""))
	}
	if r.perPage != nil {
		localVarQueryParams.Add("per_page", parameterToString(*r.perPage, ""))
	}
	if r.nextToken != nil {
		localVarQueryParams.Add("next_token", parameterToString(*r.nextToken, ""))
	}
	if r.index != nil {
		localVarQueryParams.Add("index", parameterToString(*r.index, ""))
	}
	if r.wait != nil {
		localVarQueryParams.Add("wait", parameterToString(*r.wait, ""))
	}
	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{"application/json"}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	if r.xNomadToken != nil {
		localVarHeaderParams["X-Nomad-Token"] = parameterToString(*r.xNomadToken, "")
	}
	req, err := a.client.prepareRequest(r.ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, localVarFormFileName, localVarFileName, localVarFileBytes)
	if err != nil {
		return localVarReturnValue, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(req)
	if err != nil || localVarHTTPResponse == nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	localVarBody, err := _ioutil.ReadAll(localVarHTTPResponse.Body)
	localVarHTTPResponse.Body.Close()
	localVarHTTPResponse.Body = _ioutil.NopCloser(bytes.NewBuffer(localVarBody))
	if err != nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	if localVarHTTPResponse.StatusCode >= 300 {
		newErr := GenericOpenAPIError{
			body:  localVarBody,
			error: localVarHTTPResponse.Status,
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	err = a.client.decode(&localVarReturnValue, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
	if err != nil {
		newErr := GenericOpenAPIError{
			body:  localVarBody,
			error: err.Error(),
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	return localVarReturnValue, localVarHTTPResponse, nil
}

type ApiGetJobsRequest struct {
	ctx _context.Context
	ApiService *JobsApiService
	region *string
	stale *string
	prefix *string
	namespace *string
	perPage *int32
	nextToken *string
	index *int32
	wait *int32
	xNomadToken *string
}

func (r ApiGetJobsRequest) Region(region string) ApiGetJobsRequest {
	r.region = &region
	return r
}
func (r ApiGetJobsRequest) Stale(stale string) ApiGetJobsRequest {
	r.stale = &stale
	return r
}
func (r ApiGetJobsRequest) Prefix(prefix string) ApiGetJobsRequest {
	r.prefix = &prefix
	return r
}
func (r ApiGetJobsRequest) Namespace(namespace string) ApiGetJobsRequest {
	r.namespace = &namespace
	return r
}
func (r ApiGetJobsRequest) PerPage(perPage int32) ApiGetJobsRequest {
	r.perPage = &perPage
	return r
}
func (r ApiGetJobsRequest) NextToken(nextToken string) ApiGetJobsRequest {
	r.nextToken = &nextToken
	return r
}
func (r ApiGetJobsRequest) Index(index int32) ApiGetJobsRequest {
	r.index = &index
	return r
}
func (r ApiGetJobsRequest) Wait(wait int32) ApiGetJobsRequest {
	r.wait = &wait
	return r
}
func (r ApiGetJobsRequest) XNomadToken(xNomadToken string) ApiGetJobsRequest {
	r.xNomadToken = &xNomadToken
	return r
}

func (r ApiGetJobsRequest) Execute() ([]JobListStub, *_nethttp.Response, error) {
	return r.ApiService.GetJobsExecute(r)
}

/*
 * GetJobs List all known jobs registered with Nomad. See https://www.nomadproject.io/api-docs/jobs#list-jobs.
 * <p>This endpoint lists all known jobs in the system registered with Nomad.</p>
<table>
    <thead>
        <tr>
            <th>Method</th>
            <th>Path</th>
            <th>Produces</th>
        </tr>
    </thead>
    <tbody>
        <tr>
            <td><code>GET</code></td>
            <td><code>/v1/jobs</code></td>
            <td><code>application/json</code></td>
        </tr>
    </tbody>
</table>
<p>The table below shows this endpoint&#39;s support for
    <a href="/api-docs#blocking-queries">blocking queries</a> and
    <a href="/api-docs#acls">required ACLs</a>.
</p>
<table>
    <thead>
        <tr>
            <th>Blocking Queries</th>
            <th>ACL Required</th>
        </tr>
    </thead>
    <tbody>
        <tr>
            <td><code>YES</code></td>
            <td><code>namespace:list-jobs</code></td>
        </tr>
    </tbody>
</table>
<h3 id="parameters">Parameters</h3>
<ul>
    <li>
        <p><code>prefix</code> <code>(string: &quot;&quot;)</code> - Specifies a string to filter jobs on based on
            an index prefix. This is specified as a query string parameter.</p>
    </li>
    <li>
        <p><code>namespace</code> <code>(string: &quot;default&quot;)</code> - Specifies the target namespace. Specifying
            <code>*</code> would return all jobs across all the authorized namespaces.
        </p>
    </li>
</ul>
<h3 id="sample-request">Sample Request</h3>
<pre><code class="language-shell-session">$ curl https://localhost:4646/v1/jobs
</code></pre>
<pre><code class="language-shell-session">$ curl https://localhost:4646/v1/jobs?prefix=team
</code></pre>
<pre><code class="language-shell-session">$ curl https://localhost:4646/v1/jobs?namespace=*&amp;prefix=team
</code></pre>
<h3 id="sample-response">Sample Response</h3>
<pre><code class="language-json">[
  {
    &quot;ID&quot;: &quot;example&quot;,
    &quot;ParentID&quot;: &quot;&quot;,
    &quot;Name&quot;: &quot;example&quot;,
    &quot;Type&quot;: &quot;service&quot;,
    &quot;Priority&quot;: 50,
    &quot;Status&quot;: &quot;pending&quot;,
    &quot;StatusDescription&quot;: &quot;&quot;,
    &quot;JobSummary&quot;: {
      &quot;JobID&quot;: &quot;example&quot;,
      &quot;Namespace&quot;: &quot;default&quot;,
      &quot;Summary&quot;: {
        &quot;cache&quot;: {
          &quot;Queued&quot;: 1,
          &quot;Complete&quot;: 1,
          &quot;Failed&quot;: 0,
          &quot;Running&quot;: 0,
          &quot;Starting&quot;: 0,
          &quot;Lost&quot;: 0
        }
      },
      &quot;Children&quot;: {
        &quot;Pending&quot;: 0,
        &quot;Running&quot;: 0,
        &quot;Dead&quot;: 0
      },
      &quot;CreateIndex&quot;: 52,
      &quot;ModifyIndex&quot;: 96
    },
    &quot;CreateIndex&quot;: 52,
    &quot;ModifyIndex&quot;: 93,
    &quot;JobModifyIndex&quot;: 52
  }
]
</code></pre>
 * @param ctx _context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
 * @return ApiGetJobsRequest
 */
func (a *JobsApiService) GetJobs(ctx _context.Context) ApiGetJobsRequest {
	return ApiGetJobsRequest{
		ApiService: a,
		ctx: ctx,
	}
}

/*
 * Execute executes the request
 * @return []JobListStub
 */
func (a *JobsApiService) GetJobsExecute(r ApiGetJobsRequest) ([]JobListStub, *_nethttp.Response, error) {
	var (
		localVarHTTPMethod   = _nethttp.MethodGet
		localVarPostBody     interface{}
		localVarFormFileName string
		localVarFileName     string
		localVarFileBytes    []byte
		localVarReturnValue  []JobListStub
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "JobsApiService.GetJobs")
	if err != nil {
		return localVarReturnValue, nil, GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/jobs"

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := _neturl.Values{}
	localVarFormParams := _neturl.Values{}

	if r.region != nil {
		localVarQueryParams.Add("region", parameterToString(*r.region, ""))
	}
	if r.stale != nil {
		localVarQueryParams.Add("stale", parameterToString(*r.stale, ""))
	}
	if r.prefix != nil {
		localVarQueryParams.Add("prefix", parameterToString(*r.prefix, ""))
	}
	if r.namespace != nil {
		localVarQueryParams.Add("namespace", parameterToString(*r.namespace, ""))
	}
	if r.perPage != nil {
		localVarQueryParams.Add("per_page", parameterToString(*r.perPage, ""))
	}
	if r.nextToken != nil {
		localVarQueryParams.Add("next_token", parameterToString(*r.nextToken, ""))
	}
	if r.index != nil {
		localVarQueryParams.Add("index", parameterToString(*r.index, ""))
	}
	if r.wait != nil {
		localVarQueryParams.Add("wait", parameterToString(*r.wait, ""))
	}
	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{"application/json"}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	if r.xNomadToken != nil {
		localVarHeaderParams["X-Nomad-Token"] = parameterToString(*r.xNomadToken, "")
	}
	req, err := a.client.prepareRequest(r.ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, localVarFormFileName, localVarFileName, localVarFileBytes)
	if err != nil {
		return localVarReturnValue, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(req)
	if err != nil || localVarHTTPResponse == nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	localVarBody, err := _ioutil.ReadAll(localVarHTTPResponse.Body)
	localVarHTTPResponse.Body.Close()
	localVarHTTPResponse.Body = _ioutil.NopCloser(bytes.NewBuffer(localVarBody))
	if err != nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	if localVarHTTPResponse.StatusCode >= 300 {
		newErr := GenericOpenAPIError{
			body:  localVarBody,
			error: localVarHTTPResponse.Status,
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	err = a.client.decode(&localVarReturnValue, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
	if err != nil {
		newErr := GenericOpenAPIError{
			body:  localVarBody,
			error: err.Error(),
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	return localVarReturnValue, localVarHTTPResponse, nil
}

type ApiPostJobEvaluateRequestRequest struct {
	ctx _context.Context
	ApiService *JobsApiService
	jobName string
	jobEvaluateRequest *JobEvaluateRequest
	region *string
	namespace *string
	xNomadToken *string
}

func (r ApiPostJobEvaluateRequestRequest) JobEvaluateRequest(jobEvaluateRequest JobEvaluateRequest) ApiPostJobEvaluateRequestRequest {
	r.jobEvaluateRequest = &jobEvaluateRequest
	return r
}
func (r ApiPostJobEvaluateRequestRequest) Region(region string) ApiPostJobEvaluateRequestRequest {
	r.region = &region
	return r
}
func (r ApiPostJobEvaluateRequestRequest) Namespace(namespace string) ApiPostJobEvaluateRequestRequest {
	r.namespace = &namespace
	return r
}
func (r ApiPostJobEvaluateRequestRequest) XNomadToken(xNomadToken string) ApiPostJobEvaluateRequestRequest {
	r.xNomadToken = &xNomadToken
	return r
}

func (r ApiPostJobEvaluateRequestRequest) Execute() (JobRegisterResponse, *_nethttp.Response, error) {
	return r.ApiService.PostJobEvaluateRequestExecute(r)
}

/*
 * PostJobEvaluateRequest Creates a new evaluation for the given job. This can be used to force run the scheduling logic if necessary. See https://www.nomadproject.io/api-docs/jobs#create-job-evaluation.
 * @param ctx _context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
 * @param jobName The job identifier.
 * @return ApiPostJobEvaluateRequestRequest
 */
func (a *JobsApiService) PostJobEvaluateRequest(ctx _context.Context, jobName string) ApiPostJobEvaluateRequestRequest {
	return ApiPostJobEvaluateRequestRequest{
		ApiService: a,
		ctx: ctx,
		jobName: jobName,
	}
}

/*
 * Execute executes the request
 * @return JobRegisterResponse
 */
func (a *JobsApiService) PostJobEvaluateRequestExecute(r ApiPostJobEvaluateRequestRequest) (JobRegisterResponse, *_nethttp.Response, error) {
	var (
		localVarHTTPMethod   = _nethttp.MethodPost
		localVarPostBody     interface{}
		localVarFormFileName string
		localVarFileName     string
		localVarFileBytes    []byte
		localVarReturnValue  JobRegisterResponse
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "JobsApiService.PostJobEvaluateRequest")
	if err != nil {
		return localVarReturnValue, nil, GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/job/{jobName}/evaluate"
	localVarPath = strings.Replace(localVarPath, "{"+"jobName"+"}", _neturl.PathEscape(parameterToString(r.jobName, "")), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := _neturl.Values{}
	localVarFormParams := _neturl.Values{}
	if r.jobEvaluateRequest == nil {
		return localVarReturnValue, nil, reportError("jobEvaluateRequest is required and must be specified")
	}

	if r.region != nil {
		localVarQueryParams.Add("region", parameterToString(*r.region, ""))
	}
	if r.namespace != nil {
		localVarQueryParams.Add("namespace", parameterToString(*r.namespace, ""))
	}
	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{"application/json"}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{"application/json"}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	if r.xNomadToken != nil {
		localVarHeaderParams["X-Nomad-Token"] = parameterToString(*r.xNomadToken, "")
	}
	// body params
	localVarPostBody = r.jobEvaluateRequest
	req, err := a.client.prepareRequest(r.ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, localVarFormFileName, localVarFileName, localVarFileBytes)
	if err != nil {
		return localVarReturnValue, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(req)
	if err != nil || localVarHTTPResponse == nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	localVarBody, err := _ioutil.ReadAll(localVarHTTPResponse.Body)
	localVarHTTPResponse.Body.Close()
	localVarHTTPResponse.Body = _ioutil.NopCloser(bytes.NewBuffer(localVarBody))
	if err != nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	if localVarHTTPResponse.StatusCode >= 300 {
		newErr := GenericOpenAPIError{
			body:  localVarBody,
			error: localVarHTTPResponse.Status,
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	err = a.client.decode(&localVarReturnValue, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
	if err != nil {
		newErr := GenericOpenAPIError{
			body:  localVarBody,
			error: err.Error(),
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	return localVarReturnValue, localVarHTTPResponse, nil
}

type ApiPostJobPlanRequestRequest struct {
	ctx _context.Context
	ApiService *JobsApiService
	jobName string
	jobPlanRequest *JobPlanRequest
	region *string
	namespace *string
	xNomadToken *string
}

func (r ApiPostJobPlanRequestRequest) JobPlanRequest(jobPlanRequest JobPlanRequest) ApiPostJobPlanRequestRequest {
	r.jobPlanRequest = &jobPlanRequest
	return r
}
func (r ApiPostJobPlanRequestRequest) Region(region string) ApiPostJobPlanRequestRequest {
	r.region = &region
	return r
}
func (r ApiPostJobPlanRequestRequest) Namespace(namespace string) ApiPostJobPlanRequestRequest {
	r.namespace = &namespace
	return r
}
func (r ApiPostJobPlanRequestRequest) XNomadToken(xNomadToken string) ApiPostJobPlanRequestRequest {
	r.xNomadToken = &xNomadToken
	return r
}

func (r ApiPostJobPlanRequestRequest) Execute() (JobPlanResponse, *_nethttp.Response, error) {
	return r.ApiService.PostJobPlanRequestExecute(r)
}

/*
 * PostJobPlanRequest This endpoint invokes a dry-run of the scheduler for the job. See https://www.nomadproject.io/api-docs/jobs#create-job-plan
 * @param ctx _context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
 * @param jobName The job identifier.
 * @return ApiPostJobPlanRequestRequest
 */
func (a *JobsApiService) PostJobPlanRequest(ctx _context.Context, jobName string) ApiPostJobPlanRequestRequest {
	return ApiPostJobPlanRequestRequest{
		ApiService: a,
		ctx: ctx,
		jobName: jobName,
	}
}

/*
 * Execute executes the request
 * @return JobPlanResponse
 */
func (a *JobsApiService) PostJobPlanRequestExecute(r ApiPostJobPlanRequestRequest) (JobPlanResponse, *_nethttp.Response, error) {
	var (
		localVarHTTPMethod   = _nethttp.MethodPost
		localVarPostBody     interface{}
		localVarFormFileName string
		localVarFileName     string
		localVarFileBytes    []byte
		localVarReturnValue  JobPlanResponse
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "JobsApiService.PostJobPlanRequest")
	if err != nil {
		return localVarReturnValue, nil, GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/job/{jobName}/plan"
	localVarPath = strings.Replace(localVarPath, "{"+"jobName"+"}", _neturl.PathEscape(parameterToString(r.jobName, "")), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := _neturl.Values{}
	localVarFormParams := _neturl.Values{}
	if r.jobPlanRequest == nil {
		return localVarReturnValue, nil, reportError("jobPlanRequest is required and must be specified")
	}

	if r.region != nil {
		localVarQueryParams.Add("region", parameterToString(*r.region, ""))
	}
	if r.namespace != nil {
		localVarQueryParams.Add("namespace", parameterToString(*r.namespace, ""))
	}
	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{"application/json"}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{"application/json"}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	if r.xNomadToken != nil {
		localVarHeaderParams["X-Nomad-Token"] = parameterToString(*r.xNomadToken, "")
	}
	// body params
	localVarPostBody = r.jobPlanRequest
	req, err := a.client.prepareRequest(r.ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, localVarFormFileName, localVarFileName, localVarFileBytes)
	if err != nil {
		return localVarReturnValue, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(req)
	if err != nil || localVarHTTPResponse == nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	localVarBody, err := _ioutil.ReadAll(localVarHTTPResponse.Body)
	localVarHTTPResponse.Body.Close()
	localVarHTTPResponse.Body = _ioutil.NopCloser(bytes.NewBuffer(localVarBody))
	if err != nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	if localVarHTTPResponse.StatusCode >= 300 {
		newErr := GenericOpenAPIError{
			body:  localVarBody,
			error: localVarHTTPResponse.Status,
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	err = a.client.decode(&localVarReturnValue, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
	if err != nil {
		newErr := GenericOpenAPIError{
			body:  localVarBody,
			error: err.Error(),
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	return localVarReturnValue, localVarHTTPResponse, nil
}

type ApiPostJobsParseRequestRequest struct {
	ctx _context.Context
	ApiService *JobsApiService
	jobsParseRequest *JobsParseRequest
}

func (r ApiPostJobsParseRequestRequest) JobsParseRequest(jobsParseRequest JobsParseRequest) ApiPostJobsParseRequestRequest {
	r.jobsParseRequest = &jobsParseRequest
	return r
}

func (r ApiPostJobsParseRequestRequest) Execute() (Job, *_nethttp.Response, error) {
	return r.ApiService.PostJobsParseRequestExecute(r)
}

/*
 * PostJobsParseRequest Parses a HCL jobspec and produce the equivalent JSON encoded job. See https://www.nomadproject.io/api-docs/jobs#parse-job.
 * @param ctx _context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
 * @return ApiPostJobsParseRequestRequest
 */
func (a *JobsApiService) PostJobsParseRequest(ctx _context.Context) ApiPostJobsParseRequestRequest {
	return ApiPostJobsParseRequestRequest{
		ApiService: a,
		ctx: ctx,
	}
}

/*
 * Execute executes the request
 * @return Job
 */
func (a *JobsApiService) PostJobsParseRequestExecute(r ApiPostJobsParseRequestRequest) (Job, *_nethttp.Response, error) {
	var (
		localVarHTTPMethod   = _nethttp.MethodPost
		localVarPostBody     interface{}
		localVarFormFileName string
		localVarFileName     string
		localVarFileBytes    []byte
		localVarReturnValue  Job
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "JobsApiService.PostJobsParseRequest")
	if err != nil {
		return localVarReturnValue, nil, GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/jobs/parse"

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := _neturl.Values{}
	localVarFormParams := _neturl.Values{}
	if r.jobsParseRequest == nil {
		return localVarReturnValue, nil, reportError("jobsParseRequest is required and must be specified")
	}

	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{"application/json"}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{"application/json"}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	// body params
	localVarPostBody = r.jobsParseRequest
	req, err := a.client.prepareRequest(r.ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, localVarFormFileName, localVarFileName, localVarFileBytes)
	if err != nil {
		return localVarReturnValue, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(req)
	if err != nil || localVarHTTPResponse == nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	localVarBody, err := _ioutil.ReadAll(localVarHTTPResponse.Body)
	localVarHTTPResponse.Body.Close()
	localVarHTTPResponse.Body = _ioutil.NopCloser(bytes.NewBuffer(localVarBody))
	if err != nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	if localVarHTTPResponse.StatusCode >= 300 {
		newErr := GenericOpenAPIError{
			body:  localVarBody,
			error: localVarHTTPResponse.Status,
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	err = a.client.decode(&localVarReturnValue, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
	if err != nil {
		newErr := GenericOpenAPIError{
			body:  localVarBody,
			error: err.Error(),
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	return localVarReturnValue, localVarHTTPResponse, nil
}

type ApiPutJobForceRequestRequest struct {
	ctx _context.Context
	ApiService *JobsApiService
	jobName string
	region *string
	namespace *string
	xNomadToken *string
}

func (r ApiPutJobForceRequestRequest) Region(region string) ApiPutJobForceRequestRequest {
	r.region = &region
	return r
}
func (r ApiPutJobForceRequestRequest) Namespace(namespace string) ApiPutJobForceRequestRequest {
	r.namespace = &namespace
	return r
}
func (r ApiPutJobForceRequestRequest) XNomadToken(xNomadToken string) ApiPutJobForceRequestRequest {
	r.xNomadToken = &xNomadToken
	return r
}

func (r ApiPutJobForceRequestRequest) Execute() (PeriodicForceResponse, *_nethttp.Response, error) {
	return r.ApiService.PutJobForceRequestExecute(r)
}

/*
 * PutJobForceRequest Forces a new instance of the periodic job. A new instance will be created even if it violates the job's prohibit_overlap settings. As such, this should be only used to immediately run a periodic job. See [documentation](https://www.nomadproject.io/docs/commands/job/periodic-force).
 * @param ctx _context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
 * @param jobName The job identifier.
 * @return ApiPutJobForceRequestRequest
 */
func (a *JobsApiService) PutJobForceRequest(ctx _context.Context, jobName string) ApiPutJobForceRequestRequest {
	return ApiPutJobForceRequestRequest{
		ApiService: a,
		ctx: ctx,
		jobName: jobName,
	}
}

/*
 * Execute executes the request
 * @return PeriodicForceResponse
 */
func (a *JobsApiService) PutJobForceRequestExecute(r ApiPutJobForceRequestRequest) (PeriodicForceResponse, *_nethttp.Response, error) {
	var (
		localVarHTTPMethod   = _nethttp.MethodPost
		localVarPostBody     interface{}
		localVarFormFileName string
		localVarFileName     string
		localVarFileBytes    []byte
		localVarReturnValue  PeriodicForceResponse
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "JobsApiService.PutJobForceRequest")
	if err != nil {
		return localVarReturnValue, nil, GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/job/{jobName}/periodic/force"
	localVarPath = strings.Replace(localVarPath, "{"+"jobName"+"}", _neturl.PathEscape(parameterToString(r.jobName, "")), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := _neturl.Values{}
	localVarFormParams := _neturl.Values{}

	if r.region != nil {
		localVarQueryParams.Add("region", parameterToString(*r.region, ""))
	}
	if r.namespace != nil {
		localVarQueryParams.Add("namespace", parameterToString(*r.namespace, ""))
	}
	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{"application/json"}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	if r.xNomadToken != nil {
		localVarHeaderParams["X-Nomad-Token"] = parameterToString(*r.xNomadToken, "")
	}
	req, err := a.client.prepareRequest(r.ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, localVarFormFileName, localVarFileName, localVarFileBytes)
	if err != nil {
		return localVarReturnValue, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(req)
	if err != nil || localVarHTTPResponse == nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	localVarBody, err := _ioutil.ReadAll(localVarHTTPResponse.Body)
	localVarHTTPResponse.Body.Close()
	localVarHTTPResponse.Body = _ioutil.NopCloser(bytes.NewBuffer(localVarBody))
	if err != nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	if localVarHTTPResponse.StatusCode >= 300 {
		newErr := GenericOpenAPIError{
			body:  localVarBody,
			error: localVarHTTPResponse.Status,
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	err = a.client.decode(&localVarReturnValue, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
	if err != nil {
		newErr := GenericOpenAPIError{
			body:  localVarBody,
			error: err.Error(),
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	return localVarReturnValue, localVarHTTPResponse, nil
}
