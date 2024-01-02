// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package jobsubmissions

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/acl"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/shoenig/test/must"
)

func TestJobSubmissionAPI(t *testing.T) {
	nomad := e2eutil.NomadClient(t)

	e2eutil.WaitForLeader(t, nomad)
	e2eutil.WaitForNodesReady(t, nomad, 1)

	t.Run("testParseAPI", testParseAPI)
	t.Run("testRunCLIVarFlags", testRunCLIVarFlags)
	t.Run("testSubmissionACL", testSubmissionACL)
	t.Run("testMaxSize", testMaxSize)
	t.Run("testReversion", testReversion)
	t.Run("testVarFiles", testVarFiles)
}

func testParseAPI(t *testing.T) {
	nomad := e2eutil.NomadClient(t)

	jobID := "job-sub-parse-" + uuid.Short()
	jobIDs := []string{jobID}
	t.Cleanup(e2eutil.MaybeCleanupJobsAndGC(&jobIDs))

	spec, err := os.ReadFile("input/xyz.hcl")
	must.NoError(t, err)

	job, err := nomad.Jobs().ParseHCLOpts(&api.JobsParseRequest{
		JobHCL:       string(spec),
		HCLv1:        false,
		Variables:    "X=\"baz\" \n Y=50 \n Z=true \n",
		Canonicalize: true,
	})
	must.NoError(t, err)
	args := job.TaskGroups[0].Tasks[0].Config["args"]
	must.Eq(t, []any{"X baz, Y 50, Z true"}, args.([]any))
}

func testRunCLIVarFlags(t *testing.T) {
	nomad := e2eutil.NomadClient(t)

	jobID := "job-sub-cli-" + uuid.Short()
	jobIDs := []string{jobID}
	t.Cleanup(e2eutil.MaybeCleanupJobsAndGC(&jobIDs))

	// register job via cli with var arguments
	err := e2eutil.RegisterWithArgs(jobID, "input/xyz.hcl", "-var=X=foo", "-var=Y=42", "-var=Z=true")
	must.NoError(t, err)

	// find our alloc id
	allocID := e2eutil.SingleAllocID(t, jobID, "default", 0)

	// wait for alloc to complete
	_ = e2eutil.WaitForAllocStopped(t, nomad, allocID)

	// inspect alloc logs making sure our variables got set
	out, err := e2eutil.AllocLogs(allocID, "", e2eutil.LogsStdOut)
	must.NoError(t, err)
	must.Eq(t, "X foo, Y 42, Z true\n", out)

	// check the submission api
	sub, _, err := nomad.Jobs().Submission(jobID, 0, &api.QueryOptions{
		Region:    "global",
		Namespace: "default",
	})
	must.NoError(t, err)
	must.Eq(t, "hcl2", sub.Format)
	must.NotEq(t, "", sub.Source)
	must.Eq(t, map[string]string{"X": "foo", "Y": "42", "Z": "true"}, sub.VariableFlags)
	must.Eq(t, "", sub.Variables)

	// register job again with different var arguments
	err = e2eutil.RegisterWithArgs(jobID, "input/xyz.hcl", "-var=X=bar", "-var=Y=99", "-var=Z=false")
	must.NoError(t, err)

	// find our alloc id
	allocID = e2eutil.SingleAllocID(t, jobID, "default", 1)

	// wait for alloc to complete
	_ = e2eutil.WaitForAllocStopped(t, nomad, allocID)

	// inspect alloc logs making sure our new variables got set
	out, err = e2eutil.AllocLogs(allocID, "", e2eutil.LogsStdOut)
	must.NoError(t, err)
	must.Eq(t, "X bar, Y 99, Z false\n", out)

	// check the submission api for v1
	sub, _, err = nomad.Jobs().Submission(jobID, 1, &api.QueryOptions{
		Region:    "global",
		Namespace: "default",
	})
	must.NoError(t, err)
	must.Eq(t, "hcl2", sub.Format)
	must.NotEq(t, "", sub.Source)
	must.Eq(t, map[string]string{"X": "bar", "Y": "99", "Z": "false"}, sub.VariableFlags)
	must.Eq(t, "", sub.Variables)

	// check the submission api for v0 (make sure we still have it)
	sub, _, err = nomad.Jobs().Submission(jobID, 0, &api.QueryOptions{
		Region:    "global",
		Namespace: "default",
	})
	must.NoError(t, err)
	must.Eq(t, "hcl2", sub.Format)
	must.NotEq(t, "", sub.Source)
	must.Eq(t, map[string]string{
		"X": "foo",
		"Y": "42",
		"Z": "true",
	}, sub.VariableFlags)
	must.Eq(t, "", sub.Variables)

	// deregister the job with purge
	e2eutil.WaitForJobStopped(t, nomad, jobID)

	// check the submission api for v0 after deregister (make sure its gone)
	sub, _, err = nomad.Jobs().Submission(jobID, 0, &api.QueryOptions{
		Region:    "global",
		Namespace: "default",
	})
	must.ErrorContains(t, err, "job source not found")
	must.Nil(t, sub)
}

func testSubmissionACL(t *testing.T) {
	nomad := e2eutil.NomadClient(t)

	// setup an acl cleanup thing
	aclCleanup := acl.NewCleanup()
	defer aclCleanup.Run(t, nomad)

	// create a namespace for ourselves
	myNamespaceName := "submission-acl-" + uuid.Short()
	namespaceClient := nomad.Namespaces()
	_, err := namespaceClient.Register(&api.Namespace{
		Name: myNamespaceName,
	}, &api.WriteOptions{
		Region: "global",
	})
	must.NoError(t, err)
	aclCleanup.Add(myNamespaceName, acl.NamespaceTestResourceType)

	// create a namespace for a token that will be blocked
	otherNamespaceName := "submission-other-acl-" + uuid.Short()
	_, err = namespaceClient.Register(&api.Namespace{
		Name: otherNamespaceName,
	}, &api.WriteOptions{
		Region: "global",
	})
	must.NoError(t, err)
	aclCleanup.Add(otherNamespaceName, acl.NamespaceTestResourceType)

	// create an ACL policy to read in our namespace
	myNamespacePolicy := api.ACLPolicy{
		Name:        "submission-acl-" + uuid.Short(),
		Rules:       `namespace "` + myNamespaceName + `" {policy = "write"}`,
		Description: "This namespace is for Job Submissions e2e testing",
	}
	_, err = nomad.ACLPolicies().Upsert(&myNamespacePolicy, nil)
	must.NoError(t, err)
	aclCleanup.Add(myNamespacePolicy.Name, acl.ACLPolicyTestResourceType)

	// create an ACL policy to read in the other namespace
	otherNamespacePolicy := api.ACLPolicy{
		Name:        "submission-other-acl-" + uuid.Short(),
		Rules:       `namespace "` + otherNamespaceName + `" {policy = "read"}`,
		Description: "This is another namespace for Job Submissions e2e testing",
	}
	_, err = nomad.ACLPolicies().Upsert(&otherNamespacePolicy, nil)
	must.NoError(t, err)
	aclCleanup.Add(otherNamespacePolicy.Name, acl.ACLPolicyTestResourceType)

	// create a token that can read in our namespace
	aclTokensClient := nomad.ACLTokens()
	myToken, _, err := aclTokensClient.Create(&api.ACLToken{
		Name:     "submission-my-read-token-" + uuid.Short(),
		Type:     "client",
		Policies: []string{myNamespacePolicy.Name},
	}, &api.WriteOptions{
		Region:    "global",
		Namespace: myNamespaceName,
	})
	must.NoError(t, err)
	aclCleanup.Add(myToken.AccessorID, acl.ACLTokenTestResourceType)

	// create a token that can read in the other namespace
	otherToken, _, err := aclTokensClient.Create(&api.ACLToken{
		Name:     "submission-other-read-token-" + uuid.Short(),
		Type:     "client",
		Policies: []string{otherNamespacePolicy.Name},
	}, &api.WriteOptions{
		Region:    "global",
		Namespace: otherNamespaceName,
	})
	must.NoError(t, err)
	aclCleanup.Add(otherToken.AccessorID, acl.ACLTokenTestResourceType)

	// prepare to submit a job
	jobID := "job-sub-cli-" + uuid.Short()
	jobIDs := []string{jobID}
	t.Cleanup(e2eutil.MaybeCleanupJobsAndGC(&jobIDs))

	// register job via cli with var arguments (using management token)
	err = e2eutil.RegisterWithArgs(jobID, "input/xyz.hcl", "-namespace", myNamespaceName, "-var=X=foo", "-var=Y=42", "-var=Z=true")
	must.NoError(t, err)

	// find our alloc id
	allocID := e2eutil.SingleAllocID(t, jobID, myNamespaceName, 0)

	// wait for alloc to complete
	_ = e2eutil.WaitForAllocStopped(t, nomad, allocID)

	// inspect alloc logs making sure our variables got set
	out, err := e2eutil.AllocLogs(allocID, myNamespaceName, e2eutil.LogsStdOut)
	must.NoError(t, err)
	must.Eq(t, "X foo, Y 42, Z true\n", out)

	// get submission using my token
	sub, _, err := nomad.Jobs().Submission(jobID, 0, &api.QueryOptions{
		Region:    "global",
		Namespace: myNamespaceName,
		AuthToken: myToken.SecretID,
	})
	must.NoError(t, err)
	must.Eq(t, "hcl2", sub.Format)
	must.NotEq(t, "", sub.Source)
	must.Eq(t, map[string]string{"X": "foo", "Y": "42", "Z": "true"}, sub.VariableFlags)
	must.Eq(t, "", sub.Variables)

	// get submission using other token (fail)
	sub, _, err = nomad.Jobs().Submission(jobID, 0, &api.QueryOptions{
		Region:    "global",
		Namespace: myNamespaceName,
		AuthToken: otherToken.SecretID,
	})
	must.ErrorContains(t, err, "Permission denied")
	must.Nil(t, sub)
}

func testMaxSize(t *testing.T) {
	jobID := "job-sub-max-" + uuid.Short()
	jobIDs := []string{jobID}
	t.Cleanup(e2eutil.MaybeCleanupJobsAndGC(&jobIDs))

	// modify huge.hcl to exceed the default 1 megabyte limit
	b, err := os.ReadFile("input/huge.hcl")
	must.NoError(t, err)
	huge := strings.Replace(string(b), "REPLACE", strings.Repeat("A", 2e6), 1)
	tmpDir := t.TempDir()
	hugeFile := filepath.Join(tmpDir, "huge.hcl")
	err = os.WriteFile(hugeFile, []byte(huge), 0o644)
	must.NoError(t, err)

	// register the huge job file and expect a warning
	output, err := e2eutil.RegisterGetOutput(jobID, hugeFile)
	must.NoError(t, err)
	must.StrContains(t, output, "job source size of 2.0 MB exceeds maximum of 1.0 MB and will be discarded")

	// check the submission api making sure it is not there
	nomad := e2eutil.NomadClient(t)
	sub, _, err := nomad.Jobs().Submission(jobID, 0, &api.QueryOptions{
		Region:    "global",
		Namespace: "default",
	})
	must.ErrorContains(t, err, "job source not found")
	must.Nil(t, sub)
}

func testReversion(t *testing.T) {
	nomad := e2eutil.NomadClient(t)

	jobID := "job-sub-reversion-" + uuid.Short()
	jobIDs := []string{jobID}
	t.Cleanup(e2eutil.MaybeCleanupJobsAndGC(&jobIDs))

	// register job 3 times
	for i := 0; i < 3; i++ {
		yVar := fmt.Sprintf("-var=Y=%d", i)

		// register the job
		err := e2eutil.RegisterWithArgs(jobID, "input/xyz.hcl", "-var=X=hello", yVar, "-var=Z=false")
		must.NoError(t, err)

		// find our alloc id
		allocID := e2eutil.SingleAllocID(t, jobID, "", i)

		// wait for alloc to complete
		_ = e2eutil.WaitForAllocStopped(t, nomad, allocID)
	}

	// revert the job back to version 1

	err := e2eutil.Revert(jobID, "input/xyz.hcl", 1)
	must.NoError(t, err)

	// there should be a submission for version 3, and it should
	// contain Y=1 as did the version 1 of the job
	expectY := []string{"0", "1", "2", "1"}
	for version := 0; version < 4; version++ {
		sub, _, err := nomad.Jobs().Submission(jobID, version, &api.QueryOptions{
			Region:    "global",
			Namespace: "default",
		})
		must.NoError(t, err)
		must.Eq(t, expectY[version], sub.VariableFlags["Y"])
	}
}

func testVarFiles(t *testing.T) {
	nomad := e2eutil.NomadClient(t)

	jobID := "job-sub-var-files-" + uuid.Short()
	jobIDs := []string{jobID}
	t.Cleanup(e2eutil.MaybeCleanupJobsAndGC(&jobIDs))

	// register the xyz job using x.hcl y.hcl z.hcl var files
	err := e2eutil.RegisterWithArgs(
		jobID,
		"input/xyz.hcl",
		"-var-file=input/x.hcl",
		"-var-file=input/y.hcl",
		"-var-file=input/z.hcl",
	)
	must.NoError(t, err)

	const version = 0

	// find our alloc id
	allocID := e2eutil.SingleAllocID(t, jobID, "", version)

	// wait for alloc to complete
	_ = e2eutil.WaitForAllocStopped(t, nomad, allocID)

	// get submission
	sub, _, err := nomad.Jobs().Submission(jobID, version, &api.QueryOptions{
		Region:    "global",
		Namespace: "default",
	})
	must.NoError(t, err)
	must.StrContains(t, sub.Variables, `X = "my var file x value"`)
	must.StrContains(t, sub.Variables, `Y = 700`)
	must.StrContains(t, sub.Variables, `Z = true`)
}
