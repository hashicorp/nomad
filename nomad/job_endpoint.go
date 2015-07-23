package nomad

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Job endpoint is used for job interactions
type Job struct {
	srv *Server
}

// Register is used to upsert a job for scheduling
func (j *Job) Register(args *structs.JobRegisterRequest, reply *structs.GenericResponse) error {
	if done, err := j.srv.forward("Job.Register", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "register"}, time.Now())

	// Validate the arguments
	if args.Job == nil {
		return fmt.Errorf("missing job for registration")
	}
	if args.Job.ID == "" {
		return fmt.Errorf("missing job ID for registration")
	}
	if args.Job.Name == "" {
		return fmt.Errorf("missing job name for registration")
	}

	// Commit this update via Raft
	_, index, err := j.srv.raftApply(structs.JobRegisterRequestType, args)
	if err != nil {
		j.srv.logger.Printf("[ERR] nomad.job: Register failed: %v", err)
		return err
	}

	// Set the reply index
	reply.Index = index
	return nil
}

// Deregister is used to remove a job the cluster.
func (j *Job) Deregister(args *structs.JobDeregisterRequest, reply *structs.GenericResponse) error {
	if done, err := j.srv.forward("Job.Deregister", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "deregister"}, time.Now())

	// Commit this update via Raft
	_, index, err := j.srv.raftApply(structs.JobDeregisterRequestType, args)
	if err != nil {
		j.srv.logger.Printf("[ERR] nomad.job: Deregister failed: %v", err)
		return err
	}

	// Set the reply index
	reply.Index = index
	return nil
}

// GetJob is used to request information about a specific job
func (j *Job) GetJob(args *structs.JobSpecificRequest,
	reply *structs.SingleJobResponse) error {
	if done, err := j.srv.forward("Job.GetJob", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "get_job"}, time.Now())

	// Look for the job
	snap, err := j.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}
	out, err := snap.GetJobByID(args.JobID)
	if err != nil {
		return err
	}

	// Setup the output
	if out != nil {
		reply.Job = out
		reply.Index = out.ModifyIndex
	} else {
		// Use the last index that affected the nodes table
		index, err := snap.GetIndex("jobs")
		if err != nil {
			return err
		}
		reply.Index = index
	}

	// Set the query response
	j.srv.setQueryMeta(&reply.QueryMeta)
	return nil
}
