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
func (j *Job) Register(args *structs.JobRegisterRequest, reply *structs.JobRegisterResponse) error {
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
	if args.Job.Type == "" {
		return fmt.Errorf("missing job type for registration")
	}
	if args.Job.Type == structs.JobTypeCore {
		return fmt.Errorf("job type cannot be core")
	}

	// Ensure priorities are bounded
	if args.Job.Priority < structs.JobMinPriority {
		args.Job.Priority = structs.JobMinPriority
	} else if args.Job.Priority > structs.JobMaxPriority {
		args.Job.Priority = structs.JobMaxPriority
	}

	// Commit this update via Raft
	_, index, err := j.srv.raftApply(structs.JobRegisterRequestType, args)
	if err != nil {
		j.srv.logger.Printf("[ERR] nomad.job: Register failed: %v", err)
		return err
	}

	// Create a new evaluation
	eval := &structs.Evaluation{
		ID:             generateUUID(),
		Priority:       args.Job.Priority,
		Type:           args.Job.Type,
		TriggeredBy:    structs.EvalTriggerJobRegister,
		JobID:          args.Job.ID,
		JobModifyIndex: index,
		Status:         structs.EvalStatusPending,
	}
	update := &structs.EvalUpdateRequest{
		Evals:        []*structs.Evaluation{eval},
		WriteRequest: structs.WriteRequest{Region: args.Region},
	}

	// Commit this evaluation via Raft
	_, evalIndex, err := j.srv.raftApply(structs.EvalUpdateRequestType, update)
	if err != nil {
		j.srv.logger.Printf("[ERR] nomad.job: Eval create failed: %v", err)
		return err
	}

	// Setup the reply
	reply.EvalID = eval.ID
	reply.EvalCreateIndex = evalIndex
	reply.JobModifyIndex = index
	reply.Index = evalIndex
	return nil
}

// Deregister is used to remove a job the cluster.
func (j *Job) Deregister(args *structs.JobDeregisterRequest, reply *structs.JobDeregisterResponse) error {
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

	// Create a new evaluation
	// XXX: The job priority / type is strange for this, since it's not a high
	// priority even if the job was. The scheduler itself also doesn't matter,
	// since all should be able to handle deregistration in the same way.
	eval := &structs.Evaluation{
		ID:             generateUUID(),
		Priority:       structs.JobDefaultPriority,
		Type:           structs.JobTypeService,
		TriggeredBy:    structs.EvalTriggerJobDeregister,
		JobID:          args.JobID,
		JobModifyIndex: index,
		Status:         structs.EvalStatusPending,
	}
	update := &structs.EvalUpdateRequest{
		Evals:        []*structs.Evaluation{eval},
		WriteRequest: structs.WriteRequest{Region: args.Region},
	}

	// Commit this evaluation via Raft
	_, evalIndex, err := j.srv.raftApply(structs.EvalUpdateRequestType, update)
	if err != nil {
		j.srv.logger.Printf("[ERR] nomad.job: Eval create failed: %v", err)
		return err
	}

	// Setup the reply
	reply.EvalID = eval.ID
	reply.EvalCreateIndex = evalIndex
	reply.JobModifyIndex = index
	reply.Index = evalIndex
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
