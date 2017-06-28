package nomad

import "github.com/hashicorp/nomad/nomad/structs"

// deploymentWatcherStateShim is the shim that provides the state watching
// methods. These should be set by the server and passed to the deployment
// watcher.
type deploymentWatcherStateShim struct {
	// region is the region the server is a member of. It is used to
	// auto-populate requests that do not have it set
	region string

	// evaluations returns the set of evaluations for the given job
	evaluations func(args *structs.JobSpecificRequest, reply *structs.JobEvaluationsResponse) error

	// allocations returns the set of allocations that are part of the
	// deployment.
	allocations func(args *structs.DeploymentSpecificRequest, reply *structs.AllocListResponse) error

	// list is used to list all the deployments in the system
	list func(args *structs.DeploymentListRequest, reply *structs.DeploymentListResponse) error

	// getJobVersions is used to lookup the versions of a job. This is used when
	// rolling back to find the latest stable job
	getJobVersions func(args *structs.JobSpecificRequest, reply *structs.JobVersionsResponse) error

	// getJob is used to lookup a particular job.
	getJob func(args *structs.JobSpecificRequest, reply *structs.SingleJobResponse) error
}

func (d *deploymentWatcherStateShim) Evaluations(args *structs.JobSpecificRequest, reply *structs.JobEvaluationsResponse) error {
	if args.Region == "" {
		args.Region = d.region
	}

	return d.evaluations(args, reply)
}

func (d *deploymentWatcherStateShim) Allocations(args *structs.DeploymentSpecificRequest, reply *structs.AllocListResponse) error {
	if args.Region == "" {
		args.Region = d.region
	}

	return d.allocations(args, reply)
}

func (d *deploymentWatcherStateShim) List(args *structs.DeploymentListRequest, reply *structs.DeploymentListResponse) error {
	if args.Region == "" {
		args.Region = d.region
	}

	return d.list(args, reply)
}

func (d *deploymentWatcherStateShim) GetJobVersions(args *structs.JobSpecificRequest, reply *structs.JobVersionsResponse) error {
	if args.Region == "" {
		args.Region = d.region
	}

	return d.getJobVersions(args, reply)
}

func (d *deploymentWatcherStateShim) GetJob(args *structs.JobSpecificRequest, reply *structs.SingleJobResponse) error {
	if args.Region == "" {
		args.Region = d.region
	}

	return d.getJob(args, reply)
}

// deploymentWatcherRaftShim is the shim that provides the state watching
// methods. These should be set by the server and passed to the deployment
// watcher.
type deploymentWatcherRaftShim struct {
	// apply is used to apply a message to Raft
	apply raftApplyFn
}

func (d *deploymentWatcherRaftShim) UpsertEvals(evals []*structs.Evaluation) (uint64, error) {
	update := &structs.EvalUpdateRequest{
		Evals: evals,
	}
	_, index, err := d.apply(structs.EvalUpdateRequestType, update)
	return index, err
}

func (d *deploymentWatcherRaftShim) UpsertJob(job *structs.Job) (uint64, error) {
	update := &structs.JobRegisterRequest{
		Job: job,
	}
	_, index, err := d.apply(structs.JobRegisterRequestType, update)
	return index, err
}

func (d *deploymentWatcherRaftShim) UpsertDeploymentStatusUpdate(u *structs.DeploymentStatusUpdateRequest) (uint64, error) {
	_, index, err := d.apply(structs.DeploymentStatusUpdateRequestType, u)
	return index, err
}

func (d *deploymentWatcherRaftShim) UpsertDeploymentPromotion(req *structs.ApplyDeploymentPromoteRequest) (uint64, error) {
	_, index, err := d.apply(structs.DeploymentPromoteRequestType, req)
	return index, err
}

func (d *deploymentWatcherRaftShim) UpsertDeploymentAllocHealth(req *structs.ApplyDeploymentAllocHealthRequest) (uint64, error) {
	_, index, err := d.apply(structs.DeploymentAllocHealthRequestType, req)
	return index, err
}
