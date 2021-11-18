package eval_priority

import (
	"fmt"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
)

type EvalPriorityTest struct {
	framework.TC
	jobIDs []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "EvalPriority",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(EvalPriorityTest),
		},
	})
}

func (tc *EvalPriorityTest) BeforeAll(f *framework.F) {
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 1)
}

func (tc *EvalPriorityTest) AfterEach(f *framework.F) {
	for _, id := range tc.jobIDs {
		_, _, err := tc.Nomad().Jobs().Deregister(id, true, nil)
		f.NoError(err)
	}
	tc.jobIDs = []string{}

	_, err := e2eutil.Command("nomad", "system", "gc")
	f.NoError(err)
}

// TestJobRegisterWithoutEvalPriority makes sure that when not specifying an
// eval priority on job register, the job priority is used.
func (tc *EvalPriorityTest) TestJobRegisterWithoutEvalPriority(f *framework.F) {

	// Generate a jobID and attempt to register the job using the eval
	// priority. In case there is a problem found here and the job registers,
	// we need to ensure it gets cleaned up.
	jobID := "test-eval-priority-" + uuid.Generate()[0:8]
	f.NoError(e2eutil.Register(jobID, "eval_priority/inputs/default_job_priority.nomad"))
	tc.jobIDs = append(tc.jobIDs, jobID)

	// Wait for the deployment to finish.
	f.NoError(e2eutil.WaitForLastDeploymentStatus(jobID, "default", "successful", nil))

	// Pull the job evaluation list from the API and ensure that this didn't
	// error and contains two evals.
	//
	// Eval 1: the job registration eval.
	// Eval 2: the deployment watcher eval.
	evals, _, err := tc.Nomad().Jobs().Evaluations(jobID, nil)
	f.NoError(err)
	f.Len(evals, 2, "job expected to have one eval")

	// All evaluations should have the higher priority as they are as a result
	// of an operator request.
	for _, eval := range evals {
		f.Equal(50, eval.Priority)
	}
}

// TestJobRegisterWithEvalPriority tests registering a job with a specified
// evaluation priority. It checks whether the created evaluation has a priority
// that matches our supplied value.
func (tc *EvalPriorityTest) TestJobRegisterWithEvalPriority(f *framework.F) {

	// Pick an eval priority.
	evalPriority := 93

	// Generate a jobID and register the job using the eval priority.
	jobID := "test-eval-priority-" + uuid.Generate()[0:8]
	f.NoError(e2eutil.RegisterWithArgs(jobID,
		"eval_priority/inputs/default_job_priority.nomad",
		fmt.Sprintf("-eval-priority=%v", evalPriority)))
	tc.jobIDs = append(tc.jobIDs, jobID)

	// Wait for the deployment to finish.
	f.NoError(e2eutil.WaitForLastDeploymentStatus(jobID, "default", "successful", nil))

	// Pull the job evaluation list from the API and ensure that this didn't
	// error and contains two evals.
	//
	// Eval 1: the job registration eval.
	// Eval 2: the deployment watcher eval.
	evals, _, err := tc.Nomad().Jobs().Evaluations(jobID, nil)
	f.NoError(err)
	f.Len(evals, 2, "job expected to have one eval")

	// All evaluations should have the higher priority as they are as a result
	// of an operator request.
	for _, eval := range evals {
		f.Equal(evalPriority, eval.Priority)
	}
}

// TestJobRegisterWithInvalidEvalPriority tests registering a job with a
// specified evaluation priority that is not valid.
func (tc *EvalPriorityTest) TestJobRegisterWithInvalidEvalPriority(f *framework.F) {

	// Pick an eval priority that is outside the supported bounds of 1-100.
	evalPriority := 999

	// Generate a jobID and attempt to register the job using the eval
	// priority. In case there is a problem found here and the job registers,
	// we need to ensure it gets cleaned up.
	jobID := "test-eval-priority-" + uuid.Generate()[0:8]
	f.Error(e2eutil.RegisterWithArgs(jobID,
		"eval_priority/inputs/default_job_priority.nomad",
		fmt.Sprintf("-eval-priority=%v", evalPriority)))
}

// TestJobDeregisterWithoutEvalPriority makes sure that when not specifying an
// eval priority on job deregister, the job priority is used.
func (tc *EvalPriorityTest) TestJobDeregisterWithoutEvalPriority(f *framework.F) {

	// Generate a jobID and attempt to register the job using the eval
	// priority. In case there is a problem found here and the job registers,
	// we need to ensure it gets cleaned up.
	jobID := "test-eval-priority-" + uuid.Generate()[0:8]
	f.NoError(e2eutil.Register(jobID, "eval_priority/inputs/default_job_priority.nomad"))
	tc.jobIDs = append(tc.jobIDs, jobID)

	// Wait for the deployment to finish.
	f.NoError(e2eutil.WaitForLastDeploymentStatus(jobID, "default", "successful", nil))

	// Deregister the job.
	_, _, err := tc.Nomad().Jobs().Deregister(jobID, true, nil)
	f.NoError(err)

	// Grab the evals from the server and ensure we actually got some.
	evals, _, err := tc.Nomad().Jobs().Evaluations(jobID, nil)
	f.NoError(err)
	f.NotZero(len(evals))

	// Identify the evaluation which was a result of the deregsiter and check
	// the priority.
	var deregEval *api.Evaluation
	for _, eval := range evals {
		if eval.TriggeredBy == "job-deregister" {
			deregEval = eval
			break
		}
	}
	f.NotNil(deregEval)
	f.Equal(50, deregEval.Priority)
	tc.jobIDs = []string{}
}

// TestJobDeregisterWithEvalPriority tests deregistering a job with a specified
// evaluation priority. It checks whether the created evaluation has a priority
// that matches our supplied value.
func (tc *EvalPriorityTest) TestJobDeregisterWithEvalPriority(f *framework.F) {

	// Generate a jobID and register the job using the eval priority.
	jobID := "test-eval-priority-" + uuid.Generate()[0:8]
	f.NoError(e2eutil.Register(jobID, "eval_priority/inputs/default_job_priority.nomad"))
	tc.jobIDs = append(tc.jobIDs, jobID)

	// Wait for the deployment to finish.
	f.NoError(e2eutil.WaitForLastDeploymentStatus(jobID, "default", "successful", nil))

	// Pick an eval priority.
	evalPriority := 91

	// Deregister the job.
	_, _, err := tc.Nomad().Jobs().DeregisterOpts(jobID, &api.DeregisterOptions{EvalPriority: evalPriority}, nil)
	f.NoError(err)

	// Grab the evals from the server and ensure we actually got some.
	evals, _, err := tc.Nomad().Jobs().Evaluations(jobID, nil)
	f.NoError(err)
	f.NotZero(len(evals))

	// Identify the evaluation which was a result of the deregsiter and check
	// the priority.
	var deregEval *api.Evaluation
	for _, eval := range evals {
		if eval.TriggeredBy == "job-deregister" {
			deregEval = eval
			break
		}
	}
	f.NotNil(deregEval)
	f.Equal(evalPriority, deregEval.Priority)
	tc.jobIDs = []string{}
}

// TestJobDeregisterWithInvalidEvalPriority tests deregistering a job with a
// specified evaluation priority that is not valid. There is no need to
// register a job first as the validation is done within the agent HTTP handler
// before sending a server RPC request.
func (tc *EvalPriorityTest) TestJobDeregisterWithInvalidEvalPriority(f *framework.F) {

	// Pick an eval priority that is outside the supported bounds of 1-100.
	evalPriority := 999

	// Generate a jobID and attempt to register the job using the eval
	// priority. In case there is a problem found here and the job registers,
	// we need to ensure it gets cleaned up.
	jobID := "test-eval-priority-" + uuid.Generate()[0:8]
	_, _, err := tc.Nomad().Jobs().DeregisterOpts(jobID, &api.DeregisterOptions{EvalPriority: evalPriority}, nil)
	f.Error(err, "an error was expected due to invalid eval priority")
}
