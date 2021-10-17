package nomad

import (
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Periodic endpoint is used for periodic job interactions
type Periodic struct {
	srv    *Server
	logger log.Logger
}

// Force is used to force a new instance of a periodic job
func (p *Periodic) Force(args *structs.PeriodicForceRequest, reply *structs.PeriodicForceResponse) error {
	if done, err := p.srv.forward("Periodic.Force", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "periodic", "force"}, time.Now())

	// Check for write-job permissions
	if aclObj, err := p.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityDispatchJob) && !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilitySubmitJob) {
		return structs.ErrPermissionDenied
	}

	// Validate the arguments
	if args.JobID == "" {
		return fmt.Errorf("missing job ID for evaluation")
	}

	// Lookup the job
	snap, err := p.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}

	ws := memdb.NewWatchSet()
	job, err := snap.JobByID(ws, args.RequestNamespace(), args.JobID)
	if err != nil {
		return err
	}
	if job == nil {
		return fmt.Errorf("job not found")
	}

	if !job.IsPeriodic() {
		return fmt.Errorf("can't force launch non-periodic job")
	}

	// Force run the job.
	eval, err := p.srv.periodicDispatcher.ForceRun(args.RequestNamespace(), job.ID)
	if err != nil {
		return fmt.Errorf("force launch for job %q failed: %v", job.ID, err)
	}

	reply.EvalID = eval.ID
	reply.EvalCreateIndex = eval.CreateIndex
	reply.Index = eval.CreateIndex
	return nil
}
