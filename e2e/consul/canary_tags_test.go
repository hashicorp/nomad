package consul_test

import (
	"flag"
	"testing"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/jobspec"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var integration = flag.Bool("integration", false, "run integration tests")

func TestConsul(t *testing.T) {
	if !*integration {
		t.Skip("skipping test in non-integration mode.")
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "Consul Canary Tags Test")
}

var _ = Describe("Consul Canary Tags Test", func() {

	var (
		agent       *consulapi.Agent
		allocations *api.Allocations
		deployments *api.Deployments
		jobs        *api.Jobs
		system      *api.System
		job         *api.Job
		specFile    string
	)

	BeforeSuite(func() {
		consulConf := consulapi.DefaultConfig()
		consulClient, err := consulapi.NewClient(consulConf)
		Expect(err).ShouldNot(HaveOccurred())
		agent = consulClient.Agent()

		conf := api.DefaultConfig()
		client, err := api.NewClient(conf)
		Expect(err).ShouldNot(HaveOccurred())
		allocations = client.Allocations()
		deployments = client.Deployments()
		jobs = client.Jobs()
		system = client.System()
	})

	JustBeforeEach(func() {
		var err error
		job, err = jobspec.ParseFile(specFile)
		Expect(err).ShouldNot(HaveOccurred())
		job.ID = helper.StringToPtr(*job.ID + uuid.Generate()[22:])
		resp, _, err := jobs.Register(job, nil)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(resp.EvalID).ShouldNot(BeEmpty())
	})

	AfterEach(func() {
		jobs.Deregister(*job.ID, true, nil)
		system.GarbageCollect()
	})

	Describe("Consul Canary Tags Test", func() {
		Context("Canary Tags", func() {
			BeforeEach(func() {
				specFile = "input/canary_tags.hcl"
			})

			It("Should set and unset canary tags", func() {

				// Eventually be running and healthy
				Eventually(func() []string {
					deploys, _, err := jobs.Deployments(*job.ID, nil)
					Expect(err).ShouldNot(HaveOccurred())
					healthyDeploys := make([]string, 0, len(deploys))
					for _, d := range deploys {
						if d.Status == "successful" {
							healthyDeploys = append(healthyDeploys, d.ID)
						}
					}
					return healthyDeploys
				}, 5*time.Second, 20*time.Millisecond).Should(HaveLen(1))

				// Start a deployment
				job.Meta = map[string]string{"version": "2"}
				resp, _, err := jobs.Register(job, nil)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(resp.EvalID).ShouldNot(BeEmpty())

				// Eventually have a canary
				var deploys []*api.Deployment
				Eventually(func() []*api.Deployment {
					deploys, _, err = jobs.Deployments(*job.ID, nil)
					Expect(err).ShouldNot(HaveOccurred())
					return deploys
				}, 2*time.Second, 20*time.Millisecond).Should(HaveLen(2))

				var deploy *api.Deployment
				Eventually(func() []string {
					deploy, _, err = deployments.Info(deploys[0].ID, nil)
					Expect(err).ShouldNot(HaveOccurred())
					return deploy.TaskGroups["consul_canary_test"].PlacedCanaries
				}, 2*time.Second, 20*time.Millisecond).Should(HaveLen(1))

				Eventually(func() bool {
					allocID := deploy.TaskGroups["consul_canary_test"].PlacedCanaries[0]
					alloc, _, err := allocations.Info(allocID, nil)
					Expect(err).ShouldNot(HaveOccurred())
					return alloc.DeploymentStatus != nil && alloc.DeploymentStatus.Healthy != nil && *alloc.DeploymentStatus.Healthy
				}, 3*time.Second, 20*time.Millisecond).Should(BeTrue())

				// Check Consul for canary tags
				Eventually(func() []string {
					services, err := agent.Services()
					Expect(err).ShouldNot(HaveOccurred())
					for _, v := range services {
						if v.Service == "canarytest" {
							return v.Tags
						}
					}
					return nil
				}, 2*time.Second, 20*time.Millisecond).Should(
					Equal([]string{"foo", "canary"}))

				// Manually promote
				{
					resp, _, err := deployments.PromoteAll(deploys[0].ID, nil)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(resp.EvalID).ShouldNot(BeEmpty())
				}

				// Eventually canary is removed
				Eventually(func() bool {
					allocID := deploy.TaskGroups["consul_canary_test"].PlacedCanaries[0]
					alloc, _, err := allocations.Info(allocID, nil)
					Expect(err).ShouldNot(HaveOccurred())
					return alloc.DeploymentStatus.Canary
				}, 2*time.Second, 20*time.Millisecond).Should(BeFalse())

				// Check Consul canary tags were removed
				Eventually(func() []string {
					services, err := agent.Services()
					Expect(err).ShouldNot(HaveOccurred())
					for _, v := range services {
						if v.Service == "canarytest" {
							return v.Tags
						}
					}
					return nil
				}, 2*time.Second, 20*time.Millisecond).Should(
					Equal([]string{"foo", "bar"}))
			})
		})
	})
})
