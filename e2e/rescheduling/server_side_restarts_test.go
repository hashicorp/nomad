package rescheduling

import (
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/jobspec"
	_ "github.com/hashicorp/nomad/jobspec"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
			return ret
		}

		// allocStatusesRescheduled is a helper function that pulls
		// out client statuses only from rescheduled allocs
		allocStatusesRescheduled = func() []string {
			allocs, _, err := jobs.Allocations(*job.ID, false, nil)
			Expect(err).ShouldNot(HaveOccurred())
			var ret []string
			for _, a := range allocs {
				if a.RescheduleTracker != nil && len(a.RescheduleTracker.Events) > 0 {
					ret = append(ret, a.ClientStatus)
				}
			}
			return ret
		}
	)

	BeforeSuite(func() {
		conf := api.DefaultConfig()
		conf.Address = "http://localhost:4646"

		// Create client
		client, err := api.NewClient(conf)
		Expect(err).ShouldNot(HaveOccurred())
		jobs = client.Jobs()
		system = client.System()
	})

	JustBeforeEach(func() {
		job, err = jobspec.ParseFile(specFile)
		Expect(err).ShouldNot(HaveOccurred())

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

		Context("Reschedule attempts maxed out", func() {
			BeforeEach(func() {
				specFile = "input/rescheduling_fail.hcl"
			})
			// Expect 3 original plus 6 rescheduled allocs from 2 attempts
			var expected []string
			for i := 0; i < 9; i++ {
				expected = append(expected, "failed")
			}
			It("Should have all failed", func() {
				Eventually(allocStatuses, 5*time.Second, time.Second).ShouldNot(
					SatisfyAll(ContainElement("pending"),
						ContainElement("running")))
			})
		})

		Context("Reschedule attempts succeeded", func() {
			BeforeEach(func() {
				specFile = "input/reschedule_success.hcl"
			})
			It("Should have some running allocs", func() {
				Eventually(allocStatuses, 5*time.Second, time.Second).Should(
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
				It("Should have no rescheduled allocs", func() {
					job.TaskGroups[0].Tasks[0].Config["args"] = []string{"-c", "lol"}
					_, _, err := jobs.Register(job, nil)
					Expect(err).ShouldNot(HaveOccurred())
					Eventually(allocStatusesRescheduled, 2*time.Second, time.Second).Should(BeEmpty())
				})
			})

		})

		Context("Reschedule with canary", func() {
			BeforeEach(func() {
				specFile = "input/rescheduling_canary.hcl"
			})
			It("Should have all running allocs", func() {
				Eventually(allocStatuses, 3*time.Second, time.Second).Should(
					ConsistOf([]string{"running", "running", "running"}))
			})
			Context("Updating job to make allocs fail", func() {
				It("Should have no rescheduled allocs", func() {
					job.TaskGroups[0].Tasks[0].Config["args"] = []string{"-c", "lol"}
					_, _, err := jobs.Register(job, nil)
					Expect(err).ShouldNot(HaveOccurred())
					Eventually(allocStatusesRescheduled, 2*time.Second, time.Second).Should(BeEmpty())
				})
			})

		})

	})

})
