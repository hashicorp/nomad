package structs

// Context defines the scope in which a search for Nomad object operates, and
// is also used to query the matching index value for this context.
type Context string

const (
	// Individual context types.
	Allocs          Context = "allocs"
	Deployments     Context = "deployment"
	Evals           Context = "evals"
	Jobs            Context = "jobs"
	Nodes           Context = "nodes"
	Namespaces      Context = "namespaces"
	Quotas          Context = "quotas"
	Recommendations Context = "recommendations"
	ScalingPolicies Context = "scaling_policy"
	Plugins         Context = "plugins"
	Volumes         Context = "volumes"

	// Subtypes used in fuzzy matching.
	Groups   Context = "groups"
	Services Context = "services"
	Tasks    Context = "tasks"
	Images   Context = "images"
	Commands Context = "commands"
	Classes  Context = "classes"

	// Union context types.
	All Context = "all"
)

// SearchConfig is used in servers to configure search API options.
type SearchConfig struct {
	// FuzzyEnabled toggles whether the FuzzySearch API is enabled. If not
	// enabled, requests to /v1/search/fuzzy will reply with a 404 response code.
	FuzzyEnabled bool `hcl:"fuzzy_enabled"`

	// LimitQuery limits the number of objects searched in the FuzzySearch API.
	// The results are indicated as truncated if the limit is reached.
	//
	// Lowering this value can reduce resource consumption of Nomad server when
	// the FuzzySearch API is enabled.
	LimitQuery int `hcl:"limit_query"`

	// LimitResults limits the number of results provided by the FuzzySearch API.
	// The results are indicated as truncate if the limit is reached.
	//
	// Lowering this value can reduce resource consumption of Nomad server per
	// fuzzy search request when the FuzzySearch API is enabled.
	LimitResults int `hcl:"limit_results"`

	// MinTermLength is the minimum length of Text required before the FuzzySearch
	// API will return results.
	//
	// Increasing this value can avoid resource consumption on Nomad server by
	// reducing searches with less meaningful results.
	MinTermLength int `hcl:"min_term_length"`
}

// SearchResponse is used to return matches and information about whether
// the match list is truncated specific to each type of Context.
type SearchResponse struct {
	// Map of Context types to ids which match a specified prefix
	Matches map[Context][]string

	// Truncations indicates whether the matches for a particular Context have
	// been truncated
	Truncations map[Context]bool

	QueryMeta
}

// SearchRequest is used to parameterize a request, and returns a
// list of matches made up of jobs, allocations, evaluations, and/or nodes,
// along with whether or not the information returned is truncated.
type SearchRequest struct {
	// Prefix is what ids are matched to. I.e, if the given prefix were
	// "a", potential matches might be "abcd" or "aabb"
	Prefix string

	// Context is the type that can be matched against. A context can be a job,
	// node, evaluation, allocation, or empty (indicated every context should be
	// matched)
	Context Context

	QueryOptions
}

// FuzzyMatch is used to describe the ID of an object which may be a machine
// readable UUID or a human readable Name. If the object is a component of a Job,
// the Scope is a list of IDs starting from Namespace down to the parent object of
// ID.
//
// e.g. A Task-level service would have scope like,
//   ["<namespace>", "<job>", "<group>", "<task>"]
type FuzzyMatch struct {
	ID    string   // ID is UUID or Name of object
	Scope []string `json:",omitempty"` // IDs of parent objects
}

// FuzzySearchResponse is used to return fuzzy matches and information about
// whether the match list is truncated specific to each type of searchable Context.
type FuzzySearchResponse struct {
	// Matches is a map of Context types to IDs which fuzzy match a specified query.
	Matches map[Context][]FuzzyMatch

	// Truncations indicates whether the matches for a particular Context have
	// been truncated.
	Truncations map[Context]bool

	QueryMeta
}

// FuzzySearchRequest is used to parameterize a fuzzy search request, and returns
// a list of matches made up of jobs, allocations, evaluations, and/or nodes,
// along with whether or not the information returned is truncated.
type FuzzySearchRequest struct {
	// Text is what names are fuzzy-matched to. E.g. if the given text were
	// "py", potential matches might be "python", "mypy", etc. of jobs, nodes,
	// allocs, groups, services, commands, images, classes.
	Text string

	// Context is the type that can be matched against. A Context of "all" indicates
	// all Contexts types are queried for matching.
	Context Context

	QueryOptions
}
