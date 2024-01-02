// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package jobs3

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/hashicorp/go-set"
	nomadapi "github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/v3/util3"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/jobspec2"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

type Submission struct {
	t *testing.T

	nomadClient *nomadapi.Client

	jobSpec       string
	jobID         string
	origJobID     string
	noRandomJobID bool
	noCleanup     bool
	timeout       time.Duration
	verbose       bool

	vars         *set.Set[string] // key=value
	waitComplete *set.Set[string] // groups to wait until complete
	inNamespace  string
	authToken    string
}

func (sub *Submission) queryOptions() *nomadapi.QueryOptions {
	return &nomadapi.QueryOptions{
		Namespace: sub.inNamespace,
		AuthToken: sub.authToken,
	}
}

type Logs struct {
	Stdout string
	Stderr string
}

// TaskLogs returns the logs of the given task, using a random allocation of
// the given group.
func (sub *Submission) TaskLogs(group, task string) Logs {
	byAlloc := sub.TaskLogsByAlloc(group, task)
	must.Positive(sub.t, len(byAlloc), must.Sprintf("no allocations found for %s/%s", group, task))

	var result Logs
	for _, logs := range byAlloc {
		result = logs
		break
	}
	return result
}

// TaskLogsByAlloc returns the logs of the given task, organized by allocation.
func (sub *Submission) TaskLogsByAlloc(group, task string) map[string]Logs {
	result := make(map[string]Logs)

	// get list of allocs for the job
	queryOpts := sub.queryOptions()
	jobsAPI := sub.nomadClient.Jobs()
	stubs, _, err := jobsAPI.Allocations(sub.jobID, false, queryOpts)
	must.NoError(sub.t, err, must.Sprintf("failed to query allocations for %s/%s", group, task))

	// get logs for each task in the group allocations
	for _, stub := range stubs {
		if stub.TaskGroup == group {
			result[stub.ID] = sub.getTaskLogs(stub.ID, task)
		}
	}
	return result
}

func (sub *Submission) getTaskLogs(allocID, task string) Logs {
	queryOpts := sub.queryOptions()
	allocAPI := sub.nomadClient.Allocations()
	alloc, _, err := allocAPI.Info(allocID, queryOpts)
	must.NoError(sub.t, err, must.Sprintf("failed to query allocation for %s", allocID))

	fsAPI := sub.nomadClient.AllocFS()
	read := func(path string) string {
		rc, err := fsAPI.ReadAt(alloc, path, 0, 0, queryOpts)
		must.NoError(sub.t, err, must.Sprintf("failed to read alloc logs for %s", allocID))
		b, err := io.ReadAll(rc)
		must.NoError(sub.t, err, must.Sprintf("failed to read alloc logs for %s", allocID))
		must.NoError(sub.t, rc.Close(), must.Sprint("failed to close log stream"))
		return string(b)
	}

	stdout := fmt.Sprintf("alloc/logs/%s.stdout.0", task)
	stderr := fmt.Sprintf("alloc/logs/%s.stderr.0", task)

	return Logs{
		Stdout: read(stdout),
		Stderr: read(stderr),
	}
}

// JobID provides the (possibly) randomized jobID associated with this Submission.
func (sub *Submission) JobID() string {
	return sub.jobID
}

func (sub *Submission) logf(msg string, args ...any) {
	sub.t.Helper()
	util3.Log3(sub.t, sub.verbose, msg, args...)
}

func (sub *Submission) cleanup() {
	if sub.noCleanup {
		return
	}

	// deregister the job that was submitted
	jobsAPI := sub.nomadClient.Jobs()
	sub.logf("deregister job %q", sub.jobID)
	_, _, err := jobsAPI.Deregister(sub.jobID, true, &nomadapi.WriteOptions{
		Namespace: sub.inNamespace,
	})
	test.NoError(sub.t, err, test.Sprintf("failed to deregister job %q", sub.origJobID))

	// force a system gc just in case
	sysAPI := sub.nomadClient.System()
	sub.logf("system gc")
	err = sysAPI.GarbageCollect()
	test.NoError(sub.t, err, test.Sprint("failed to gc"))

	// todo: should probably loop over the gc until the job is actually gone
}

type Option func(*Submission)

type Cleanup func()

func Submit(t *testing.T, filename string, opts ...Option) (*Submission, Cleanup) {
	sub := initialize(t, filename)

	for _, opt := range opts {
		opt(sub)
	}

	sub.setClient() // setup base api clients
	sub.run()       // submit job and wait on deployment
	sub.waits()     // wait on batch/sysbatch allocations

	return sub, sub.cleanup
}

func Namespace(name string) Option {
	return func(sub *Submission) {
		sub.inNamespace = name
	}
}
func AuthToken(token string) Option {
	return func(sub *Submission) {
		sub.authToken = token
	}
}

var (
	idRe = regexp.MustCompile(`(?m)^job "(.*)" \{`)
)

func (sub *Submission) run() {
	if !sub.noRandomJobID {
		sub.jobID = fmt.Sprintf("%s-%03d", sub.origJobID, rand.Int()%1000)
		sub.jobSpec = idRe.ReplaceAllString(sub.jobSpec, fmt.Sprintf("job %q {", sub.jobID))
	}

	parseConfig := &jobspec2.ParseConfig{
		// Path
		Body:    []byte(sub.jobSpec),
		AllowFS: true,
		ArgVars: sub.vars.Slice(),
		// VarFiles
		// VarContent
		// Envs
		// Strict
	}

	job, err := jobspec2.ParseWithConfig(parseConfig)
	must.NoError(sub.t, err, must.Sprint("failed to parse job"))
	must.NotNil(sub.t, job)

	if job.Type == nil {
		job.Type = pointer.Of("service")
	}

	writeOpts := &nomadapi.WriteOptions{
		Namespace: sub.inNamespace,
		AuthToken: sub.authToken,
	}

	jobsAPI := sub.nomadClient.Jobs()
	sub.logf("register (%s) job: %q", *job.Type, sub.jobID)
	regResp, _, err := jobsAPI.Register(job, writeOpts)
	must.NoError(sub.t, err)
	evalID := regResp.EvalID

	queryOpts := &nomadapi.QueryOptions{
		Namespace: sub.inNamespace,
		AuthToken: sub.authToken,
	}

	// setup a context with our submission timeout
	ctx, cancel := context.WithTimeout(context.Background(), sub.timeout)
	defer cancel()

	// we need to go through evals until we find the deployment
	evalAPI := sub.nomadClient.Evaluations()

	// start eval lookup loop
	var deploymentID string
EVAL:
	for {
		// check if we have passed timeout expiration
		select {
		case <-ctx.Done():
			must.Unreachable(sub.t, must.Sprint("timeout reached waiting for eval"))
		default:
		}

		eval, _, err := evalAPI.Info(evalID, queryOpts)
		must.NoError(sub.t, err)

		sub.logf("checking eval: %s, status: %s", evalID, eval.Status)

		switch eval.Status {

		case nomadapi.EvalStatusComplete:
			deploymentID = eval.DeploymentID
			break EVAL
		case nomadapi.EvalStatusFailed:
			must.Unreachable(sub.t, must.Sprint("eval failed"))
		case nomadapi.EvalStatusCancelled:
			must.Unreachable(sub.t, must.Sprint("eval cancelled"))
		default:
			time.Sleep(1 * time.Second)
		}

		nextEvalID := eval.NextEval
		if nextEvalID != "" {
			evalID = nextEvalID
			continue
		}
	}

	switch *job.Type {
	case "service":
		// need to monitor the deployment until it is complete
		depAPI := sub.nomadClient.Deployments()
	DEPLOY:
		for {

			// check if we have passed timeout expiration
			select {
			case <-ctx.Done():
				must.Unreachable(sub.t, must.Sprint("timeout reached waiting for deployment"))
			default:
			}

			dep, _, err := depAPI.Info(deploymentID, queryOpts)
			must.NoError(sub.t, err)

			sub.logf("checking deployment: %s, status: %s", dep.ID, dep.Status)

			switch dep.Status {
			case nomadapi.DeploymentStatusBlocked:
				must.Unreachable(sub.t, must.Sprint("deployment is blocked"))
			case nomadapi.DeploymentStatusCancelled:
				must.Unreachable(sub.t, must.Sprint("deployment is cancelled"))
			case nomadapi.DeploymentStatusFailed:
				must.Unreachable(sub.t, must.Sprint("deployment is failed"))
			case nomadapi.DeploymentStatusPaused:
				must.Unreachable(sub.t, must.Sprint("deployment is paused"))
			case nomadapi.DeploymentStatusPending:
				break
			case nomadapi.DeploymentStatusRunning:
				break
			case nomadapi.DeploymentStatusSuccessful:
				sub.logf("deployment %s was a success", dep.ID)
				break DEPLOY
			case nomadapi.DeploymentStatusUnblocking:
				must.Unreachable(sub.t, must.Sprint("deployment is unblocking"))
			default:
				break
			}
			time.Sleep(1 * time.Second)
		}
	// todo: more job types
	default:
	}

}

func (sub *Submission) waitAlloc(group, id string) {
	queryOpts := sub.queryOptions()
	allocAPI := sub.nomadClient.Allocations()

	// Set up a context with our submission timeout.
	ctx, cancel := context.WithTimeout(context.Background(), sub.timeout)
	defer cancel()

ALLOCATION:
	for {

		// Check if we have passed timeout expiration.
		select {
		case <-ctx.Done():
			must.Unreachable(sub.t, must.Sprint("timeout reached waiting for alloc"))
		default:
		}

		latest, _, err := allocAPI.Info(id, queryOpts)
		must.NoError(sub.t, err)

		status := latest.ClientStatus
		sub.logf("wait for %q allocation %s, status: %s", group, id, status)
		switch status {
		case nomadapi.AllocClientStatusLost:
			must.Unreachable(sub.t, must.Sprintf("group %q allocation %s lost", group, id))
		case nomadapi.AllocClientStatusFailed:
			must.Unreachable(sub.t, must.Sprintf("group %q allocation %s failed", group, id))
		case nomadapi.AllocClientStatusPending:
			break
		case nomadapi.AllocClientStatusRunning:
			break
		case nomadapi.AllocClientStatusComplete:
			break ALLOCATION
		}

		time.Sleep(1 * time.Second)
	}
}

func (sub *Submission) waits() {
	queryOpts := sub.queryOptions()
	jobsAPI := sub.nomadClient.Jobs()
	allocations, _, err := jobsAPI.Allocations(sub.jobID, false, queryOpts)
	must.NoError(sub.t, err)

	// for each alloc, if this is an alloc we want to wait on, wait on it
	for _, alloc := range allocations {
		id := alloc.ID
		group := alloc.TaskGroup
		if sub.waitComplete.Contains(group) {
			sub.waitAlloc(group, id)
		}
	}
}

func (sub *Submission) setClient() {
	nomadClient, nomadErr := nomadapi.NewClient(nomadapi.DefaultConfig())
	must.NoError(sub.t, nomadErr, must.Sprint("failed to create nomad api client"))
	sub.nomadClient = nomadClient
}

func initialize(t *testing.T, filename string) *Submission {
	b, err := os.ReadFile(filename)
	must.NoError(t, err, must.Sprintf("failed to read job file %q", filename))

	job := string(b)
	jobID := idRe.FindStringSubmatch(job)[1]
	must.NotEq(t, "", jobID, must.Sprintf("could not find job id in %q", filename))

	return &Submission{
		t:            t,
		jobSpec:      job,
		jobID:        jobID,
		origJobID:    jobID,
		timeout:      20 * time.Second,
		vars:         set.New[string](0),
		waitComplete: set.New[string](0),
	}
}

func DisableRandomJobID() Option {
	return func(sub *Submission) {
		sub.noRandomJobID = true
	}
}

func DisableCleanup() Option {
	return func(sub *Submission) {
		sub.noCleanup = true
	}
}

func Timeout(timeout time.Duration) Option {
	return func(c *Submission) {
		c.timeout = timeout
	}
}

// Verbose will turn on verbose logging.
func Verbose(on bool) Option {
	return func(c *Submission) {
		c.verbose = on
	}
}

// Set an HCL variable.
func Var(key, value string) Option {
	return func(sub *Submission) {
		sub.vars.Insert(fmt.Sprintf("%s=%s", key, value))
	}
}

// WaitComplete will wait until all allocations of the given group are
// in the "complete" state (or timeout, or terminal with another status).
func WaitComplete(group string) Option {
	return func(sub *Submission) {
		sub.waitComplete.Insert(group)
	}
}

// SkipEvalComplete will skip waiting for the evaluation(s) to be complete.
//
// Implies SkipDeploymentHealthy.
func SkipEvalComplete() Option {
	panic("not yet implemented")
}

// SkipDeploymentHealthy will skip waiting for the deployment to become
// healthy.
func SkipDeploymentHealthy() Option {
	panic("not yet implemented")
}
