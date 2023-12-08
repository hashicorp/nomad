// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// truncateLimit is the maximum number of matches that will be returned for a
	// prefix for a specific context.
	//
	// Does not apply to fuzzy searching.
	truncateLimit = 20
)

var (
	// ossContexts are the oss contexts which are searched to find matches
	// for a given prefix
	ossContexts = []structs.Context{
		structs.Allocs,
		structs.Jobs,
		structs.Nodes,
		structs.NodePools,
		structs.Evals,
		structs.Deployments,
		structs.Plugins,
		structs.Volumes,
		structs.ScalingPolicies,
		structs.Variables,
		structs.Namespaces,
	}
)

// Search endpoint is used to look up matches for a given prefix and context
type Search struct {
	srv    *Server
	ctx    *RPCContext
	logger hclog.Logger
}

func NewSearchEndpoint(srv *Server, ctx *RPCContext) *Search {
	return &Search{srv: srv, ctx: ctx, logger: srv.logger.Named("search")}
}

// getPrefixMatches extracts matches for an iterator, and returns a list of ids for
// these matches.
func (s *Search) getPrefixMatches(iter memdb.ResultIterator, prefix string) ([]string, bool) {
	var matches []string

	for i := 0; i < truncateLimit; i++ {
		raw := iter.Next()
		if raw == nil {
			break
		}

		var id string
		switch t := raw.(type) {
		case *structs.Job:
			id = t.ID
		case *structs.Evaluation:
			id = t.ID
		case *structs.Allocation:
			id = t.ID
		case *structs.Node:
			id = t.ID
		case *structs.NodePool:
			id = t.Name
		case *structs.Deployment:
			id = t.ID
		case *structs.CSIPlugin:
			id = t.ID
		case *structs.CSIVolume:
			id = t.ID
		case *structs.ScalingPolicy:
			id = t.ID
		case *structs.Namespace:
			id = t.Name
		case *structs.VariableEncrypted:
			id = t.Path
		default:
			matchID, ok := getEnterpriseMatch(raw)
			if !ok {
				s.logger.Error("unexpected type for resources context", "type", fmt.Sprintf("%T", t))
				continue
			}

			id = matchID
		}

		if !strings.HasPrefix(id, prefix) {
			continue
		}

		matches = append(matches, id)
	}

	return matches, iter.Next() != nil
}

func (s *Search) getFuzzyMatches(iter memdb.ResultIterator, text string) (map[structs.Context][]structs.FuzzyMatch, map[structs.Context]bool) {
	limitQuery := s.srv.config.SearchConfig.LimitQuery
	limitResults := s.srv.config.SearchConfig.LimitResults

	unsorted := make(map[structs.Context][]fuzzyMatch)
	truncations := make(map[structs.Context]bool)

	accumulateSet := func(limited bool, set map[structs.Context][]fuzzyMatch) {
		for ctx, matches := range set {
			for _, match := range matches {
				if len(unsorted[ctx]) < limitResults {
					unsorted[ctx] = append(unsorted[ctx], match)
				} else {
					// truncated by results limit
					truncations[ctx] = true
					return
				}
				if limited {
					// truncated by query limit
					truncations[ctx] = true
					return
				}
			}
		}
	}

	accumulateSingle := func(limited bool, ctx structs.Context, match *fuzzyMatch) {
		if match != nil {
			if len(unsorted[ctx]) < limitResults {
				unsorted[ctx] = append(unsorted[ctx], *match)
			} else {
				// truncated by results limit
				truncations[ctx] = true
				return
			}
			if limited {
				// truncated by query limit
				truncations[ctx] = true
				return
			}
		}
	}

	limited := func(i int, iter memdb.ResultIterator) bool {
		if i == limitQuery-1 {
			return iter.Next() != nil
		}
		return false
	}

	for i := 0; i < limitQuery; i++ {
		raw := iter.Next()
		if raw == nil {
			break
		}

		switch t := raw.(type) {
		case *structs.Job:
			set := s.fuzzyMatchesJob(t, text)
			accumulateSet(limited(i, iter), set)
		default:
			ctx, match := s.fuzzyMatchSingle(raw, text)
			accumulateSingle(limited(i, iter), ctx, match)
		}
	}

	// sort the set of match results
	for ctx := range unsorted {
		sortSet(unsorted[ctx])
	}

	// create the result out of exported types
	m := make(map[structs.Context][]structs.FuzzyMatch, len(unsorted))
	for ctx, matches := range unsorted {
		m[ctx] = make([]structs.FuzzyMatch, 0, len(matches))
		for _, match := range matches {
			m[ctx] = append(m[ctx], structs.FuzzyMatch{
				ID:    match.id,
				Scope: match.scope,
			})
		}
	}

	return m, truncations
}

// fuzzyIndex returns the index of text in name, ignoring case.
//
//	text is assumed to be lower case.
//	-1 is returned if name does not contain text.
func fuzzyIndex(name, text string) int {
	lower := strings.ToLower(name)
	return strings.Index(lower, text)
}

// fuzzySingleMatch determines if the ID of raw is a fuzzy match with text.
// Returns the context and score or nil if there is no match.
func (s *Search) fuzzyMatchSingle(raw interface{}, text string) (structs.Context, *fuzzyMatch) {
	var (
		name  string // fuzzy searchable name
		scope []string
		ctx   structs.Context
	)

	switch t := raw.(type) {
	case *structs.Node:
		name = t.Name
		scope = []string{t.ID}
		ctx = structs.Nodes
	case *structs.NodePool:
		name = t.Name
		scope = []string{t.Name}
		ctx = structs.NodePools
	case *structs.Namespace:
		name = t.Name
		ctx = structs.Namespaces
	case *structs.Allocation:
		name = t.Name
		scope = []string{t.Namespace, t.ID}
		ctx = structs.Allocs
	case *structs.CSIPlugin:
		name = t.ID
		ctx = structs.Plugins
	case *structs.VariableEncrypted:
		name = t.Path
		scope = []string{t.Namespace, t.Path}
		ctx = structs.Variables
	}

	if idx := fuzzyIndex(name, text); idx >= 0 {
		return ctx, &fuzzyMatch{
			id:    name,
			score: idx,
			scope: scope,
		}
	}

	return "", nil
}

// getFuzzyMatchesJob digs through j and extracts matches against several types
// of matchable Context. Results are categorized by Context and paired with their
// score, but are unsorted.
//
//	job.name
//	job|group.name
//	job|group|service.name
//	job|group|task.name
//	job|group|task|service.name
//	job|group|task|driver.{image,command,class}
func (*Search) fuzzyMatchesJob(j *structs.Job, text string) map[structs.Context][]fuzzyMatch {
	sm := make(map[structs.Context][]fuzzyMatch)
	ns := j.Namespace
	job := j.ID

	// job.name
	if idx := fuzzyIndex(j.Name, text); idx >= 0 {
		sm[structs.Jobs] = append(sm[structs.Jobs], score(j.Name, ns, idx, job))
	}

	// job|group.name
	for _, group := range j.TaskGroups {
		if idx := fuzzyIndex(group.Name, text); idx >= 0 {
			sm[structs.Groups] = append(sm[structs.Groups], score(group.Name, ns, idx, job))
		}

		// job|group|service.name
		for _, service := range group.Services {
			if idx := fuzzyIndex(service.Name, text); idx >= 0 {
				sm[structs.Services] = append(sm[structs.Services], score(service.Name, ns, idx, job, group.Name))
			}
		}

		// job|group|task.name
		for _, task := range group.Tasks {
			if idx := fuzzyIndex(task.Name, text); idx >= 0 {
				sm[structs.Tasks] = append(sm[structs.Tasks], score(task.Name, ns, idx, job, group.Name))
			}

			// job|group|task|service.name
			for _, service := range task.Services {
				if idx := fuzzyIndex(service.Name, text); idx >= 0 {
					sm[structs.Services] = append(sm[structs.Services], score(service.Name, ns, idx, job, group.Name, task.Name))
				}
			}

			// job|group|task|config.{image,command,class}
			switch task.Driver {
			case "docker":
				image := getConfigParam(task.Config, "image")
				if idx := fuzzyIndex(image, text); idx >= 0 {
					sm[structs.Images] = append(sm[structs.Images], score(image, ns, idx, job, group.Name, task.Name))
				}
			case "exec", "raw_exec":
				command := getConfigParam(task.Config, "command")
				if idx := fuzzyIndex(command, text); idx >= 0 {
					sm[structs.Commands] = append(sm[structs.Commands], score(command, ns, idx, job, group.Name, task.Name))
				}
			case "java":
				class := getConfigParam(task.Config, "class")
				if idx := fuzzyIndex(class, text); idx >= 0 {
					sm[structs.Classes] = append(sm[structs.Classes], score(class, ns, idx, job, group.Name, task.Name))
				}
			}
		}
	}

	return sm
}

func getConfigParam(config map[string]interface{}, param string) string {
	if config == nil || config[param] == nil {
		return ""
	}

	s, ok := config[param].(string)
	if !ok {
		return ""
	}

	return s
}

type fuzzyMatch struct {
	id    string
	scope []string
	score int
}

func score(id, namespace string, score int, scope ...string) fuzzyMatch {
	return fuzzyMatch{
		id:    id,
		score: score,
		scope: append([]string{namespace}, scope...),
	}
}

func sortSet(matches []fuzzyMatch) {
	sort.Slice(matches, func(a, b int) bool {
		A, B := matches[a], matches[b]

		// sort by index
		switch {
		case A.score < B.score:
			return true
		case B.score < A.score:
			return false
		}

		// shorter length matched text is more likely to be the thing being
		// searched for (in theory)
		//
		// this also causes exact matches to score best, which is desirable
		idA, idB := A.id, B.id
		switch {
		case len(idA) < len(idB):
			return true
		case len(idB) < len(idA):
			return false
		}

		// same index and same length, break ties alphabetically
		return idA < idB
	})
}

// getResourceIter takes a context and returns a memdb iterator specific to
// that context
func getResourceIter(context structs.Context, aclObj *acl.ACL, namespace, prefix string, ws memdb.WatchSet, store *state.StateStore) (memdb.ResultIterator, error) {
	switch context {
	case structs.Jobs:
		return store.JobsByIDPrefix(ws, namespace, prefix)
	case structs.Evals:
		return store.EvalsByIDPrefix(ws, namespace, prefix, state.SortDefault)
	case structs.Allocs:
		return store.AllocsByIDPrefix(ws, namespace, prefix, state.SortDefault)
	case structs.Nodes:
		return store.NodesByIDPrefix(ws, prefix)
	case structs.NodePools:
		iter, err := store.NodePoolsByNamePrefix(ws, prefix, state.SortDefault)
		if err != nil {
			return nil, err
		}
		if aclObj == nil || aclObj.IsManagement() {
			return iter, nil
		}
		return memdb.NewFilterIterator(iter, nodePoolCapFilter(aclObj)), nil
	case structs.Deployments:
		return store.DeploymentsByIDPrefix(ws, namespace, prefix, state.SortDefault)
	case structs.Plugins:
		return store.CSIPluginsByIDPrefix(ws, prefix)
	case structs.ScalingPolicies:
		return store.ScalingPoliciesByIDPrefix(ws, namespace, prefix)
	case structs.Volumes:
		return store.CSIVolumesByIDPrefix(ws, namespace, prefix)
	case structs.Namespaces:
		iter, err := store.NamespacesByNamePrefix(ws, prefix)
		if err != nil {
			return nil, err
		}
		if aclObj == nil {
			return iter, nil
		}
		return memdb.NewFilterIterator(iter, nsCapFilter(aclObj)), nil
	case structs.Variables:
		iter, err := store.GetVariablesByPrefix(ws, prefix)
		if err != nil {
			return nil, err
		}
		if aclObj == nil {
			return iter, nil
		}
		return memdb.NewFilterIterator(iter, nsCapFilter(aclObj)), nil
	default:
		return getEnterpriseResourceIter(context, aclObj, namespace, prefix, ws, store)
	}
}

// wildcard is a helper for determining if namespace is '*', used to determine
// if objects from every namespace should be considered when iterating, and that
// additional ACL checks will be necessary.
func wildcard(namespace string) bool {
	return namespace == structs.AllNamespacesSentinel
}

func getFuzzyResourceIterator(context structs.Context, aclObj *acl.ACL, namespace string, ws memdb.WatchSet, store *state.StateStore) (memdb.ResultIterator, error) {
	switch context {
	case structs.Jobs:
		if wildcard(namespace) {
			iter, err := store.Jobs(ws)
			return nsCapIterFilter(iter, err, aclObj)
		}
		return store.JobsByNamespace(ws, namespace)

	case structs.Allocs:
		if wildcard(namespace) {
			iter, err := store.Allocs(ws, state.SortDefault)
			return nsCapIterFilter(iter, err, aclObj)
		}
		return store.AllocsByNamespace(ws, namespace)

	case structs.Variables:
		if wildcard(namespace) {
			iter, err := store.Variables(ws)
			return nsCapIterFilter(iter, err, aclObj)
		}
		return store.GetVariablesByNamespace(ws, namespace)

	case structs.Nodes:
		if wildcard(namespace) {
			iter, err := store.Nodes(ws)
			return nsCapIterFilter(iter, err, aclObj)
		}
		return store.Nodes(ws)

	case structs.NodePools:
		iter, err := store.NodePools(ws, state.SortDefault)
		if err != nil {
			return nil, err
		}

		if aclObj == nil || aclObj.IsManagement() {
			return iter, nil
		}
		return memdb.NewFilterIterator(iter, nodePoolCapFilter(aclObj)), nil

	case structs.Plugins:
		if wildcard(namespace) {
			iter, err := store.CSIPlugins(ws)
			return nsCapIterFilter(iter, err, aclObj)
		}
		return store.CSIPlugins(ws)

	case structs.Namespaces:
		iter, err := store.Namespaces(ws)
		return nsCapIterFilter(iter, err, aclObj)

	default:
		return getEnterpriseFuzzyResourceIter(context, aclObj, namespace, ws, store)
	}
}

// nsCapIterFilter wraps an iterator with a filter for removing items that the token
// does not have permission to read (whether missing the capability or in the
// wrong namespace).
func nsCapIterFilter(iter memdb.ResultIterator, err error, aclObj *acl.ACL) (memdb.ResultIterator, error) {
	if err != nil {
		return nil, err
	}
	if aclObj == nil {
		return iter, nil
	}
	return memdb.NewFilterIterator(iter, nsCapFilter(aclObj)), nil
}

// nsCapFilter produces a memdb.FilterFunc for removing objects not accessible
// by aclObj during a table scan.
func nsCapFilter(aclObj *acl.ACL) memdb.FilterFunc {
	return func(v interface{}) bool {
		switch t := v.(type) {
		case *structs.Job:
			return !aclObj.AllowNsOp(t.Namespace, acl.NamespaceCapabilityReadJob)

		case *structs.Allocation:
			return !aclObj.AllowNsOp(t.Namespace, acl.NamespaceCapabilityReadJob)

		case *structs.VariableEncrypted:
			return !aclObj.AllowVariableSearch(t.Namespace)

		case *structs.Namespace:
			return !aclObj.AllowNamespace(t.Name)

		case *structs.Node:
			return !aclObj.AllowNodeRead()

		case *structs.CSIPlugin:
			return !aclObj.AllowPluginRead()

		default:
			return false
		}
	}
}

// nodePoolCapFilter produces a memdb.FilterFunc for removing node pools not
// accessible by aclObj during a table scan.
func nodePoolCapFilter(aclObj *acl.ACL) memdb.FilterFunc {
	return func(v interface{}) bool {
		pool := v.(*structs.NodePool)
		return !aclObj.AllowNodePoolOperation(pool.Name, acl.NodePoolCapabilityRead)
	}
}

// If the length of a prefix is odd, return a subset to the last even character
// This only applies to UUIDs, jobs are excluded
func roundUUIDDownIfOdd(prefix string, context structs.Context) string {
	if context == structs.Jobs {
		return prefix
	}

	// We ignore the count of hyphens when calculating if the prefix is even:
	// E.g "e3671fa4-21"
	numHyphens := strings.Count(prefix, "-")
	l := len(prefix) - numHyphens
	if l%2 == 0 {
		return prefix
	}
	return prefix[:len(prefix)-1]
}

// silenceError determines whether err is an error we care about when getting an
// iterator from the state store - we ignore errors about invalid UUIDs, since
// we sometimes try to lookup by Name and not UUID.
func (*Search) silenceError(err error) bool {
	if err == nil {
		return true
	}

	e := err.Error()
	switch {
	// Searching other contexts with job names raises an error, which in
	// this case we want to ignore.
	case strings.Contains(e, "Invalid UUID: encoding/hex"):
	case strings.Contains(e, "UUID have 36 characters"):
	case strings.Contains(e, "must be even length"):
	case strings.Contains(e, "UUID should have maximum of 4"):
	default:
		// err was not nil and not about UUID prefix, something bad happened
		return false
	}

	return true
}

// PrefixSearch is used to list matches for a given prefix, and returns
// matching jobs, evaluations, allocations, and/or nodes.
func (s *Search) PrefixSearch(args *structs.SearchRequest, reply *structs.SearchResponse) error {

	authErr := s.srv.Authenticate(s.ctx, args)
	if done, err := s.srv.forward("Search.PrefixSearch", args, args, reply); done {
		return err
	}
	s.srv.MeasureRPCRate("search", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "search", "prefix_search"}, time.Now())

	aclObj, err := s.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	namespace := args.RequestNamespace()

	// Require read permissions for the context, ex. node:read or
	// namespace:read-job
	if !sufficientSearchPerms(aclObj, namespace, args.Context) {
		return structs.ErrPermissionDenied
	}

	reply.Matches = make(map[structs.Context][]string)
	reply.Truncations = make(map[structs.Context]bool)

	// Setup the blocking query
	opts := blockingOptions{
		queryMeta: &reply.QueryMeta,
		queryOpts: &structs.QueryOptions{},
		run: func(ws memdb.WatchSet, state *state.StateStore) error {

			iters := make(map[structs.Context]memdb.ResultIterator)
			contexts := filteredSearchContexts(aclObj, namespace, args.Context)

			for _, ctx := range contexts {
				iter, err := getResourceIter(ctx, aclObj, namespace, roundUUIDDownIfOdd(args.Prefix, args.Context), ws, state)
				if err != nil {
					if !s.silenceError(err) {
						return err
					}
				} else {
					iters[ctx] = iter
				}
			}

			// Return matches for the given prefix
			for k, v := range iters {
				res, isTrunc := s.getPrefixMatches(v, args.Prefix)
				reply.Matches[k] = res
				reply.Truncations[k] = isTrunc
			}

			// Set the index for the context. If the context has been specified, it
			// will be used as the index of the response. Otherwise, the
			// maximum index from all resources will be used.
			for _, ctx := range contexts {
				index, err := state.Index(contextToIndex(ctx))
				if err != nil {
					return err
				}
				if index > reply.Index {
					reply.Index = index
				}
			}

			s.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		}}
	return s.srv.blockingRPC(&opts)
}

// sufficientSearchPerms returns true if the provided ACL has access to *any*
// capabilities required for prefix searching. This is intended as a performance
// improvement so that we don't do expensive queries and then filter the results
// if the user will never get any results. The caller still needs to filter
// anything it gets from the state store.
//
// Returns true if aclObj is nil or is for a management token
func sufficientSearchPerms(aclObj *acl.ACL, namespace string, context structs.Context) bool {
	if aclObj == nil || aclObj.IsManagement() {
		return true
	}

	// Reject requests that explicitly specify a disallowed context. This
	// should give the user better feedback than simply filtering out all
	// results and returning an empty list.
	switch context {
	case structs.Nodes:
		return aclObj.AllowNodeRead()
	case structs.NodePools:
		// The search term alone is not enough to determine if the token is
		// allowed to access the given prefix since it may not match node pool
		// label in the policy. Node pools will be filtered when iterating over
		// the results.
		return aclObj.AllowNodePoolSearch()
	case structs.Namespaces:
		return aclObj.AllowNamespace(namespace)
	case structs.Allocs, structs.Deployments, structs.Evals, structs.Jobs,
		structs.ScalingPolicies, structs.Recommendations:
		return aclObj.AllowNsOp(namespace, acl.NamespaceCapabilityReadJob)
	case structs.Volumes:
		return acl.NamespaceValidator(acl.NamespaceCapabilityCSIListVolume,
			acl.NamespaceCapabilityCSIReadVolume,
			acl.NamespaceCapabilityListJobs,
			acl.NamespaceCapabilityReadJob)(aclObj, namespace)
	case structs.Variables:
		return aclObj.AllowVariableSearch(namespace)
	case structs.Plugins:
		return aclObj.AllowPluginList()
	case structs.Quotas:
		return aclObj.AllowQuotaRead()
	}

	return true
}

// FuzzySearch is used to list fuzzy or prefix matches for a given text argument and Context.
// If the Context is "all", all searchable contexts are searched. If ACLs are enabled,
// results are limited to policies of the provided ACL token.
//
// These types are limited to prefix UUID searching:
//
//	Evals, Deployments, ScalingPolicies, Volumes
//
// These types are available for fuzzy searching:
//
//	Nodes, Node Pools, Namespaces, Jobs, Allocs, Plugins, Variables
//
// Jobs are a special case that expand into multiple types, and whose return
// values include Scope which is a descending list of IDs of parent objects,
// starting with the Namespace. The subtypes of jobs are fuzzy searchable.
//
// The Jobs type expands into these sub types:
//
//	Jobs, Groups, Services, Tasks, Images, Commands, Classes
//
// The results are in descending order starting with strongest match, per Context type.
func (s *Search) FuzzySearch(args *structs.FuzzySearchRequest, reply *structs.FuzzySearchResponse) error {

	authErr := s.srv.Authenticate(s.ctx, args)
	if done, err := s.srv.forward("Search.FuzzySearch", args, args, reply); done {
		return err
	}
	s.srv.MeasureRPCRate("search", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "search", "fuzzy_search"}, time.Now())

	aclObj, err := s.srv.ResolveACL(args)
	if err != nil {
		return err
	}

	namespace := args.RequestNamespace()
	context := args.Context

	if !sufficientFuzzySearchPerms(aclObj, namespace, context) {
		return structs.ErrPermissionDenied
	}

	// check that fuzzy search API is enabled
	if !s.srv.config.SearchConfig.FuzzyEnabled {
		return fmt.Errorf("fuzzy search is not enabled")
	}

	// check the query term meets minimum length
	min := s.srv.config.SearchConfig.MinTermLength
	if n := len(args.Text); n < min {
		return fmt.Errorf("fuzzy search query must be at least %d characters, got %d", min, n)
	}

	// for case-insensitive searching, lower-case the search term once and reuse
	text := strings.ToLower(args.Text)

	// accumulate fuzzy search results and any truncations
	reply.Matches = make(map[structs.Context][]structs.FuzzyMatch)
	reply.Truncations = make(map[structs.Context]bool)

	// Setup the blocking query
	opts := blockingOptions{
		queryMeta: &reply.QueryMeta,
		queryOpts: new(structs.QueryOptions),
		run: func(ws memdb.WatchSet, state *state.StateStore) error {

			fuzzyIters := make(map[structs.Context]memdb.ResultIterator)
			prefixIters := make(map[structs.Context]memdb.ResultIterator)

			prefixContexts := filteredSearchContexts(aclObj, namespace, context)
			fuzzyContexts := filteredFuzzySearchContexts(aclObj, namespace, context)

			// Gather the iterators used for prefix searching from those allowable contexts
			for _, ctx := range prefixContexts {
				switch ctx {
				// only apply on the types that use UUID prefix searching
				case structs.Evals, structs.Deployments, structs.ScalingPolicies, structs.Volumes, structs.Quotas, structs.Recommendations:
					iter, err := getResourceIter(ctx, aclObj, namespace, roundUUIDDownIfOdd(args.Prefix, args.Context), ws, state)
					if err != nil {
						if !s.silenceError(err) {
							return err
						}
					} else {
						prefixIters[ctx] = iter
					}
				}
			}

			// Gather the iterators used for fuzzy searching from those allowable contexts
			for _, ctx := range fuzzyContexts {
				switch ctx {
				// skip the types that use UUID prefix searching
				case structs.Evals, structs.Deployments, structs.ScalingPolicies, structs.Volumes, structs.Quotas, structs.Recommendations:
					continue
				default:
					iter, err := getFuzzyResourceIterator(ctx, aclObj, namespace, ws, state)
					if err != nil {
						return err
					}
					fuzzyIters[ctx] = iter
				}
			}

			// Set prefix matches of the given text
			for ctx, iter := range prefixIters {
				res, isTrunc := s.getPrefixMatches(iter, args.Text)
				matches := make([]structs.FuzzyMatch, 0, len(res))
				for _, result := range res {
					matches = append(matches, structs.FuzzyMatch{ID: result})
				}
				reply.Matches[ctx] = matches
				reply.Truncations[ctx] = isTrunc
			}

			// Set fuzzy matches of the given text
			for iterCtx, iter := range fuzzyIters {

				// prefill truncations of iterable types so keys will exist in
				// the response for negative results
				reply.Truncations[iterCtx] = false

				matches, truncations := s.getFuzzyMatches(iter, text)
				for ctx := range matches {
					reply.Matches[ctx] = matches[ctx]
				}

				for ctx := range truncations {
					// only contains positive results
					reply.Truncations[ctx] = truncations[ctx]
				}
			}

			// Set the index for the context. If the context has been specified,
			// it will be used as the index of the response. Otherwise, the maximum
			// index from all the resources will be used.
			for _, ctx := range fuzzyContexts {
				index, err := state.Index(contextToIndex(ctx))
				if err != nil {
					return err
				}
				if index > reply.Index {
					reply.Index = index
				}
			}

			s.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		},
	}

	return s.srv.blockingRPC(&opts)
}

// expandContext returns either allContexts if context is 'all', or a one
// element slice with context by itself.
func expandContext(context structs.Context) []structs.Context {
	switch context {
	case structs.All:
		c := make([]structs.Context, len(allContexts))
		copy(c, allContexts)
		return c
	default:
		return []structs.Context{context}
	}
}

// sufficientFuzzySearchPerms returns true if the searched namespace is the wildcard
// namespace, indicating we should bypass the preflight ACL checks otherwise performed
// by sufficientSearchPerms. This is to support fuzzy searching multiple namespaces
// with tokens that have permission for more than one namespace. The actual ACL
// validation will be performed while scanning objects instead, where we have finally
// have a concrete namespace to work with.
func sufficientFuzzySearchPerms(aclObj *acl.ACL, namespace string, context structs.Context) bool {
	if wildcard(namespace) {
		return true
	}
	return sufficientSearchPerms(aclObj, namespace, context)
}

// filteredSearchContexts returns the expanded set of contexts, filtered down
// to the subset of contexts the aclObj is valid for.
//
// If aclObj is nil, no contexts are filtered out.
func filteredSearchContexts(aclObj *acl.ACL, namespace string, context structs.Context) []structs.Context {
	desired := expandContext(context)

	// If ACLs aren't enabled return all contexts
	if aclObj == nil {
		return desired
	}
	if aclObj.IsManagement() {
		return desired
	}
	jobRead := aclObj.AllowNsOp(namespace, acl.NamespaceCapabilityReadJob)
	allowVolume := acl.NamespaceValidator(acl.NamespaceCapabilityCSIListVolume,
		acl.NamespaceCapabilityCSIReadVolume,
		acl.NamespaceCapabilityListJobs,
		acl.NamespaceCapabilityReadJob)
	volRead := allowVolume(aclObj, namespace)
	policyRead := aclObj.AllowNsOp(namespace, acl.NamespaceCapabilityListScalingPolicies)

	// Filter contexts down to those the ACL grants access to
	available := make([]structs.Context, 0, len(desired))
	for _, c := range desired {
		switch c {
		case structs.Allocs, structs.Jobs, structs.Evals, structs.Deployments:
			if jobRead {
				available = append(available, c)
			}
		case structs.ScalingPolicies:
			if policyRead || jobRead {
				available = append(available, c)
			}
		case structs.Namespaces:
			if aclObj.AllowNamespace(namespace) {
				available = append(available, c)
			}
		case structs.Variables:
			if aclObj.AllowVariableSearch(namespace) {
				available = append(available, c)
			}
		case structs.Nodes:
			if aclObj.AllowNodeRead() {
				available = append(available, c)
			}
		case structs.NodePools:
			if aclObj.AllowNodePoolSearch() {
				available = append(available, c)
			}
		case structs.Volumes:
			if volRead {
				available = append(available, c)
			}
		case structs.Plugins:
			if aclObj.AllowPluginList() {
				available = append(available, c)
			}
		default:
			if ok := filteredSearchContextsEnt(aclObj, namespace, c); ok {
				available = append(available, c)
			}
		}
	}
	return available
}

// filterFuzzySearchContexts returns every context asked for if the searched namespace
// is the wildcard namespace, indicating we should bypass ACL checks otherwise
// performed by filterSearchContexts. Instead we will rely on iterator filters to
// perform the ACL validation while scanning objects, where we have a concrete
// namespace to work with.
func filteredFuzzySearchContexts(aclObj *acl.ACL, namespace string, context structs.Context) []structs.Context {
	if wildcard(namespace) {
		return expandContext(context)
	}
	return filteredSearchContexts(aclObj, namespace, context)
}
