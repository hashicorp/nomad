package nomad

import (
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/watch"
)

type JobSummary struct {
	srv *Server
}

func (j *JobSummary) GetJobSummary(args *structs.JobSummaryRequest,
	reply *structs.SingleJobSummaryResponse) error {
	if done, err := j.srv.forward("JobSummary.GetJobSummary", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "job_summary", "get_job_summary"}, time.Now())
	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		watch:     watch.NewItems(watch.Item{JobSummary: args.JobID}),
		run: func() error {

			// Look for the job
			snap, err := j.srv.fsm.State().Snapshot()
			if err != nil {
				return err
			}
			out, err := snap.JobSummaryByID(args.JobID)
			if err != nil {
				return err
			}

			// Setup the output
			reply.JobSummary = out
			if out != nil {
				reply.Index = out.ModifyIndex
			} else {
				// Use the last index that affected the nodes table
				index, err := snap.Index("job_summary")
				if err != nil {
					return err
				}
				reply.Index = index
			}

			// Set the query response
			j.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		}}
	return j.srv.blockingRPC(&opts)

	return nil
}

func (j *JobSummary) List() error {
	return nil
}
