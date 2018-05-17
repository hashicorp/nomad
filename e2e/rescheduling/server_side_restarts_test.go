package rescheduling

import (
	"sort"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/jobspec"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
)

var _ = Describe("Server Side Restart Tests", func() {

	var (
		jobs     *api.Jobs
		system   *api.System
		job      *api.Job
		err      error
		specFile string

		// allocStatuses is a helper function that pulls
		// out client statuses from a slice of allocs
		allocStatuses = func() []string {
			allocs, _, err := jobs.Allocations(*job.ID, false, nil)
			Expect(err).ShouldNot(HaveOccurred())
			var ret []string
			for _, a := range allocs {
				ret = append(ret, a.ClientStatus)
			}
			sort.Strings(ret)
			return ret
		}

		// allocStatusesRescheduled is a helper function that pulls
		// out client statuses only from rescheduled allocs
		allocStatusesRescheduled = func() []string {
			allocs, _, err := jobs.Allocations(*job.ID, false, nil)
			Expect(err).ShouldNot(HaveOccurred())
			var ret []string
			for _, a := range allocs {
				if (a.RescheduleTracker != nil && len(a.RescheduleTracker.Events) > 0) || a.FollowupEvalID != "" {
					ret = append(ret, a.ClientStatus)
				}
			}
			return ret
		}

		// deploymentStatus is a helper function that returns deployment status of all deployments
		// sorted by time
		deploymentStatus = func() []string {
			deploys, _, err := jobs.Deployments(*job.ID, nil)
			Expect(err).ShouldNot(HaveOccurred())
			var ret []string
			sort.Slice(deploys, func(i, j int) bool {
				return deploys[i].CreateIndex < deploys[j].CreateIndex
			})
			for _, d := range deploys {
				ret = append(ret, d.Status)
			}
			return ret
		}
	)

	BeforeSuite(func() {
		conf := api.DefaultConfig()

		// Create client
		client, err := api.NewClient(conf)
		Expect(err).ShouldNot(HaveOccurred())
		jobs = client.Jobs()
		system = client.System()
	})

	JustBeforeEach(func() {
		job, err = jobspec.ParseFile(specFile)
		Expect(err).ShouldNot(HaveOccurred())
		job.ID = helper.StringToPtr(uuid.Generate())
		resp, _, err := jobs.Register(job, nil)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(resp.EvalID).ShouldNot(BeEmpty())

	})

	AfterEach(func() {
		//Deregister job
		jobs.Deregister(*job.ID, true, nil)
		system.GarbageCollect()
	})

	Describe("Reschedule Stanza Tests", func() {

		Context("No reschedule attempts", func() {
			BeforeEach(func() {
				specFile = "input/norescheduling.hcl"
			})

			It("Should have exactly three allocs and all failed", func() {
				Eventually(allocStatuses, 5*time.Second, time.Second).Should(ConsistOf([]string{"failed", "failed", "failed"}))
			})
		})

		Context("System jobs should never be rescheduled", func() {
			BeforeEach(func() {
				specFile = "input/rescheduling_system.hcl"
			})

			It("Should have exactly one failed alloc", func() {
				Eventually(allocStatuses, 10*time.Second, time.Second).Should(ConsistOf([]string{"failed"}))
			})
		})

		Context("Default Rescheduling", func() {
			BeforeEach(func() {
				specFile = "input/rescheduling_default.hcl"
			})
			It("Should have exactly three allocs and all failed after 5 secs", func() {
				Eventually(allocStatuses, 5*time.Second, time.Second).Should(ConsistOf([]string{"failed", "failed", "failed"}))
			})
			// wait until first exponential delay kicks in and rescheduling is attempted
			It("Should have exactly six allocs and all failed after 35 secs", func() {
				if !*slow {
					Skip("Skipping slow test")
				}
				Eventually(allocStatuses, 35*time.Second, time.Second).Should(ConsistOf([]string{"failed", "failed", "failed", "failed", "failed", "failed"}))
			})
		})

		Context("Reschedule attempts maxed out", func() {
			BeforeEach(func() {
				specFile = "input/rescheduling_fail.hcl"
			})
			It("Should have all failed", func() {
				Eventually(allocStatuses, 6*time.Second, time.Second).ShouldNot(
					SatisfyAll(ContainElement("pending"),
						ContainElement("running")))
			})
			Context("Updating job to change its version", func() {
				It("Should have running allocs now", func() {
					job.TaskGroups[0].Tasks[0].Config["args"] = []string{"-c", "sleep 15000"}
					_, _, err := jobs.Register(job, nil)
					Expect(err).ShouldNot(HaveOccurred())
					Eventually(allocStatuses, 5*time.Second, time.Second).Should(ContainElement("running"))
				})
			})
		})

		Context("Reschedule attempts succeeded", func() {
			BeforeEach(func() {
				specFile = "input/reschedule_success.hcl"
			})
			It("Should have some running allocs", func() {
				Eventually(allocStatuses, 6*time.Second, time.Second).Should(
					ContainElement("running"))
			})
		})

		Context("Reschedule with update stanza", func() {
			BeforeEach(func() {
				specFile = "input/rescheduling_update.hcl"
			})
			It("Should have all running allocs", func() {
				Eventually(allocStatuses, 3*time.Second, time.Second).Should(
					ConsistOf([]string{"running", "running", "running"}))
			})
			Context("Updating job to make allocs fail", func() {
				It("Should have rescheduled allocs until progress deadline", func() {
					job.TaskGroups[0].Tasks[0].Config["args"] = []string{"-c", "lol"}
					_, _, err := jobs.Register(job, nil)
					Expect(err).ShouldNot(HaveOccurred())
					Eventually(allocStatusesRescheduled, 5*time.Second, time.Second).ShouldNot(BeEmpty())
				})
			})

		})

		Context("Reschedule with canary", func() {
			BeforeEach(func() {
				specFile = "input/rescheduling_canary.hcl"
			})
			It("Should have running allocs and successful deployment", func() {
				Eventually(allocStatuses, 3*time.Second, time.Second).Should(
					ConsistOf([]string{"running", "running", "running"}))

				time.Sleep(2 * time.Second) //TODO(preetha) figure out why this wasn't working with ginkgo constructs
				Eventually(deploymentStatus(), 2*time.Second, time.Second).Should(
					ContainElement(structs.DeploymentStatusSuccessful))
			})

			Context("Updating job to make allocs fail", func() {
				It("Should have rescheduled allocs until progress deadline", func() {
					job.TaskGroups[0].Tasks[0].Config["args"] = []string{"-c", "lol"}
					_, _, err := jobs.Register(job, nil)
					Expect(err).ShouldNot(HaveOccurred())
					Eventually(allocStatusesRescheduled, 5*time.Second, time.Second).ShouldNot(BeEmpty())

					// Verify new deployment and its status
					// Deployment status should be running (because of progress deadline)
					time.Sleep(3 * time.Second) //TODO(preetha) figure out why this wasn't working with ginkgo constructs
					Eventually(deploymentStatus(), 2*time.Second, time.Second).Should(
						ContainElement(structs.DeploymentStatusRunning))
				})
			})

		})

		Context("Reschedule with canary, auto revert with short progress deadline ", func() {
			BeforeEach(func() {
				specFile = "input/rescheduling_canary_autorevert.hcl"
			})
			It("Should have running allocs and successful deployment", func() {
				Eventually(allocStatuses, 3*time.Second, time.Second).Should(
					ConsistOf([]string{"running", "running", "running"}))

				time.Sleep(2 * time.Second)
				Eventually(deploymentStatus(), 2*time.Second, time.Second).Should(
					ContainElement(structs.DeploymentStatusSuccessful))

				// Make an update that causes the job to fail
				job.TaskGroups[0].Tasks[0].Config["args"] = []string{"-c", "lol"}
				_, _, err := jobs.Register(job, nil)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(allocStatusesRescheduled, 2*time.Second, time.Second).Should(BeEmpty())

				// Wait for the revert
				Eventually(allocStatuses, 3*time.Second, time.Second).Should(
					ConsistOf([]string{"failed", "failed", "failed", "running", "running", "running"}))
				// Verify new deployment and its status
				// There should be one successful, one failed, and one more successful (after revert)
				time.Sleep(5 * time.Second) //TODO(preetha) figure out why this wasn't working with ginkgo constructs
				Eventually(deploymentStatus(), 5*time.Second, time.Second).Should(
					ConsistOf(structs.DeploymentStatusSuccessful, structs.DeploymentStatusFailed, structs.DeploymentStatusSuccessful))
			})

		})

		Context("Reschedule with max parallel/auto_revert false", func() {
			BeforeEach(func() {
				specFile = "input/rescheduling_maxp.hcl"
			})
			It("Should have running allocs and successful deployment", func() {
				Eventually(allocStatuses, 3*time.Second, time.Second).Should(
					ConsistOf([]string{"running", "running", "running"}))

				time.Sleep(2 * time.Second)
				Eventually(deploymentStatus(), 2*time.Second, time.Second).Should(
					ContainElement(structs.DeploymentStatusSuccessful))
			})

			Context("Updating job to make allocs fail", func() {
				It("Should have rescheduled allocs till progress deadline", func() {
					job.TaskGroups[0].Tasks[0].Config["args"] = []string{"-c", "lol"}
					_, _, err := jobs.Register(job, nil)
					Expect(err).ShouldNot(HaveOccurred())
					Eventually(allocStatusesRescheduled, 3*time.Second, time.Second).ShouldNot(BeEmpty())

					// Should have 1 failed from max_parallel
					Eventually(allocStatuses, 3*time.Second, time.Second).Should(
						ConsistOf([]string{"complete", "failed", "running", "running"}))

					// Verify new deployment and its status
					time.Sleep(2 * time.Second)
					Eventually(deploymentStatus(), 2*time.Second, time.Second).Should(
						ContainElement(structs.DeploymentStatusRunning))
				})
			})

		})

		Context("Reschedule with max parallel, auto revert true and short progress deadline", func() {
			BeforeEach(func() {
				specFile = "input/rescheduling_maxp_autorevert.hcl"
			})
			It("Should have running allocs and successful deployment", func() {
				Eventually(allocStatuses, 3*time.Second, time.Second).Should(
					ConsistOf([]string{"running", "running", "running"}))

				time.Sleep(4 * time.Second)
				Eventually(deploymentStatus(), 2*time.Second, time.Second).Should(
					ContainElement(structs.DeploymentStatusSuccessful))

				// Make an update that causes the job to fail
				job.TaskGroups[0].Tasks[0].Config["args"] = []string{"-c", "lol"}
				_, _, err := jobs.Register(job, nil)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(allocStatusesRescheduled, 2*time.Second, time.Second).Should(BeEmpty())

				// Wait for the revert
				Eventually(allocStatuses, 5*time.Second, time.Second).Should(
					ConsistOf([]string{"complete", "failed", "running", "running", "running"}))

				// Verify new deployment and its status
				// There should be one successful, one failed, and one more successful (after revert)
				time.Sleep(5 * time.Second)
				Eventually(deploymentStatus(), 2*time.Second, time.Second).Should(
					ConsistOf(structs.DeploymentStatusSuccessful, structs.DeploymentStatusFailed, structs.DeploymentStatusSuccessful))
			})

		})

	})

})
