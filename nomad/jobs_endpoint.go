package nomad

import (
	"net/http"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-set/v2"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/state/paginator"
	"github.com/hashicorp/nomad/nomad/structs"
)

func NewJobsEndpoint(s *Server, ctx *RPCContext) *Jobs {
	return &Jobs{
		srv:    s,
		ctx:    ctx,
		logger: s.logger.Named("jobs"),
	}
}

type Jobs struct {
	srv    *Server
	ctx    *RPCContext
	logger hclog.Logger
}

func (j *Jobs) Statuses(
	args *structs.JobsStatusesRequest,
	reply *structs.JobsStatusesResponse) error {
	// TODO: auth, rate limiting, etc...

	// compare between state unblocks to see if the RPC should unblock (namely, if any jobs have gone away)
	prevJobs := set.New[structs.NamespacedID](0)

	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			var err error
			jobs := make([]structs.UIJob, 0)
			newJobs := set.New[structs.NamespacedID](0)
			for _, j := range args.Jobs {
				job, err := state.JobByID(ws, j.Namespace, j.ID)
				if err != nil {
					return err
				}
				if job == nil {
					continue
				}
				uiJob, idx, err := UIJobFromJob(ws, state, job)
				if err != nil {
					return err
				}
				jobs = append(jobs, uiJob)
				newJobs.Insert(job.NamespacedID())
				if idx > reply.Index {
					reply.Index = idx
				}
			}
			// mainly for if a job goes away
			if !newJobs.Equal(prevJobs) {
				reply.Index, err = state.Index("jobs")
				if err != nil {
					return err
				}
			}
			prevJobs = newJobs
			reply.Jobs = jobs
			j.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		}}
	return j.srv.blockingRPC(&opts)
}

func (j *Jobs) Statuses2(
	args *structs.JobsStatuses2Request,
	reply *structs.JobsStatuses2Response) error {

	// totally lifted from Job.List
	authErr := j.srv.Authenticate(j.ctx, args)
	if done, err := j.srv.forward("Jobs.Statuses2", args, args, reply); done {
		return err
	}
	j.srv.MeasureRPCRate("jobs", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "jobs", "statuses"}, time.Now())

	namespace := args.RequestNamespace()

	// Check for list-job permissions
	aclObj, err := j.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	if !aclObj.AllowNsOp(namespace, acl.NamespaceCapabilityListJobs) {
		return structs.ErrPermissionDenied
	}
	allow := aclObj.AllowNsOpFunc(acl.NamespaceCapabilityListJobs)

	// compare between state unblocks to see if the RPC should unblock (namely, if any jobs have gone away)
	prevJobs := set.New[structs.NamespacedID](0)

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Capture all the jobs
			var err error
			var iter memdb.ResultIterator

			// Get the namespaces the user is allowed to access.
			allowableNamespaces, err := allowedNSes(aclObj, state, allow)
			if err == structs.ErrPermissionDenied {
				// return empty jobs if token isn't authorized for any
				// namespace, matching other endpoints
				reply.Jobs = make([]structs.UIJob, 0)
			} else if err != nil {
				return err
			} else {
				if prefix := args.QueryOptions.Prefix; prefix != "" {
					iter, err = state.JobsByIDPrefix(ws, namespace, prefix)
				} else if namespace != structs.AllNamespacesSentinel {
					iter, err = state.JobsByNamespace(ws, namespace)
				} else {
					iter, err = state.Jobs(ws)
				}
				if err != nil {
					return err
				}

				tokenizer := paginator.NewStructsTokenizer(
					iter,
					paginator.StructsTokenizerOptions{
						WithNamespace: true,
						WithID:        true,
					},
				)
				filters := []paginator.Filter{
					paginator.NamespaceFilter{
						AllowableNamespaces: allowableNamespaces,
					},
				}

				jobs := make([]structs.UIJob, 0)
				newJobs := set.New[structs.NamespacedID](0)
				pager, err := paginator.NewPaginator(iter, tokenizer, filters, args.QueryOptions,
					func(raw interface{}) error {
						job := raw.(*structs.Job)
						//summary, err := state.JobSummaryByID(ws, job.Namespace, job.ID)
						//if err != nil || summary == nil {
						//	return fmt.Errorf("unable to look up summary for job: %v", job.ID)
						//}
						uiJob, idx, err := UIJobFromJob(ws, state, job)
						if err != nil {
							return err
						}

						jobs = append(jobs, uiJob)
						newJobs.Insert(job.NamespacedID())

						if idx > reply.Index {
							reply.Index = idx
						}
						return nil
					})
				if err != nil {
					return structs.NewErrRPCCodedf(
						http.StatusBadRequest, "failed to create result paginator: %v", err)
				}

				nextToken, err := pager.Page()
				if err != nil {
					return structs.NewErrRPCCodedf(
						http.StatusBadRequest, "failed to read result page: %v", err)
				}

				if !newJobs.Equal(prevJobs) {
					reply.Index, err = state.Index("jobs")
					if err != nil {
						return err
					}
				}
				prevJobs = newJobs

				reply.QueryMeta.NextToken = nextToken
				reply.Jobs = jobs
			}

			// Set the query response
			j.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		}}
	return j.srv.blockingRPC(&opts)
}

func UIJobFromJob(ws memdb.WatchSet, state *state.StateStore, job *structs.Job) (structs.UIJob, uint64, error) {
	idx := job.ModifyIndex

	uiJob := structs.UIJob{
		NamespacedID: structs.NamespacedID{
			ID:        job.ID,
			Namespace: job.Namespace,
		},
		Name:        job.Name,
		Type:        job.Type,
		NodePool:    job.NodePool,
		Datacenters: job.Datacenters,
		Priority:    job.Priority,
		Version:     job.Version,
		// included here for completeness, populated below.
		Allocs:        nil,
		ChildStatuses: nil,
		GroupCountSum: 0,
		DeploymentID:  "",
	}
	for _, tg := range job.TaskGroups {
		uiJob.GroupCountSum += tg.Count
	}

	if job.IsParameterized() || job.IsPeriodic() {
		children, err := state.JobsByIDPrefix(ws, job.Namespace, job.ID)
		if err != nil {
			return uiJob, idx, err
		}
		for {
			child := children.Next()
			if child == nil {
				break
			}
			j := child.(*structs.Job)
			if j.ParentID != job.ID {
				continue
			}
			if j.ModifyIndex > idx {
				idx = j.ModifyIndex
			}
			uiJob.ChildStatuses = append(uiJob.ChildStatuses, j.Status)
		}
	}

	allocs, err := state.AllocsByJob(ws, job.Namespace, job.ID, false)
	if err != nil {
		return uiJob, idx, err
	}
	for _, a := range allocs {
		alloc := structs.JobStatusAlloc{
			ID:           a.ID,
			Group:        a.TaskGroup,
			ClientStatus: a.ClientStatus,
			JobVersion:   a.Job.Version,
		}
		// TODO: use methods instead of fields directly?
		if a.DeploymentStatus != nil {
			alloc.DeploymentStatus.Canary = a.DeploymentStatus.Canary
			if a.DeploymentStatus.Healthy != nil {
				alloc.DeploymentStatus.Healthy = *a.DeploymentStatus.Healthy
			}
		}
		uiJob.Allocs = append(uiJob.Allocs, alloc)
		if a.ModifyIndex > idx { // TODO: unblock if an alloc goes away (like GC)
			idx = a.ModifyIndex
		}
	}

	deploys, err := state.DeploymentsByJobID(ws, job.Namespace, job.ID, false)
	if err != nil {
		return uiJob, idx, err
	}
	for _, d := range deploys {
		if d.Active() {
			uiJob.DeploymentID = d.ID
		}
		if d.ModifyIndex > idx {
			idx = d.ModifyIndex
		}
	}
	return uiJob, idx, nil
}

func (j *Jobs) Statuses3(
	args *structs.JobsStatusesRequest,
	reply *structs.JobsStatusesResponse) error {

	// totally lifted from Job.List
	authErr := j.srv.Authenticate(j.ctx, args)
	if done, err := j.srv.forward("Jobs.Statuses3", args, args, reply); done {
		return err
	}
	j.srv.MeasureRPCRate("jobs", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "jobs", "statuses"}, time.Now())

	namespace := args.RequestNamespace()
	// The ns from the UI by default is "*" which scans the whole "jobs" table.
	// If specific jobs are requested, all with the same namespace,
	// we may get some extra efficiency, especially for non-contiguous job IDs.
	if len(args.Jobs) > 0 {
		nses := set.New[string](0)
		for _, j := range args.Jobs {
			nses.Insert(j.Namespace)
		}
		if nses.Size() == 1 {
			namespace = nses.Slice()[0]
		}
	}

	// Check for list-job permissions
	aclObj, err := j.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	if !aclObj.AllowNsOp(namespace, acl.NamespaceCapabilityListJobs) {
		return structs.ErrPermissionDenied
	}
	allow := aclObj.AllowNsOpFunc(acl.NamespaceCapabilityListJobs)

	// compare between state run() unblocks to see if the RPC should unblock.
	// i.e. if new job(s) shift the page, or when job(s) go away.
	prevJobs := set.New[structs.NamespacedID](0)

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			var err error
			var iter memdb.ResultIterator

			// Get the namespaces the user is allowed to access.
			allowableNamespaces, err := allowedNSes(aclObj, state, allow)
			if err == structs.ErrPermissionDenied {
				// return empty jobs if token isn't authorized for any
				// namespace, matching other endpoints
				reply.Jobs = make([]structs.UIJob, 0)
			} else if err != nil {
				return err
			} else {
				if prefix := args.QueryOptions.Prefix; prefix != "" {
					iter, err = state.JobsByIDPrefix(ws, namespace, prefix)
				} else if namespace != structs.AllNamespacesSentinel {
					iter, err = state.JobsByNamespace(ws, namespace)
				} else {
					iter, err = state.Jobs(ws)
				}
				if err != nil {
					return err
				}

				tokenizer := paginator.NewStructsTokenizer(
					iter,
					paginator.StructsTokenizerOptions{
						WithNamespace: true,
						WithID:        true,
					},
				)
				filters := []paginator.Filter{
					paginator.NamespaceFilter{
						AllowableNamespaces: allowableNamespaces,
					},
					// don't include child jobs; we'll look them up later, per parent.
					paginator.GenericFilter{Allow: func(i interface{}) (bool, error) {
						job := i.(*structs.Job)
						return job.ParentID == "", nil
					}},
				}
				// only provide specific jobs if requested.
				if len(args.Jobs) > 0 {
					jobSet := set.From[structs.NamespacedID](args.Jobs)
					filters = append(filters, paginator.GenericFilter{
						Allow: func(i interface{}) (bool, error) {
							job := i.(*structs.Job)
							return jobSet.Contains(job.NamespacedID()), nil
						},
					})
				}

				jobs := make([]structs.UIJob, 0)
				newJobs := set.New[structs.NamespacedID](0)
				pager, err := paginator.NewPaginator(iter, tokenizer, filters, args.QueryOptions,
					func(raw interface{}) error {
						job := raw.(*structs.Job)
						if job == nil {
							return nil
						}

						uiJob, idx, err := UIJobFromJob(ws, state, job)
						if err != nil {
							return err
						}

						jobs = append(jobs, uiJob)
						newJobs.Insert(job.NamespacedID())

						if idx > reply.Index {
							reply.Index = idx
						}
						return nil
					})
				if err != nil {
					return structs.NewErrRPCCodedf(
						http.StatusBadRequest, "failed to create result paginator: %v", err)
				}

				nextToken, err := pager.Page()
				if err != nil {
					return structs.NewErrRPCCodedf(
						http.StatusBadRequest, "failed to read result page: %v", err)
				}

				// if the page has updated, or a job has gone away, bump the index to latest jobs entry.
				if !newJobs.Equal(prevJobs) {
					reply.Index, err = state.Index("jobs")
					if err != nil {
						return err
					}
				}
				prevJobs = newJobs

				reply.QueryMeta.NextToken = nextToken
				reply.Jobs = jobs
			}

			// Set the query response
			j.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		}}
	return j.srv.blockingRPC(&opts)
}
