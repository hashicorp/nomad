package consul

import (
	"fmt"
	"reflect"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/jobspec"
)

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "consul",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(CanaryTagsCase),
		},
	})
}

type CanaryTagsCase struct {
	framework.TC
}

func (c *CanaryTagsCase) AfterEach(f *framework.F) {
	jobID, ok := f.Value("jobID").(string)
	if !ok {
		return
	}
	c.Nomad().Jobs().Deregister(jobID, true, nil)
	c.Nomad().System().GarbageCollect()
}

func (c *CanaryTagsCase) TestCanaryTags(f *framework.F) {
	allocations := c.Nomad().Allocations()
	deployments := c.Nomad().Deployments()
	jobs := c.Nomad().Jobs()
	job, err := jobspec.ParseFile("consul/input/canary_tags.hcl")
	f.NoError(err)
	job.ID = helper.StringToPtr(*job.ID + uuid.Generate()[22:])
	f.Set("jobID", *job.ID)
	resp, _, err := jobs.Register(job, nil)
	f.NoError(err)
	f.NotEmpty(resp.EvalID)
	f.NotNil(c.Consul())

	// Eventually be running and healthy
	f.Eventually(func() error {
		deploys, _, err := jobs.Deployments(*job.ID, nil)
		f.NoError(err)
		healthyDeploys := make([]string, 0, len(deploys))
		for _, d := range deploys {
			if d.Status == "successful" {
				healthyDeploys = append(healthyDeploys, d.ID)
			}
		}
		if len(healthyDeploys) != 1 {
			return fmt.Errorf("should have 1 healthy deployment")
		}
		return nil
	}, 5*time.Second, 20*time.Millisecond)

	// Start a deployment
	job.Meta = map[string]string{"version": "2"}
	resp, _, err = jobs.Register(job, nil)
	f.NoError(err)
	f.NotEmpty(resp.EvalID)

	// Eventually have a canary
	var deploys []*api.Deployment
	f.Eventually(func() error {
		deploys, _, err = jobs.Deployments(*job.ID, nil)
		f.NoError(err)
		if len(deploys) != 2 {
			return fmt.Errorf("should have 2 deploys, got %d", len(deploys))
		}
		return nil
	}, 2*time.Second, 20*time.Millisecond)

	var deploy *api.Deployment
	f.Eventually(func() error {
		deploy, _, err = deployments.Info(deploys[0].ID, nil)
		f.NoError(err)
		placedCanaries := len(deploy.TaskGroups["consul_canary_test"].PlacedCanaries)
		if placedCanaries != 1 {
			return fmt.Errorf("should have 1 placed canaries, got %d", placedCanaries)
		}
		return nil
	}, 2*time.Second, 20*time.Millisecond)

	f.Eventually(func() error {
		allocID := deploy.TaskGroups["consul_canary_test"].PlacedCanaries[0]
		alloc, _, err := allocations.Info(allocID, nil)
		f.NoError(err)
		if alloc.DeploymentStatus != nil && alloc.DeploymentStatus.Healthy != nil && *alloc.DeploymentStatus.Healthy {
			return nil
		}
		return fmt.Errorf("expected canary to be healthy")
	}, 3*time.Second, 20*time.Millisecond)

	// Check Consul for canary tags
	f.Eventually(func() error {
		services, err := c.Consul().Agent().Services()
		f.NoError(err)
		for _, v := range services {
			if v.Service == "canarytest" {
				if reflect.DeepEqual(v.Tags, []string{"foo", "canary"}) {
					return nil
				}
				return fmt.Errorf("expected tags ['foo', 'canary'], got %v", v.Tags)
			}
		}
		return fmt.Errorf("expected 'canarytest' service but not found")
	}, 2*time.Second, 20*time.Millisecond)

	// Manually promote
	promoteResp, _, err := deployments.PromoteAll(deploys[0].ID, nil)
	f.NoError(err)
	f.NotEmpty(promoteResp.EvalID)

	// Eventually canary is removed
	f.Eventually(func() error {
		allocID := deploy.TaskGroups["consul_canary_test"].PlacedCanaries[0]
		alloc, _, err := allocations.Info(allocID, nil)
		f.NoError(err)
		if alloc.DeploymentStatus.Canary {
			return fmt.Errorf("canary should be false")
		}
		return nil
	}, 2*time.Second, 20*time.Millisecond)

	// Check Consul canary tags were removed
	f.Eventually(func() error {
		services, err := c.Consul().Agent().Services()
		f.NoError(err)
		for _, v := range services {
			if v.Service == "canarytest" {
				if reflect.DeepEqual(v.Tags, []string{"foo", "bar"}) {
					return nil
				}
				return fmt.Errorf("expected tags ['foo', 'bar'], got %v", v.Tags)
			}
		}
		return fmt.Errorf("expected 'canarytest' service but not found")

	}, 2*time.Second, 20*time.Millisecond)

}
