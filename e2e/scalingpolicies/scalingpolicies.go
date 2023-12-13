// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scalingpolicies

import (
	"os"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/stretchr/testify/require"
)

type ScalingPolicyE2ETest struct {
	framework.TC
	namespaceIDs     []string
	namespacedJobIDs [][2]string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "ScalingPolicies",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(ScalingPolicyE2ETest),
		},
	})

}

func (tc *ScalingPolicyE2ETest) BeforeAll(f *framework.F) {
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 1)
}

func (tc *ScalingPolicyE2ETest) AfterEach(f *framework.F) {
	if os.Getenv("NOMAD_TEST_SKIPCLEANUP") == "1" {
		return
	}

	for _, namespacedJob := range tc.namespacedJobIDs {
		err := e2eutil.StopJob(namespacedJob[1], "-purge", "-namespace",
			namespacedJob[0])
		f.Assert().NoError(err)
	}
	tc.namespacedJobIDs = [][2]string{}

	for _, ns := range tc.namespaceIDs {
		_, err := e2eutil.Command("nomad", "namespace", "delete", ns)
		f.Assert().NoError(err)
	}
	tc.namespaceIDs = []string{}

	_, err := e2eutil.Command("nomad", "system", "gc")
	f.Assert().NoError(err)
}

// TestScalingPolicies multi-namespace scaling policy test which performs reads
// and job manipulations to ensure Nomad behaves as expected.
func (tc *ScalingPolicyE2ETest) TestScalingPolicies(f *framework.F) {
	t := f.T()

	// Create our non-default namespace.
	_, err := e2eutil.Command("nomad", "namespace", "apply", "NamespaceA")
	f.NoError(err, "could not create namespace")
	tc.namespaceIDs = append(tc.namespaceIDs, "NamespaceA")

	// Register the jobs, capturing their IDs.
	jobDefault1 := tc.run(f, "scalingpolicies/input/namespace_default_1.nomad", "default", []string{"running"})
	jobDefault2 := tc.run(f, "scalingpolicies/input/namespace_default_1.nomad", "default", []string{"running"})
	jobA := tc.run(f, "scalingpolicies/input/namespace_a_1.nomad", "NamespaceA", []string{"running"})

	// Setup some reused query options.
	defaultQueryOpts := api.QueryOptions{Namespace: "default"}
	aQueryOpts := api.QueryOptions{Namespace: "NamespaceA"}

	// Perform initial listings to check each namespace has the correct number
	// of policies.
	defaultPolicyList, _, err := tc.Nomad().Scaling().ListPolicies(&defaultQueryOpts)
	require.NoError(t, err)
	require.Len(t, defaultPolicyList, 2)

	policyListA, _, err := tc.Nomad().Scaling().ListPolicies(&aQueryOpts)
	require.NoError(t, err)
	require.Len(t, policyListA, 1)

	// Deregister a job from the default namespace and then check all the
	// response objects.
	_, _, err = tc.Nomad().Jobs().Deregister(jobDefault1, true, &api.WriteOptions{Namespace: "default"})
	require.NoError(t, err)

	for i, namespacedJob := range tc.namespacedJobIDs {
		if namespacedJob[1] == jobDefault1 && namespacedJob[0] == "default" {
			tc.namespacedJobIDs = append(tc.namespacedJobIDs[:i], tc.namespacedJobIDs[i+1:]...)
			break
		}
	}

	defaultPolicyList, _, err = tc.Nomad().Scaling().ListPolicies(&defaultQueryOpts)
	require.NoError(t, err)
	require.Len(t, defaultPolicyList, 1)

	defaultPolicy := defaultPolicyList[0]
	require.True(t, defaultPolicy.Enabled)
	require.Equal(t, "horizontal", defaultPolicy.Type)
	require.Equal(t, defaultPolicy.Target["Namespace"], "default")
	require.Equal(t, defaultPolicy.Target["Job"], jobDefault2)
	require.Equal(t, defaultPolicy.Target["Group"], "horizontally_scalable")

	defaultPolicyInfo, _, err := tc.Nomad().Scaling().GetPolicy(defaultPolicy.ID, &defaultQueryOpts)
	require.NoError(t, err)
	require.Equal(t, *defaultPolicyInfo.Min, int64(1))
	require.Equal(t, *defaultPolicyInfo.Max, int64(10))
	require.Equal(t, defaultPolicyInfo.Policy["cooldown"], "13m")
	require.Equal(t, defaultPolicyInfo.Target["Namespace"], "default")
	require.Equal(t, defaultPolicyInfo.Target["Job"], jobDefault2)
	require.Equal(t, defaultPolicyInfo.Target["Group"], "horizontally_scalable")

	// Check response objects from the namespace with name "NamespaceA".
	aPolicyList, _, err := tc.Nomad().Scaling().ListPolicies(&aQueryOpts)
	require.NoError(t, err)
	require.Len(t, aPolicyList, 1)

	aPolicy := aPolicyList[0]
	require.True(t, aPolicy.Enabled)
	require.Equal(t, "horizontal", aPolicy.Type)
	require.Equal(t, aPolicy.Target["Namespace"], "NamespaceA")
	require.Equal(t, aPolicy.Target["Job"], jobA)
	require.Equal(t, aPolicy.Target["Group"], "horizontally_scalable")

	aPolicyInfo, _, err := tc.Nomad().Scaling().GetPolicy(aPolicy.ID, &aQueryOpts)
	require.NoError(t, err)
	require.Equal(t, *aPolicyInfo.Min, int64(1))
	require.Equal(t, *aPolicyInfo.Max, int64(10))
	require.Equal(t, aPolicyInfo.Policy["cooldown"], "13m")
	require.Equal(t, aPolicyInfo.Target["Namespace"], "NamespaceA")
	require.Equal(t, aPolicyInfo.Target["Job"], jobA)
	require.Equal(t, aPolicyInfo.Target["Group"], "horizontally_scalable")

	// List policies using the splat namespace operator.
	splatPolicyList, _, err := tc.Nomad().Scaling().ListPolicies(&api.QueryOptions{Namespace: "*"})
	require.NoError(t, err)
	require.Len(t, splatPolicyList, 2)

	// Deregister the job from the "NamespaceA" namespace and then check the
	// response objects.
	_, _, err = tc.Nomad().Jobs().Deregister(jobA, true, &api.WriteOptions{Namespace: "NamespaceA"})
	require.NoError(t, err)

	for i, namespacedJob := range tc.namespacedJobIDs {
		if namespacedJob[1] == jobA && namespacedJob[0] == "NamespaceA" {
			tc.namespacedJobIDs = append(tc.namespacedJobIDs[:i], tc.namespacedJobIDs[i+1:]...)
			break
		}
	}

	aPolicyList, _, err = tc.Nomad().Scaling().ListPolicies(&aQueryOpts)
	require.NoError(t, err)
	require.Len(t, aPolicyList, 0)

	// Update the running job scaling policy and ensure the changes are
	// reflected.
	err = e2eutil.Register(jobDefault2, "scalingpolicies/input/namespace_default_2.nomad")
	require.NoError(t, err)

	defaultPolicyList, _, err = tc.Nomad().Scaling().ListPolicies(&defaultQueryOpts)
	require.NoError(t, err)
	require.Len(t, defaultPolicyList, 1)

	defaultPolicyInfo, _, err = tc.Nomad().Scaling().GetPolicy(defaultPolicyList[0].ID, &defaultQueryOpts)
	require.NoError(t, err)
	require.Equal(t, *defaultPolicyInfo.Min, int64(1))
	require.Equal(t, *defaultPolicyInfo.Max, int64(11))
	require.Equal(t, defaultPolicyInfo.Policy["cooldown"], "14m")
	require.Equal(t, defaultPolicyInfo.Target["Namespace"], "default")
	require.Equal(t, defaultPolicyInfo.Target["Job"], jobDefault2)
	require.Equal(t, defaultPolicyInfo.Target["Group"], "horizontally_scalable")

}

// run is a helper which runs a job within a namespace, providing the caller
// with the generated jobID.
func (tc *ScalingPolicyE2ETest) run(f *framework.F, jobSpec, ns string, expected []string) string {
	jobID := "test-scaling-policy-" + uuid.Generate()[0:8]
	f.NoError(e2eutil.Register(jobID, jobSpec))
	tc.namespacedJobIDs = append(tc.namespacedJobIDs, [2]string{ns, jobID})
	f.NoError(e2eutil.WaitForAllocStatusExpected(jobID, ns, expected), "job should be running")
	return jobID
}
