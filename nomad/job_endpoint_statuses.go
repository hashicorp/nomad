// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"errors"
	"net/http"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/state/paginator"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Statuses looks up info about jobs, their allocs, and latest deployment.
func (j *Job) Statuses(
	args *structs.JobStatusesRequest,
	reply *structs.JobStatusesResponse) error {

	authErr := j.srv.Authenticate(j.ctx, args)
	if done, err := j.srv.forward("Job.Statuses", args, args, reply); done {
		return err
	}
	j.srv.MeasureRPCRate("job", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "jobs", "statuses"}, time.Now())

	namespace := args.RequestNamespace()
	// the namespace from the UI by default is "*", but if specific jobs are
	// requested, all with the same namespace, AllowNsOp() below may be able
	// to quickly deny the request if the token lacks permissions for that ns,
	// rather than iterating the whole jobs table and filtering out every job.
	if len(args.Jobs) > 0 {
		nses := set.New[string](1)
		for _, j := range args.Jobs {
			nses.Insert(j.Namespace)
		}
		if nses.Size() == 1 {
			namespace = nses.Slice()[0]
		}
	}

	// check for read-job permissions, since this endpoint includes alloc info
	// and possibly a deployment ID, and those APIs require read-job.
	aclObj, err := j.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	if !aclObj.AllowNsOp(namespace, acl.NamespaceCapabilityReadJob) {
		return structs.ErrPermissionDenied
	}
	allow := aclObj.AllowNsOpFunc(acl.NamespaceCapabilityReadJob)

	store := j.srv.State()

	// get the namespaces the user is allowed to access.
	allowableNamespaces, err := allowedNSes(aclObj, store, allow)
	if errors.Is(err, structs.ErrPermissionDenied) {
		// return empty jobs if token isn't authorized for any
		// namespace, matching other endpoints
		reply.Jobs = make([]structs.JobStatusesJob, 0)
		return nil
	} else if err != nil {
		return err
	}
	// since the state index we're using doesn't include namespace, explicitly
	// set the user-provided ns to our filter if needed.  we've already verified
	// that the user has access to the specific namespace above
	if namespace != "" && namespace != structs.AllNamespacesSentinel {
		allowableNamespaces = map[string]bool{
			namespace: true,
		}
	}

	// compare between state run() unblocks to see if the RPC, as a whole,
	// should unblock. i.e. if new jobs shift the page, or when jobs go away.
	prevJobs := set.New[structs.NamespacedID](0)

	// because the state index is in order of ModifyIndex, lowest to highest,
	// SortDefault would show oldest jobs first, so instead invert the default
	// to show most recent job changes first.
	args.QueryOptions.Reverse = !args.QueryOptions.Reverse
	sort := state.QueryOptionSort(args.QueryOptions)

	// special blocking note: this endpoint employs an unconventional method
	// of determining the reply.Index in order to avoid unblocking when
	// something changes "off page" -- instead of using the latest index
	// from any/all of the state tables queried here (all of: "jobs", "allocs",
	// "deployments"), we use the highest ModifyIndex of all items encountered
	// while iterating.
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			var err error
			var iter memdb.ResultIterator

			// the UI jobs index page shows most-recently changed first.
			iter, err = state.JobsByModifyIndex(ws, sort)
			if err != nil {
				return err
			}

			// set up tokenizer and filters
			tokenizer := paginator.NewStructsTokenizer(
				iter,
				paginator.StructsTokenizerOptions{
					OnlyModifyIndex: true,
				},
			)
			filters := []paginator.Filter{
				paginator.NamespaceFilter{
					AllowableNamespaces: allowableNamespaces,
				},
				// skip child jobs unless requested to include them
				paginator.GenericFilter{Allow: func(i interface{}) (bool, error) {
					if args.IncludeChildren {
						return true, nil
					}
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

			jobs := make([]structs.JobStatusesJob, 0)
			newJobs := set.New[structs.NamespacedID](0)
			pager, err := paginator.NewPaginator(iter, tokenizer, filters, args.QueryOptions,
				func(raw interface{}) error {
					job := raw.(*structs.Job)

					// this is where the sausage is made
					jsj, highestIndexOnPage, err := jobStatusesJobFromJob(ws, state, job)
					if err != nil {
						return err
					}

					jobs = append(jobs, jsj)
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
					http.StatusInternalServerError, "failed to create result paginator: %v", err)
			}

			nextToken, err := pager.Page()
			if err != nil {
				return structs.NewErrRPCCodedf(
					http.StatusInternalServerError, "failed to read result page: %v", err)
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

func jobStatusesJobFromJob(ws memdb.WatchSet, store *state.StateStore, job *structs.Job) (structs.JobStatusesJob, uint64, error) {
	highestIdx := job.ModifyIndex

	jsj := structs.JobStatusesJob{
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
		ParentID:    job.ParentID,
		SubmitTime:  job.SubmitTime,
		ModifyIndex: job.ModifyIndex,
		// included here for completeness, populated below.
		Allocs:           nil,
		GroupCountSum:    0,
		ChildStatuses:    nil,
		LatestDeployment: nil,
		Stop:             job.Stop,
		Status:           job.Status,
	}

	_, jsj.IsPack = job.Meta["pack.name"]

	// the GroupCountSum will map to how many allocations we expect to run
	// (for service jobs)
	for _, tg := range job.TaskGroups {
		jsj.GroupCountSum += tg.Count
	}

	// collect the statuses of child jobs
	if job.IsParameterized() || job.IsPeriodic() {
		jsj.ChildStatuses = make([]string, 0) // set to not-nil
		children, err := store.JobsByIDPrefix(ws, job.Namespace, job.ID, state.SortDefault)
		if err != nil {
			return jsj, highestIdx, err
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
			if j.ModifyIndex > highestIdx {
				highestIdx = j.ModifyIndex
			}
			jsj.ChildStatuses = append(jsj.ChildStatuses, j.Status)
		}
		// no allocs or deployments for parameterized/period jobs,
		// so we're done here.
		return jsj, highestIdx, err
	}

	// collect info about allocations
	allocs, err := store.AllocsByJob(ws, job.Namespace, job.ID, true)
	if err != nil {
		return jsj, highestIdx, err
	}
	for _, a := range allocs {
		jsa := structs.JobStatusesAlloc{
			ID:             a.ID,
			Group:          a.TaskGroup,
			ClientStatus:   a.ClientStatus,
			NodeID:         a.NodeID,
			JobVersion:     a.Job.Version,
			FollowupEvalID: a.FollowupEvalID,
			HasPausedTask:  a.HasAnyPausedTasks(),
		}
		if a.DeploymentStatus != nil {
			jsa.DeploymentStatus.Canary = a.DeploymentStatus.IsCanary()
			jsa.DeploymentStatus.Healthy = a.DeploymentStatus.Healthy
		}
		jsj.Allocs = append(jsj.Allocs, jsa)

		if a.ModifyIndex > highestIdx {
			highestIdx = a.ModifyIndex
		}
	}

	// look for latest deployment
	deploy, err := store.LatestDeploymentByJobID(ws, job.Namespace, job.ID)
	if err != nil {
		return jsj, highestIdx, err
	}
	if deploy != nil {
		jsj.LatestDeployment = &structs.JobStatusesLatestDeployment{
			ID:                deploy.ID,
			IsActive:          deploy.Active(),
			JobVersion:        deploy.JobVersion,
			Status:            deploy.Status,
			StatusDescription: deploy.StatusDescription,
			AllAutoPromote:    deploy.HasAutoPromote(),
			RequiresPromotion: deploy.RequiresPromotion(),
		}

		if deploy.ModifyIndex > highestIdx {
			highestIdx = deploy.ModifyIndex
		}
	}
	return jsj, highestIdx, nil
}
