package namespaces

import (
	"fmt"
	"os"
	"strings"

	e2e "github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
)

type NamespacesE2ETest struct {
	framework.TC
	namespaceIDs     []string
	namespacedJobIDs [][2]string // [(ns, jobID)]
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "Namespaces",
		CanRunLocal: true,
		Consul:      true,
		Cases: []framework.TestCase{
			new(NamespacesE2ETest),
		},
	})

}

func (tc *NamespacesE2ETest) BeforeAll(f *framework.F) {
	e2e.WaitForLeader(f.T(), tc.Nomad())
	e2e.WaitForNodesReady(f.T(), tc.Nomad(), 1)
}

func (tc *NamespacesE2ETest) AfterEach(f *framework.F) {
	if os.Getenv("NOMAD_TEST_SKIPCLEANUP") == "1" {
		return
	}

	for _, pair := range tc.namespacedJobIDs {
		ns := pair[0]
		jobID := pair[1]
		if ns != "" {
			err := e2e.StopJob(jobID, "-purge", "-namespace", ns)
			f.Assert().NoError(err)
		} else {
			err := e2e.StopJob(jobID, "-purge")
			f.Assert().NoError(err)
		}
	}
	tc.namespacedJobIDs = [][2]string{}

	for _, ns := range tc.namespaceIDs {
		_, err := e2e.Command("nomad", "namespace", "delete", ns)
		f.Assert().NoError(err)
	}
	tc.namespaceIDs = []string{}

	_, err := e2e.Command("nomad", "system", "gc")
	f.Assert().NoError(err)
}

// TestNamespacesFiltering exercises the -namespace flag on various commands
// to ensure that they are properly isolated
func (tc *NamespacesE2ETest) TestNamespacesFiltering(f *framework.F) {

	_, err := e2e.Command("nomad", "namespace", "apply",
		"-description", "namespace A", "NamespaceA")
	f.NoError(err, "could not create namespace")
	tc.namespaceIDs = append(tc.namespaceIDs, "NamespaceA")

	_, err = e2e.Command("nomad", "namespace", "apply",
		"-description", "namespace B", "NamespaceB")
	f.NoError(err, "could not create namespace")
	tc.namespaceIDs = append(tc.namespaceIDs, "NamespaceB")

	run := func(jobspec, ns string) string {
		jobID := "test-namespace-" + uuid.Generate()[0:8]
		f.NoError(e2e.Register(jobID, jobspec))
		tc.namespacedJobIDs = append(tc.namespacedJobIDs, [2]string{ns, jobID})
		expected := []string{"running"}
		f.NoError(e2e.WaitForAllocStatusExpected(jobID, ns, expected), "job should be running")
		return jobID
	}

	jobA := run("namespaces/input/namespace_a.nomad", "NamespaceA")
	jobB := run("namespaces/input/namespace_b.nomad", "NamespaceB")
	jobDefault := run("namespaces/input/namespace_default.nomad", "")

	// exercise 'nomad job status' filtering
	parse := func(out string) []map[string]string {
		rows, err := e2e.ParseColumns(out)
		f.NoError(err, "failed to parse job status output: %v", out)

		result := make([]map[string]string, 0, len(rows))
		for _, row := range rows {
			jobID := row["Job ID"]
			if jobID == "" {
				jobID = row["ID"]
			}
			if strings.HasPrefix(jobID, "test-namespace-") {
				result = append(result, row)
			}
		}
		return result
	}

	out, err := e2e.Command("nomad", "job", "status", "-namespace", "NamespaceA")
	f.NoError(err, "'nomad job status -namespace NamespaceA' failed")
	rows := parse(out)
	f.Len(rows, 1)
	f.Equal(jobA, rows[0]["ID"])

	out, err = e2e.Command("nomad", "job", "status", "-namespace", "NamespaceB")
	f.NoError(err, "'nomad job status -namespace NamespaceB' failed")
	rows = parse(out)
	f.Len(rows, 1)
	f.Equal(jobB, rows[0]["ID"])

	out, err = e2e.Command("nomad", "job", "status", "-namespace", "*")
	f.NoError(err, "'nomad job status -namespace *' failed")
	rows = parse(out)
	f.Equal(3, len(rows))

	out, err = e2e.Command("nomad", "job", "status")
	f.NoError(err, "'nomad job status' failed")
	rows = parse(out)
	f.Len(rows, 1)
	f.Equal(jobDefault, rows[0]["ID"])

	// exercise 'nomad status' filtering

	out, err = e2e.Command("nomad", "status", "-namespace", "NamespaceA")
	f.NoError(err, "'nomad job status -namespace NamespaceA' failed")
	rows = parse(out)
	f.Len(rows, 1)
	f.Equal(jobA, rows[0]["ID"])

	out, err = e2e.Command("nomad", "status", "-namespace", "NamespaceB")
	f.NoError(err, "'nomad job status -namespace NamespaceB' failed")
	rows = parse(out)
	f.Len(rows, 1)
	f.Equal(jobB, rows[0]["ID"])

	out, err = e2e.Command("nomad", "status", "-namespace", "*")
	f.NoError(err, "'nomad job status -namespace *' failed")
	rows = parse(out)
	f.Equal(3, len(rows))

	out, err = e2e.Command("nomad", "status")
	f.NoError(err, "'nomad status' failed")
	rows = parse(out)
	f.Len(rows, 1)
	f.Equal(jobDefault, rows[0]["ID"])

	// exercise 'nomad deployment list' filtering
	// note: '-namespace *' is only supported for job and alloc subcommands

	out, err = e2e.Command("nomad", "deployment", "list", "-namespace", "NamespaceA")
	f.NoError(err, "'nomad job status -namespace NamespaceA' failed")
	rows = parse(out)
	f.Len(rows, 1)
	f.Equal(jobA, rows[0]["Job ID"])

	out, err = e2e.Command("nomad", "deployment", "list", "-namespace", "NamespaceB")
	f.NoError(err, "'nomad job status -namespace NamespaceB' failed")
	rows = parse(out)
	f.Equal(len(rows), 1)
	f.Equal(jobB, rows[0]["Job ID"])

	out, err = e2e.Command("nomad", "deployment", "list")
	f.NoError(err, "'nomad deployment list' failed")
	rows = parse(out)
	f.Len(rows, 1)
	f.Equal(jobDefault, rows[0]["Job ID"])

	out, err = e2e.Command("nomad", "job", "stop", jobA)
	f.Equal(fmt.Sprintf("No job(s) with prefix or id %q found\n", jobA), out)
	f.Error(err, "exit status 1")

	err = e2e.StopJob(jobA, "-namespace", "NamespaceA")
	f.NoError(err, "could not stop job in namespace")
}
