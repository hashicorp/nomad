// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"errors"
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

	authErr := j.srv.Authenticate(j.ctx, args)
	if done, err := j.srv.forward("Jobs.Statuses", args, args, reply); done {
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

	// Check for read-job permissions, since this endpoint includes alloc info
	// and possibly a deployment ID, and those APIs require read-job.
	aclObj, err := j.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	if !aclObj.AllowNsOp(namespace, acl.NamespaceCapabilityReadJob) {
		return structs.ErrPermissionDenied
	}
	allow := aclObj.AllowNsOpFunc(acl.NamespaceCapabilityReadJob)

	// Compare between state run() unblocks to see if the RPC, as a whole,
	// should unblock. i.e. if new jobs shift the page, or when jobs go away.
	prevJobs := set.New[structs.NamespacedID](0)

	sort := state.QueryOptionSort(args.QueryOptions)

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			var err error
			var iter memdb.ResultIterator

			// Get the namespaces the user is allowed to access.
			allowableNamespaces, err := allowedNSes(aclObj, state, allow)
			if errors.Is(err, structs.ErrPermissionDenied) {
				// return empty jobs if token isn't authorized for any
				// namespace, matching other endpoints
				reply.Jobs = make([]structs.UIJob, 0)
				return nil
			} else if err != nil {
				return err
			}

			// the UI jobs index page shows most-recently changed at the top,
			// and with pagination, we need to sort here on the backend.
			if true { // TODO: parameterize...?
				iter, err = state.JobsByModifyIndex(ws, sort)
			} else {
				if prefix := args.QueryOptions.Prefix; prefix != "" {
					iter, err = state.JobsByIDPrefix(ws, namespace, prefix, sort)
				} else if namespace != structs.AllNamespacesSentinel {
					iter, err = state.JobsByNamespace(ws, namespace, sort)
				} else {
					iter, err = state.Jobs(ws, sort)
				}
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
				// skip child jobs; we'll look them up later, per parent.
				paginator.GenericFilter{Allow: func(i interface{}) (bool, error) {
					job := i.(*structs.Job)
					return job.ParentID == "", nil
				}},
			}
			// only provide specific jobs if requested.
			if len(args.Jobs) > 0 {
				// set per-page to avoid iterating the whole table
				args.QueryOptions.PerPage = int32(len(args.Jobs))
				// filter in the requested jobs
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

					// this is where the sausage is made
					uiJob, highestIndexOnPage, err := UIJobFromJob(ws, state, job, args.SmartOnly)
					if err != nil {
						return err
					}

					jobs = append(jobs, uiJob)
					newJobs.Insert(job.NamespacedID())

					// by using the highest index we find on any job/alloc/
					// deployment among the jobs on the page, instead of the
					// latest index for any particular state table, we can
					// avoid unblocking the RPC if something changes "off page"
					if highestIndexOnPage > reply.Index {
						reply.Index = highestIndexOnPage
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

			// if the page has updated, or a job has gone away,
			// bump the index to latest jobs entry.
			if !prevJobs.Empty() && !newJobs.Equal(prevJobs) {
				reply.Index, err = state.Index("jobs")
				if err != nil {
					return err
				}
			}
			prevJobs = newJobs

			reply.QueryMeta.NextToken = nextToken
			reply.Jobs = jobs

			return nil
		}}
	return j.srv.blockingRPC(&opts)
}

func UIJobFromJob(ws memdb.WatchSet, store *state.StateStore, job *structs.Job, smartOnly bool) (structs.UIJob, uint64, error) {
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
		Allocs:             nil,
		SmartAlloc:         make(map[string]int),
		GroupCountSum:      0,
		ChildStatuses:      nil,
		ActiveDeploymentID: "",
		SubmitTime:         job.SubmitTime,
		ModifyIndex:        job.ModifyIndex,
	}

	// the GroupCountSum will map to how many allocations we expect to run
	// (for service jobs)
	for _, tg := range job.TaskGroups {
		uiJob.GroupCountSum += tg.Count
	}

	// collect the statuses of child jobs
	if job.IsParameterized() || job.IsPeriodic() {
		children, err := store.JobsByIDPrefix(ws, job.Namespace, job.ID, state.SortDefault)
		if err != nil {
			return uiJob, idx, err
		}
		for {
			child := children.Next()
			if child == nil {
				break
			}
			j := child.(*structs.Job)
			// note: this filters out grandchildren jobs (children of children)
			if j.ParentID != job.ID {
				continue
			}
			if j.ModifyIndex > idx {
				idx = j.ModifyIndex
			}
			uiJob.ChildStatuses = append(uiJob.ChildStatuses, j.Status)
		}
		// no allocs or deployments for parameterized/period jobs,
		// so we're done here.
		return uiJob, idx, err
	}

	// collect info about allocations
	allocs, err := store.AllocsByJob(ws, job.Namespace, job.ID, true)
	if err != nil {
		return uiJob, idx, err
	}
	for _, a := range allocs {
		if a.ModifyIndex > idx {
			idx = a.ModifyIndex
		}

		uiJob.SmartAlloc["total"]++
		uiJob.SmartAlloc[a.ClientStatus]++
		if a.DeploymentStatus != nil && a.DeploymentStatus.Canary {
			uiJob.SmartAlloc["canary"]++
		}
		// callers may wish to keep response body size smaller by excluding
		// details about allocations.
		if smartOnly {
			continue
		}

		alloc := structs.JobStatusAlloc{
			ID:           a.ID,
			Group:        a.TaskGroup,
			ClientStatus: a.ClientStatus,
			NodeID:       a.NodeID,
			JobVersion:   a.Job.Version,
		}
		if a.DeploymentStatus != nil {
			alloc.DeploymentStatus.Canary = a.DeploymentStatus.IsCanary()
			alloc.DeploymentStatus.Healthy = a.DeploymentStatus.IsHealthy()
		}
		uiJob.Allocs = append(uiJob.Allocs, alloc)
	}

	// look for active deployment
	deploy, err := store.LatestDeploymentByJobID(ws, job.Namespace, job.ID)
	if err != nil {
		return uiJob, idx, err
	}
	if deploy != nil {
		if deploy.Active() {
			uiJob.ActiveDeploymentID = deploy.ID
		}
		if deploy.ModifyIndex > idx {
			idx = deploy.ModifyIndex
		}
	}
	return uiJob, idx, nil
}
