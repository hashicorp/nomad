// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package namespaces

import (
	"fmt"
	"strings"
	"testing"

	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
)

// TestNamespacesFiltering exercises the -namespace flag on various commands to
// ensure that they are properly isolated
func TestNamespacesFiltering(t *testing.T) {

	namespaceIDs := []string{}
	namespacedJobIDs := [][2]string{} // [(ns, jobID)]

	t.Cleanup(func() {
		for _, nsJobID := range namespacedJobIDs {
			err := e2eutil.StopJob(nsJobID[1], "-purge", "-detach", "-namespace", nsJobID[0])
			test.NoError(t, err)
		}

		for _, namespaceID := range namespaceIDs {
			_, err := e2eutil.Command("nomad", "namespace", "delete", namespaceID)
			test.NoError(t, err)
		}

		_, err := e2eutil.Command("nomad", "system", "gc")
		test.NoError(t, err)
	})

	_, err := e2eutil.Command("nomad", "namespace", "apply",
		"-description", "namespace A", "NamespaceA")
	require.NoError(t, err, "could not create namespace")
	namespaceIDs = append(namespaceIDs, "NamespaceA")

	_, err = e2eutil.Command("nomad", "namespace", "apply",
		"-description", "namespace B", "NamespaceB")
	require.NoError(t, err, "could not create namespace")
	namespaceIDs = append(namespaceIDs, "NamespaceB")

	run := func(jobspec, ns string) string {
		jobID := "test-namespace-" + uuid.Generate()[0:8]
		must.NoError(t, e2eutil.Register(jobID, jobspec))
		namespacedJobIDs = append(namespacedJobIDs, [2]string{ns, jobID})
		expected := []string{"running"}
		require.NoError(t, e2eutil.WaitForAllocStatusExpected(jobID, ns, expected), "job should be running")
		return jobID
	}

	jobA := run("./input/namespace_a.nomad", "NamespaceA")
	jobB := run("./input/namespace_b.nomad", "NamespaceB")
	jobDefault := run("./input/namespace_default.nomad", "")

	// exercise 'nomad job status' filtering
	parse := func(out string) []map[string]string {
		rows, err := e2eutil.ParseColumns(out)
		require.NoError(t, err, "failed to parse job status output: %v", out)

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

	out, err := e2eutil.Command("nomad", "job", "status", "-namespace", "NamespaceA")
	require.NoError(t, err, "'nomad job status -namespace NamespaceA' failed")
	rows := parse(out)
	must.Len(t, 1, rows)
	must.Eq(t, rows[0]["ID"], jobA)

	out, err = e2eutil.Command("nomad", "job", "status", "-namespace", "NamespaceB")
	require.NoError(t, err, "'nomad job status -namespace NamespaceB' failed")
	rows = parse(out)
	must.Len(t, 1, rows)
	must.Eq(t, rows[0]["ID"], jobB)

	out, err = e2eutil.Command("nomad", "job", "status", "-namespace", "*")
	require.NoError(t, err, "'nomad job status -namespace *' failed")
	rows = parse(out)
	must.Len(t, 3, rows)

	out, err = e2eutil.Command("nomad", "job", "status")
	require.NoError(t, err, "'nomad job status' failed")
	rows = parse(out)
	must.Len(t, 1, rows)
	must.Eq(t, rows[0]["ID"], jobDefault)

	// exercise 'nomad status' filtering

	out, err = e2eutil.Command("nomad", "status", "-namespace", "NamespaceA")
	require.NoError(t, err, "'nomad job status -namespace NamespaceA' failed")
	rows = parse(out)
	must.Len(t, 1, rows)
	must.Eq(t, rows[0]["ID"], jobA)

	out, err = e2eutil.Command("nomad", "status", "-namespace", "NamespaceB")
	require.NoError(t, err, "'nomad job status -namespace NamespaceB' failed")
	rows = parse(out)
	must.Len(t, 1, rows)
	must.Eq(t, rows[0]["ID"], jobB)

	out, err = e2eutil.Command("nomad", "status", "-namespace", "*")
	require.NoError(t, err, "'nomad job status -namespace *' failed")
	rows = parse(out)
	must.Len(t, 3, rows)

	out, err = e2eutil.Command("nomad", "status")
	require.NoError(t, err, "'nomad status' failed")
	rows = parse(out)
	must.Len(t, 1, rows)
	must.Eq(t, rows[0]["ID"], jobDefault)

	// exercise 'nomad deployment list' filtering
	// note: '-namespace *' is only supported for job and alloc subcommands

	out, err = e2eutil.Command("nomad", "deployment", "list", "-namespace", "NamespaceA")
	require.NoError(t, err, "'nomad job status -namespace NamespaceA' failed")
	rows = parse(out)
	must.Len(t, 1, rows)
	must.Eq(t, rows[0]["Job ID"], jobA)

	out, err = e2eutil.Command("nomad", "deployment", "list", "-namespace", "NamespaceB")
	require.NoError(t, err, "'nomad job status -namespace NamespaceB' failed")
	rows = parse(out)
	must.Eq(t, len(rows), 1)
	must.Eq(t, rows[0]["Job ID"], jobB)

	out, err = e2eutil.Command("nomad", "deployment", "list")
	require.NoError(t, err, "'nomad deployment list' failed")
	rows = parse(out)
	must.Len(t, 1, rows)
	must.Eq(t, rows[0]["Job ID"], jobDefault)

	out, err = e2eutil.Command("nomad", "job", "stop", jobA)
	must.Eq(t, fmt.Sprintf("No job(s) with prefix or ID %q found\n", jobA), out)
	must.StrContains(t, err.Error(), "exit status 1")

	err = e2eutil.StopJob(jobA, "-namespace", "NamespaceA")
	require.NoError(t, err, "could not stop job in namespace")
}
