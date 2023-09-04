package api

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

func TestDeployments_List(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	deployments := c.Deployments()
	jobs := c.Jobs()

	// Create a job of type service
	job := testServiceJob()

	// Initially there should be no deployment
	resp0, _, err := deployments.List(nil)
	must.NoError(t, err)
	must.Len(t, 0, resp0)

	// Register
	resp1, wm, err := jobs.Register(job, nil)
	must.NoError(t, err)
	must.NotNil(t, resp1)
	must.UUIDv4(t, resp1.EvalID)
	assertWriteMeta(t, wm)

	// Query the jobs back out again
	resp2, qm, err := jobs.List(nil)
	assertQueryMeta(t, qm)
	must.NoError(t, err)
	must.Len(t, 1, resp2)

	f := func() error {
		// List the deployment
		resp3, _, err := deployments.List(nil)
		if err != nil {
			return err
		}
		if len(resp3) != 1 {
			return fmt.Errorf(fmt.Sprintf("expected 1 deployment, found %v", len(resp3)))
		}
		return nil
	}
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(f),
		wait.Timeout(3*time.Second),
		wait.Gap(100*time.Millisecond),
	))
}

func TestDeployments_PrefixList(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	deployments := c.Deployments()
	jobs := c.Jobs()

	// Initially there should be no deployment
	resp, _, err := deployments.PrefixList("b1")
	must.NoError(t, err)
	must.Len(t, 0, resp)

	// Create a job of type service
	job := testServiceJob()

	// Register the job
	resp2, wm, err := jobs.Register(job, nil)
	must.NoError(t, err)
	must.NotNil(t, resp2)
	must.UUIDv4(t, resp2.EvalID)
	assertWriteMeta(t, wm)

	// Query the jobs back out again
	resp3, qm, err := jobs.List(nil)
	assertQueryMeta(t, qm)
	must.NoError(t, err)
	must.Len(t, 1, resp3)

	f := func() error {
		// List the deployment
		resp4, _, err := jobs.Deployments(resp3[0].ID, true, nil)
		if len(resp4) != 1 {
			return fmt.Errorf("expected 1 deployment, found %v", len(resp4))
		}

		// Prefix List
		resp5, _, err := deployments.PrefixList(resp4[0].ID)
		if err != nil {
			return err
		}
		if len(resp5) != 1 {
			return fmt.Errorf(fmt.Sprintf("expected 1 deployment, found %v", len(resp5)))
		}
		return nil
	}
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(f),
		wait.Timeout(3*time.Second),
		wait.Gap(100*time.Millisecond),
	))
}

func TestDeployments_Info(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	deployments := c.Deployments()
	jobs := c.Jobs()

	// Create a job of type service
	job := testServiceJob()

	// Register
	resp, wm, err := jobs.Register(job, nil)
	must.NoError(t, err)
	must.NotNil(t, resp)
	must.UUIDv4(t, resp.EvalID)
	assertWriteMeta(t, wm)

	// Query the jobs
	resp2, qm, err := jobs.List(nil)
	assertQueryMeta(t, qm)
	must.NoError(t, err)
	must.Len(t, 1, resp2)

	f := func() error {
		// Get the deploymentID for Info
		resp3, _, err := jobs.Deployments(resp2[0].ID, true, nil)
		if len(resp3) != 1 {
			return fmt.Errorf("expected 1 deployment, found %v", len(resp3))
		}

		// Get Info using deployment ID
		resp4, _, err := deployments.Info(resp3[0].ID, nil)
		if err != nil {
			return err
		}
		if resp4.JobID != resp2[0].ID {
			return fmt.Errorf(fmt.Sprintf("expected job id: %v, found %v", resp4.JobID, resp2[0].ID))
		}
		if resp4.Namespace != resp2[0].Namespace {
			return fmt.Errorf(fmt.Sprintf("expected deployment namespace: %v, found %v", resp4.Namespace, resp2[0].Namespace))
		}
		return nil
	}
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(f),
		wait.Timeout(3*time.Second),
		wait.Gap(100*time.Millisecond),
	))
}

func TestDeployments_Allocations(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	deployments := c.Deployments()
	jobs := c.Jobs()

	// Create a job of type service
	job := testServiceJob()

	// Register
	resp, wm, err := jobs.Register(job, nil)
	must.NoError(t, err)
	must.NotNil(t, resp)
	must.UUIDv4(t, resp.EvalID)
	assertWriteMeta(t, wm)

	// Query the jobs
	resp2, qm, err := jobs.List(nil)
	assertQueryMeta(t, qm)
	must.NoError(t, err)
	must.Len(t, 1, resp2)

	f := func() error {
		// Get the deploymentID for Allocations
		resp3, _, err := jobs.Deployments(resp2[0].ID, true, nil)
		if len(resp3) != 1 {
			return fmt.Errorf("expected 1 deployment, found %v", len(resp3))
		}

		// Query deployment list
		resp4, _, err := deployments.List(nil)
		if err != nil {
			return err
		}
		if len(resp4) != 1 {
			return fmt.Errorf("expected 1 deployment, found %v", len(resp4))
		}

		// Get Allocations
		resp5, _, err := deployments.Allocations(resp3[0].ID, nil)
		if err != nil {
			return err
		}
		if len(resp5) != 0 {
			return fmt.Errorf("expected 0 Allocations, found %v", len(resp5))
		}
		return nil
	}
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(f),
		wait.Timeout(3*time.Second),
		wait.Gap(100*time.Millisecond),
	))
}

func TestDeployments_Fail(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	deployments := c.Deployments()
	jobs := c.Jobs()

	// Create a job of type service
	job := testServiceJob()

	// Register
	resp, wm, err := jobs.Register(job, nil)
	must.NoError(t, err)
	must.NotNil(t, resp)
	must.UUIDv4(t, resp.EvalID)
	assertWriteMeta(t, wm)

	// Query the jobs
	resp2, qm, err := jobs.List(nil)
	assertQueryMeta(t, qm)
	must.NoError(t, err)
	must.Len(t, 1, resp2)

	f := func() error {
		// Get the deploymentID for Failing
		resp3, _, err := jobs.Deployments(resp2[0].ID, true, nil)
		if len(resp3) != 1 {
			return fmt.Errorf("expected 1 deployment, found %v", len(resp3))
		}

		// Fail Deployment
		_, _, err = deployments.Fail(resp3[0].ID, nil)
		if err != nil {
			return err
		}

		// Query Info to check the status
		resp4, _, err := deployments.Info(resp3[0].ID, nil)
		if err != nil {
			return err
		}
		if resp4.Status != "failed" {
			return fmt.Errorf("expected failed status, got %v", resp4.Status)
		}
		return nil
	}
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(f),
		wait.Timeout(3*time.Second),
		wait.Gap(100*time.Millisecond),
	))
}

func TestDeployments_Pause(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	deployments := c.Deployments()
	jobs := c.Jobs()

	// Create a job of type service
	job := testServiceJob()

	// Register
	resp, wm, err := jobs.Register(job, nil)
	must.NoError(t, err)
	must.NotNil(t, resp)
	must.UUIDv4(t, resp.EvalID)
	assertWriteMeta(t, wm)

	// Query the jobs
	resp2, qm, err := jobs.List(nil)
	assertQueryMeta(t, qm)
	must.NoError(t, err)
	must.Len(t, 1, resp2)

	f := func() error {
		// Get the deploymentID for pausing
		resp3, _, err := jobs.Deployments(resp2[0].ID, true, nil)
		if len(resp3) != 1 {
			return fmt.Errorf("expected 1 deployment, found %v", len(resp3))
		}

		// Pause Deployment
		_, _, err = deployments.Pause(resp3[0].ID, true, nil)
		if err != nil {
			return err
		}

		// Query Info to check the status
		resp4, _, err := deployments.Info(resp3[0].ID, nil)
		if err != nil {
			return err
		}
		if resp4.Status != "paused" {
			return fmt.Errorf("expected paused status, got %v", resp4.Status)
		}
		return nil
	}
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(f),
		wait.Timeout(3*time.Second),
		wait.Gap(100*time.Millisecond),
	))
}

func TestDeployments_Unpause(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	deployments := c.Deployments()
	jobs := c.Jobs()

	// Create a job of type service
	job := testServiceJob()

	// Register
	resp, wm, err := jobs.Register(job, nil)
	must.NoError(t, err)
	must.NotNil(t, resp)
	must.UUIDv4(t, resp.EvalID)
	assertWriteMeta(t, wm)

	// Query the jobs
	resp2, qm, err := jobs.List(nil)
	assertQueryMeta(t, qm)
	must.NoError(t, err)
	must.Len(t, 1, resp2)

	f := func() error {
		// Get the deploymentID for un-pausing
		resp3, _, err := jobs.Deployments(resp2[0].ID, true, nil)
		if len(resp3) != 1 {
			return fmt.Errorf("expected 1 deployment, found %v", len(resp3))
		}

		// Pause Deployment
		_, _, err = deployments.Pause(resp3[0].ID, true, nil)
		if err != nil {
			return err
		}

		// Query Info to check the status
		resp4, _, err := deployments.Info(resp3[0].ID, nil)
		if err != nil {
			return err
		}
		if resp4.Status != "paused" {
			return fmt.Errorf("expected paused status, got %v", resp4.Status)
		}

		// UnPause the deployment
		_, _, err = deployments.Pause(resp3[0].ID, false, nil)
		must.NoError(t, err)

		// Query Info again to check the status
		resp5, _, err := deployments.Info(resp3[0].ID, nil)
		if err != nil {
			return err
		}
		if resp5.Status != "running" {
			return fmt.Errorf("expected running status, got %v", resp5.Status)
		}
		return nil
	}
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(f),
		wait.Timeout(3*time.Second),
		wait.Gap(100*time.Millisecond),
	))
}
