// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	metrics "github.com/hashicorp/go-metrics/compat"

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
		structs.HostVolumes,
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

func getFuzzyMatchesImpl[T comparable](
	iter state.ResultIterator[T], text string, limitQuery, limitResults int) (
	map[structs.Context][]structs.FuzzyMatch, map[structs.Context]bool) {

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

	i := 0
	limited := false
	for val := range iter.All() {
		i++
		if i > limitQuery {
			break
		}
		if i == limitQuery {
			limited = iter.Next() != *new(T)
		}

		switch any(val).(type) {
		case *structs.Job:
			// oof, this is gross
			set := fuzzyMatchesJob(any(val).(*structs.Job), text)
			accumulateSet(limited, set)
		default:
			ctx, match := fuzzyMatchSingle(val, text)
			accumulateSingle(limited, ctx, match)
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
func fuzzyMatchSingle(raw any, text string) (structs.Context, *fuzzyMatch) {
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
func fuzzyMatchesJob(j *structs.Job, text string) map[structs.Context][]fuzzyMatch {
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

// TODO(tgross): could almost all this logic live in the state store code?
func getPrefixMatches(contexts []structs.Context, aclObj *acl.ACL, namespace, prefix string, ws memdb.WatchSet, store *state.StateStore) (map[structs.Context][]string, map[structs.Context]bool) {
	ids := make(map[structs.Context][]string, len(contexts))
	truncs := make(map[structs.Context]bool, len(contexts))

	// note: we continue on errors from iterators because we expect to get
	// invalid prefixes and just return empty sets for that

	for _, context := range contexts {
		switch context {
		case structs.Jobs:
			iter, err := store.JobsByIDPrefix(ws, namespace, prefix, state.SortDefault)
			if err != nil {
				continue
			}
			iter = nsCapIterFilter(iter, aclObj)
			ids[context], truncs[context] = prefixMatches(iter,
				func(job *structs.Job) string { return job.ID })

		case structs.Evals:
			cprefix := roundUUIDDownIfOdd(prefix, context)
			iter, err := store.EvalsByIDPrefix(ws, namespace, cprefix, state.SortDefault)
			if err != nil {
				continue
			}
			iter = nsCapIterFilter(iter, aclObj)
			ids[context], truncs[context] = prefixMatches(iter,
				func(eval *structs.Evaluation) string { return eval.ID })

		case structs.Allocs:
			cprefix := roundUUIDDownIfOdd(prefix, context)
			iter, err := store.AllocsByIDPrefix(ws, namespace, cprefix, state.SortDefault)
			if err != nil {
				continue
			}
			iter = nsCapIterFilter(iter, aclObj)
			ids[context], truncs[context] = prefixMatches(iter,
				func(alloc *structs.Allocation) string { return alloc.ID })

		case structs.Nodes:
			cprefix := roundUUIDDownIfOdd(prefix, context)
			iter, err := store.NodesByIDPrefix(ws, cprefix)
			if err != nil {
				continue
			}
			ids[context], truncs[context] = prefixMatches(iter,
				func(node *structs.Node) string { return node.ID })

		case structs.NodePools:
			iter, err := store.NodePoolsByNamePrefix(ws, prefix, state.SortDefault)
			if err != nil {
				continue
			}
			if !aclObj.IsManagement() {
				iter = state.NewFilterIterator(iter, nodePoolCapFilter(aclObj))
			}
			ids[context], truncs[context] = prefixMatches(iter,
				func(pool *structs.NodePool) string { return pool.Name })

		case structs.Deployments:
			cprefix := roundUUIDDownIfOdd(prefix, context)
			iter, err := store.DeploymentsByIDPrefix(ws, namespace, cprefix, state.SortDefault)
			if err != nil {
				continue
			}
			iter = nsCapIterFilter(iter, aclObj)
			ids[context], truncs[context] = prefixMatches(iter,
				func(d *structs.Deployment) string { return d.ID })

		case structs.Plugins:
			iter, err := store.CSIPluginsByIDPrefix(ws, prefix)
			if err != nil {
				continue
			}
			ids[context], truncs[context] = prefixMatches(iter,
				func(p *structs.CSIPlugin) string { return p.ID })

		case structs.ScalingPolicies:
			cprefix := roundUUIDDownIfOdd(prefix, context)
			iter, err := store.ScalingPoliciesByIDPrefix(ws, namespace, cprefix)
			if err != nil {
				continue
			}
			iter = nsCapIterFilter(iter, aclObj)
			ids[context], truncs[context] = prefixMatches(iter,
				func(p *structs.ScalingPolicy) string { return p.ID })

		case structs.Volumes:
			iter, err := store.CSIVolumesByIDPrefix(ws, namespace, prefix)
			if err != nil {
				continue
			}
			iter = nsCapIterFilter(iter, aclObj)
			ids[context], truncs[context] = prefixMatches(iter,
				func(v *structs.CSIVolume) string { return v.ID })

		case structs.HostVolumes:
			cprefix := roundUUIDDownIfOdd(prefix, context)
			iter, err := store.HostVolumesByIDPrefix(ws, namespace, cprefix, state.SortDefault)
			if err != nil {
				continue
			}
			iter = nsCapIterFilter(iter, aclObj)
			ids[context], truncs[context] = prefixMatches(iter,
				func(v *structs.HostVolume) string { return v.ID })

		case structs.Namespaces:
			iter, err := store.NamespacesByNamePrefix(ws, prefix)
			if err != nil {
				continue
			}
			iter = nsCapIterFilter(iter, aclObj)
			ids[context], truncs[context] = prefixMatches(iter,
				func(ns *structs.Namespace) string {
					return ns.Name
				})

		case structs.Variables:
			iter, err := store.GetVariablesByPrefix(ws, prefix)
			if err != nil {
				continue
			}
			iter = nsCapIterFilter(iter, aclObj)
			ids[context], truncs[context] = prefixMatches(iter,
				func(v *structs.VariableEncrypted) string { return v.Path })

		default:
			// TODO(tgross): will need to figure this out
			// iter := getEnterpriseResourceIter[any](context, aclObj, namespace, prefix, ws, store)
			// ids[context], truncs[context] = prefixMatchesOld(iter,
			// 	func(raw any) string {
			// 		return "now what???" // TODO(tgross)
			// 	})
		}

	}

	return ids, truncs
}

func prefixMatches[T comparable](iter state.ResultIterator[T], getID func(T) string) ([]string, bool) {
	ids := []string{}
	isTrunc := false
	for obj := range iter.All() {
		if len(ids) >= truncateLimit {
			isTrunc = true
			break
		}
		id := getID(obj)
		ids = append(ids, id)
	}
	return ids, isTrunc
}

// wildcard is a helper for determining if namespace is '*', used to determine
// if objects from every namespace should be considered when iterating, and that
// additional ACL checks will be necessary.
func wildcard(namespace string) bool {
	return namespace == structs.AllNamespacesSentinel
}

// nsCapIterFilter wraps an iterator with a filter for removing items that the token
// does not have permission to read (whether missing the capability or in the
// wrong namespace).
func nsCapIterFilter[T comparable](iter state.ResultIterator[T], aclObj *acl.ACL) state.ResultIterator[T] {
	return state.NewFilterIterator(iter, nsCapFilter[T](aclObj))
}

// nsCapFilter produces a memdb.FilterFunc for removing objects not accessible
// by aclObj during a table scan.
func nsCapFilter[T comparable](aclObj *acl.ACL) state.FilterFunc[T] {
	return func(t T) bool {
		switch t := any(t).(type) {
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
func nodePoolCapFilter(aclObj *acl.ACL) state.FilterFunc[*structs.NodePool] {
	return func(pool *structs.NodePool) bool {
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

			contexts := filteredSearchContexts(aclObj, namespace, args.Context)

			matches, truncations := getPrefixMatches(
				contexts, aclObj, namespace, args.Prefix, ws, state)
			reply.Matches = matches
			reply.Truncations = truncations

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
	if aclObj.IsManagement() {
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
	case structs.HostVolumes:
		return acl.NamespaceValidator(acl.NamespaceCapabilityHostVolumeRead)(aclObj, namespace)
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

			// // only apply on the types that use UUID prefix searching
			prefixContexts := filteredSearchContexts(aclObj, namespace, context)
			prefixContexts = fuzzyPrefixSearchContexts(prefixContexts)

			fuzzyContexts := filteredFuzzySearchContexts(aclObj, namespace, context)

			pmatches, truncations := getPrefixMatches(
				prefixContexts, aclObj, namespace, args.Prefix, ws, state)

			reply.Truncations = truncations
			for context, res := range pmatches {
				matches := make([]structs.FuzzyMatch, 0, len(res))
				for _, result := range res {
					matches = append(matches, structs.FuzzyMatch{ID: result})
				}
				reply.Matches[context] = matches
			}

			limitQuery := s.srv.config.SearchConfig.LimitQuery
			limitResults := s.srv.config.SearchConfig.LimitResults
			matches, truncations := getFuzzyMatches(
				fuzzyContexts, aclObj, namespace, text, ws, state, limitQuery, limitResults)
			for context, res := range matches {
				reply.Matches[context] = res

				// prefill truncations of iterable types so keys will exist in
				// the response for negative results
				reply.Truncations[context] = false
			}
			for context, trunc := range truncations {
				reply.Truncations[context] = trunc
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

func mergeFuzzyMatches(
	outMatches map[structs.Context][]structs.FuzzyMatch, outTruncs map[structs.Context]bool,
	inMatches map[structs.Context][]structs.FuzzyMatch, inTruncs map[structs.Context]bool,
) {
	for ctx, match := range inMatches {
		if _, ok := outMatches[ctx]; !ok {
			outMatches[ctx] = match
		} else {
			outMatches[ctx] = append(outMatches[ctx], match...)
		}
	}
	for ctx, trunc := range inTruncs {
		outTruncs[ctx] = trunc
	}
}

func getFuzzyMatches(contexts []structs.Context, aclObj *acl.ACL, namespace, text string,
	ws memdb.WatchSet, store *state.StateStore,
	limitQuery, limitResults int,
) (
	map[structs.Context][]structs.FuzzyMatch, map[structs.Context]bool) {

	matches := make(map[structs.Context][]structs.FuzzyMatch, len(contexts))
	truncs := make(map[structs.Context]bool, len(contexts))

	for _, context := range contexts {
		switch context {
		case structs.Jobs:
			var iter state.ResultIterator[*structs.Job]
			if wildcard(namespace) {
				iter = store.Jobs(ws, state.SortDefault)
			} else {
				var err error
				iter, err = store.JobsByNamespace(ws, namespace, state.SortDefault)
				if err != nil {
					continue
				}
			}
			iter = nsCapIterFilter(iter, aclObj)
			m, t := getFuzzyMatchesImpl(iter, text, limitQuery, limitResults)
			mergeFuzzyMatches(matches, truncs, m, t)

		case structs.Allocs:
			var iter state.ResultIterator[*structs.Allocation]
			if wildcard(namespace) {
				iter = store.Allocs(ws, state.SortDefault)
			} else {
				var err error
				iter, err = store.AllocsByNamespace(ws, namespace)
				if err != nil {
					continue
				}
			}
			iter = nsCapIterFilter(iter, aclObj)
			m, t := getFuzzyMatchesImpl(iter, text, limitQuery, limitResults)
			mergeFuzzyMatches(matches, truncs, m, t)

		case structs.Variables:
			var iter state.ResultIterator[*structs.VariableEncrypted]
			if wildcard(namespace) {
				iter = store.Variables(ws)
			} else {
				var err error
				iter, err = store.GetVariablesByNamespace(ws, namespace)
				if err != nil {
					continue
				}
			}
			iter = nsCapIterFilter(iter, aclObj)
			m, t := getFuzzyMatchesImpl(iter, text, limitQuery, limitResults)
			mergeFuzzyMatches(matches, truncs, m, t)

		case structs.Nodes:
			if !aclObj.AllowNodeRead() {
				continue
			}
			iter := store.Nodes(ws)
			m, t := getFuzzyMatchesImpl(iter, text, limitQuery, limitResults)
			mergeFuzzyMatches(matches, truncs, m, t)

		case structs.NodePools:
			iter := store.NodePools(ws, state.SortDefault)
			if !aclObj.IsManagement() {
				iter = state.NewFilterIterator(iter, nodePoolCapFilter(aclObj))
			}
			m, t := getFuzzyMatchesImpl(iter, text, limitQuery, limitResults)
			mergeFuzzyMatches(matches, truncs, m, t)

		case structs.Plugins:
			if !aclObj.AllowPluginRead() {
				continue
			}
			iter := store.CSIPlugins(ws)
			m, t := getFuzzyMatchesImpl(iter, text, limitQuery, limitResults)
			mergeFuzzyMatches(matches, truncs, m, t)

		case structs.Namespaces:
			iter := store.Namespaces(ws)
			iter = nsCapIterFilter(iter, aclObj)
			m, t := getFuzzyMatchesImpl(iter, text, limitQuery, limitResults)
			mergeFuzzyMatches(matches, truncs, m, t)

		default:
			// TODO
			// iter = getEnterpriseFuzzyResourceIter[](context, aclObj, namespace, ws, store)
		}
	}

	return matches, truncs
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
		case structs.HostVolumes:
			if acl.NamespaceValidator(
				acl.NamespaceCapabilityHostVolumeRead)(aclObj, namespace) {
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

func fuzzyPrefixSearchContexts(contexts []structs.Context) []structs.Context {
	out := []structs.Context{}
	for _, context := range contexts {
		switch context {
		case structs.Evals, structs.Deployments, structs.ScalingPolicies,
			structs.Volumes, structs.HostVolumes, structs.Quotas, structs.Recommendations:
			out = append(out, context)
		default:
			continue
		}
	}
	return out
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
