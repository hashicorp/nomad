// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskenv

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	hcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// Node values that tests can rely on
	metaKey   = "instance"
	metaVal   = "t2-micro"
	attrKey   = "arch"
	attrVal   = "amd64"
	nodeName  = "test node"
	nodeClass = "test class"

	// Environment variable values that tests can rely on
	envOneKey = "NOMAD_IP"
	envOneVal = "127.0.0.1"
	envTwoKey = "NOMAD_PORT_WEB"
	envTwoVal = ":80"
)

var (
	// portMap for use in tests as its set after Builder creation
	portMap = map[string]int{
		"https": 443,
	}
)

func testEnvBuilder() *Builder {
	n := mock.Node()
	n.Attributes = map[string]string{
		attrKey: attrVal,
	}
	n.Meta = map[string]string{
		metaKey: metaVal,
	}
	n.Name = nodeName
	n.NodeClass = nodeClass

	task := mock.Job().TaskGroups[0].Tasks[0]
	task.Env = map[string]string{
		envOneKey: envOneVal,
		envTwoKey: envTwoVal,
	}
	return NewBuilder(n, mock.Alloc(), task, "global")
}

func TestEnvironment_ParseAndReplace_Env(t *testing.T) {
	ci.Parallel(t)

	env := testEnvBuilder()

	input := []string{fmt.Sprintf(`"${%v}"!`, envOneKey), fmt.Sprintf("${%s}${%s}", envOneKey, envTwoKey)}
	act := env.Build().ParseAndReplace(input)
	exp := []string{fmt.Sprintf(`"%s"!`, envOneVal), fmt.Sprintf("%s%s", envOneVal, envTwoVal)}

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("ParseAndReplace(%v) returned %#v; want %#v", input, act, exp)
	}
}

func TestEnvironment_ParseAndReplace_Meta(t *testing.T) {
	ci.Parallel(t)

	input := []string{fmt.Sprintf("${%v%v}", nodeMetaPrefix, metaKey)}
	exp := []string{metaVal}
	env := testEnvBuilder()
	act := env.Build().ParseAndReplace(input)

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("ParseAndReplace(%v) returned %#v; want %#v", input, act, exp)
	}
}

func TestEnvironment_ParseAndReplace_Attr(t *testing.T) {
	ci.Parallel(t)

	input := []string{fmt.Sprintf("${%v%v}", nodeAttributePrefix, attrKey)}
	exp := []string{attrVal}
	env := testEnvBuilder()
	act := env.Build().ParseAndReplace(input)

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("ParseAndReplace(%v) returned %#v; want %#v", input, act, exp)
	}
}

func TestEnvironment_ParseAndReplace_Node(t *testing.T) {
	ci.Parallel(t)

	input := []string{fmt.Sprintf("${%v}", nodeNameKey), fmt.Sprintf("${%v}", nodeClassKey)}
	exp := []string{nodeName, nodeClass}
	env := testEnvBuilder()
	act := env.Build().ParseAndReplace(input)

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("ParseAndReplace(%v) returned %#v; want %#v", input, act, exp)
	}
}

func TestEnvironment_ParseAndReplace_Mixed(t *testing.T) {
	ci.Parallel(t)

	input := []string{
		fmt.Sprintf("${%v}${%v%v}", nodeNameKey, nodeAttributePrefix, attrKey),
		fmt.Sprintf("${%v}${%v%v}", nodeClassKey, nodeMetaPrefix, metaKey),
		fmt.Sprintf("${%v}${%v}", envTwoKey, nodeClassKey),
	}
	exp := []string{
		fmt.Sprintf("%v%v", nodeName, attrVal),
		fmt.Sprintf("%v%v", nodeClass, metaVal),
		fmt.Sprintf("%v%v", envTwoVal, nodeClass),
	}
	env := testEnvBuilder()
	act := env.Build().ParseAndReplace(input)

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("ParseAndReplace(%v) returned %#v; want %#v", input, act, exp)
	}
}

func TestEnvironment_ReplaceEnv_Mixed(t *testing.T) {
	ci.Parallel(t)

	input := fmt.Sprintf("${%v}${%v%v}", nodeNameKey, nodeAttributePrefix, attrKey)
	exp := fmt.Sprintf("%v%v", nodeName, attrVal)
	env := testEnvBuilder()
	act := env.Build().ReplaceEnv(input)

	if act != exp {
		t.Fatalf("ParseAndReplace(%v) returned %#v; want %#v", input, act, exp)
	}
}

func TestEnvironment_AsList(t *testing.T) {
	ci.Parallel(t)

	n := mock.Node()
	n.Meta = map[string]string{
		"metaKey": "metaVal",
	}
	a := mock.Alloc()
	a.Job.ParentID = fmt.Sprintf("mock-parent-service-%s", uuid.Generate())
	a.AllocatedResources.Tasks["web"] = &structs.AllocatedTaskResources{
		Cpu: structs.AllocatedCpuResources{
			CpuShares:     500,
			ReservedCores: []uint16{0, 5, 6, 7},
		},
		Memory: structs.AllocatedMemoryResources{
			MemoryMB:    256,
			MemoryMaxMB: 512,
		},
		Networks: []*structs.NetworkResource{{
			Device:        "eth0",
			IP:            "127.0.0.1",
			ReservedPorts: []structs.Port{{Label: "https", Value: 8080}},
			MBits:         50,
			DynamicPorts:  []structs.Port{{Label: "http", Value: 80}},
		}},
	}

	a.AllocatedResources.Tasks["ssh"] = &structs.AllocatedTaskResources{
		Networks: []*structs.NetworkResource{
			{
				Device: "eth0",
				IP:     "192.168.0.100",
				MBits:  50,
				ReservedPorts: []structs.Port{
					{Label: "ssh", Value: 22},
					{Label: "other", Value: 1234},
				},
			},
		},
	}
	a.AllocatedResources.Tasks["mail"] = &structs.AllocatedTaskResources{
		Networks: []*structs.NetworkResource{
			{
				Device: "eth0",
				IP:     "fd12:3456:789a:1::1",
				MBits:  50,
				ReservedPorts: []structs.Port{
					{Label: "ipv6", Value: 2222},
				},
			},
		},
	}
	a.Namespace = "not-default"
	task := a.Job.TaskGroups[0].Tasks[0]
	task.Env = map[string]string{
		"taskEnvKey": "taskEnvVal",
	}
	env := NewBuilder(n, a, task, "global").SetDriverNetwork(
		&drivers.DriverNetwork{PortMap: map[string]int{"https": 443}},
	)

	act := env.Build().List()
	exp := []string{
		"taskEnvKey=taskEnvVal",
		"NOMAD_ADDR_http=127.0.0.1:80",
		"NOMAD_PORT_http=80",
		"NOMAD_IP_http=127.0.0.1",
		"NOMAD_ADDR_https=127.0.0.1:8080",
		"NOMAD_PORT_https=443",
		"NOMAD_IP_https=127.0.0.1",
		"NOMAD_HOST_PORT_http=80",
		"NOMAD_HOST_PORT_https=8080",
		"NOMAD_TASK_NAME=web",
		"NOMAD_GROUP_NAME=web",
		"NOMAD_ADDR_ssh_other=192.168.0.100:1234",
		"NOMAD_ADDR_ssh_ssh=192.168.0.100:22",
		"NOMAD_IP_ssh_other=192.168.0.100",
		"NOMAD_IP_ssh_ssh=192.168.0.100",
		"NOMAD_PORT_ssh_other=1234",
		"NOMAD_PORT_ssh_ssh=22",
		"NOMAD_ADDR_mail_ipv6=[fd12:3456:789a:1::1]:2222",
		"NOMAD_IP_mail_ipv6=fd12:3456:789a:1::1",
		"NOMAD_PORT_mail_ipv6=2222",
		"NOMAD_CPU_LIMIT=500",
		"NOMAD_CPU_CORES=0,5-7",
		"NOMAD_DC=dc1",
		"NOMAD_NAMESPACE=not-default",
		"NOMAD_REGION=global",
		"NOMAD_MEMORY_LIMIT=256",
		"NOMAD_MEMORY_MAX_LIMIT=512",
		"NOMAD_META_ELB_CHECK_INTERVAL=30s",
		"NOMAD_META_ELB_CHECK_MIN=3",
		"NOMAD_META_ELB_CHECK_TYPE=http",
		"NOMAD_META_FOO=bar",
		"NOMAD_META_OWNER=armon",
		"NOMAD_META_elb_check_interval=30s",
		"NOMAD_META_elb_check_min=3",
		"NOMAD_META_elb_check_type=http",
		"NOMAD_META_foo=bar",
		"NOMAD_META_owner=armon",
		fmt.Sprintf("NOMAD_JOB_ID=%s", a.Job.ID),
		"NOMAD_JOB_NAME=my-job",
		fmt.Sprintf("NOMAD_JOB_PARENT_ID=%s", a.Job.ParentID),
		fmt.Sprintf("NOMAD_ALLOC_ID=%s", a.ID),
		fmt.Sprintf("NOMAD_SHORT_ALLOC_ID=%s", a.ID[:8]),
		"NOMAD_ALLOC_INDEX=0",
	}
	sort.Strings(act)
	sort.Strings(exp)
	require.Equal(t, exp, act)
}

func TestEnvironment_AllValues(t *testing.T) {
	ci.Parallel(t)

	n := mock.Node()
	n.Meta = map[string]string{
		"metaKey":           "metaVal",
		"nested.meta.key":   "a",
		"invalid...metakey": "b",
	}
	n.CgroupParent = "abc.slice"
	a := mock.ConnectAlloc()
	a.Job.ParentID = fmt.Sprintf("mock-parent-service-%s", uuid.Generate())
	a.AllocatedResources.Tasks["web"].Networks[0] = &structs.NetworkResource{
		Device:        "eth0",
		IP:            "127.0.0.1",
		ReservedPorts: []structs.Port{{Label: "https", Value: 8080}},
		MBits:         50,
		DynamicPorts:  []structs.Port{{Label: "http", Value: 80}},
	}
	a.AllocatedResources.Tasks["web"].Cpu.ReservedCores = []uint16{0, 5, 6, 7}
	a.AllocatedResources.Tasks["ssh"] = &structs.AllocatedTaskResources{
		Networks: []*structs.NetworkResource{
			{
				Device: "eth0",
				IP:     "192.168.0.100",
				MBits:  50,
				ReservedPorts: []structs.Port{
					{Label: "ssh", Value: 22},
					{Label: "other", Value: 1234},
				},
			},
		},
	}

	a.AllocatedResources.Shared.Ports = structs.AllocatedPorts{
		{
			Label:  "admin",
			Value:  32000,
			To:     9000,
			HostIP: "127.0.0.1",
		},
	}

	sharedNet := a.AllocatedResources.Shared.Networks[0]

	// Add group network port with only a host port.
	sharedNet.DynamicPorts = append(sharedNet.DynamicPorts, structs.Port{
		Label: "hostonly",
		Value: 9998,
	})

	// Add group network reserved port with a To value.
	sharedNet.ReservedPorts = append(sharedNet.ReservedPorts, structs.Port{
		Label: "static",
		Value: 9997,
		To:    97,
	})

	task := a.Job.TaskGroups[0].Tasks[0]
	task.Env = map[string]string{
		"taskEnvKey":        "taskEnvVal",
		"nested.task.key":   "x",
		"invalid...taskkey": "y",
		".a":                "a",
		"b.":                "b",
		".":                 "c",
	}
	task.Meta = map[string]string{
		"taskMetaKey-${NOMAD_TASK_NAME}": "taskMetaVal-${node.unique.id}",
		"foo":                            "bar",
	}
	env := NewBuilder(n, a, task, "global").SetDriverNetwork(
		&drivers.DriverNetwork{PortMap: map[string]int{"https": 443}},
	)

	// Add a host environment variable which matches a task variable. It means
	// we can test to ensure the allocation ID variable from the task overrides
	// that found on the host. The second entry tests to ensure other host env
	// vars are added as expected.
	env.mu.Lock()
	env.hostEnv = map[string]string{
		AllocID:    "94fa69a3-73a5-4099-85c3-7a1b6e228796",
		"LC_CTYPE": "C.UTF-8",
	}
	env.mu.Unlock()

	values, errs, err := env.Build().AllValues()
	require.NoError(t, err)

	// Assert the keys we couldn't nest were reported
	require.Len(t, errs, 5)
	require.Contains(t, errs, "invalid...taskkey")
	require.Contains(t, errs, "meta.invalid...metakey")
	require.Contains(t, errs, ".a")
	require.Contains(t, errs, "b.")
	require.Contains(t, errs, ".")

	exp := map[string]string{
		// Node
		"node.unique.id":          n.ID,
		"node.region":             "global",
		"node.datacenter":         n.Datacenter,
		"node.unique.name":        n.Name,
		"node.class":              n.NodeClass,
		"meta.metaKey":            "metaVal",
		"attr.arch":               "x86",
		"attr.driver.exec":        "1",
		"attr.driver.mock_driver": "1",
		"attr.kernel.name":        "linux",
		"attr.nomad.version":      "0.5.0",

		// 0.9 style meta and attr
		"node.meta.metaKey":            "metaVal",
		"node.attr.arch":               "x86",
		"node.attr.driver.exec":        "1",
		"node.attr.driver.mock_driver": "1",
		"node.attr.kernel.name":        "linux",
		"node.attr.nomad.version":      "0.5.0",

		// Env
		"taskEnvKey":                                "taskEnvVal",
		"NOMAD_ADDR_http":                           "127.0.0.1:80",
		"NOMAD_PORT_http":                           "80",
		"NOMAD_IP_http":                             "127.0.0.1",
		"NOMAD_ADDR_https":                          "127.0.0.1:8080",
		"NOMAD_PORT_https":                          "443",
		"NOMAD_IP_https":                            "127.0.0.1",
		"NOMAD_HOST_PORT_http":                      "80",
		"NOMAD_HOST_PORT_https":                     "8080",
		"NOMAD_TASK_NAME":                           "web",
		"NOMAD_GROUP_NAME":                          "web",
		"NOMAD_ADDR_ssh_other":                      "192.168.0.100:1234",
		"NOMAD_ADDR_ssh_ssh":                        "192.168.0.100:22",
		"NOMAD_IP_ssh_other":                        "192.168.0.100",
		"NOMAD_IP_ssh_ssh":                          "192.168.0.100",
		"NOMAD_PORT_ssh_other":                      "1234",
		"NOMAD_PORT_ssh_ssh":                        "22",
		"NOMAD_CPU_LIMIT":                           "500",
		"NOMAD_CPU_CORES":                           "0,5-7",
		"NOMAD_DC":                                  "dc1",
		"NOMAD_PARENT_CGROUP":                       "abc.slice",
		"NOMAD_NAMESPACE":                           "default",
		"NOMAD_REGION":                              "global",
		"NOMAD_MEMORY_LIMIT":                        "256",
		"NOMAD_META_ELB_CHECK_INTERVAL":             "30s",
		"NOMAD_META_ELB_CHECK_MIN":                  "3",
		"NOMAD_META_ELB_CHECK_TYPE":                 "http",
		"NOMAD_META_FOO":                            "bar",
		"NOMAD_META_OWNER":                          "armon",
		"NOMAD_META_elb_check_interval":             "30s",
		"NOMAD_META_elb_check_min":                  "3",
		"NOMAD_META_elb_check_type":                 "http",
		"NOMAD_META_foo":                            "bar",
		"NOMAD_META_owner":                          "armon",
		"NOMAD_META_taskMetaKey_web":                "taskMetaVal-" + n.ID,
		"NOMAD_JOB_ID":                              a.Job.ID,
		"NOMAD_JOB_NAME":                            "my-job",
		"NOMAD_JOB_PARENT_ID":                       a.Job.ParentID,
		"NOMAD_ALLOC_ID":                            a.ID,
		"NOMAD_SHORT_ALLOC_ID":                      a.ID[:8],
		"NOMAD_ALLOC_INDEX":                         "0",
		"NOMAD_PORT_connect_proxy_testconnect":      "9999",
		"NOMAD_HOST_PORT_connect_proxy_testconnect": "9999",
		"NOMAD_PORT_hostonly":                       "9998",
		"NOMAD_HOST_PORT_hostonly":                  "9998",
		"NOMAD_PORT_static":                         "97",
		"NOMAD_HOST_PORT_static":                    "9997",
		"NOMAD_ADDR_admin":                          "127.0.0.1:32000",
		"NOMAD_HOST_ADDR_admin":                     "127.0.0.1:32000",
		"NOMAD_IP_admin":                            "127.0.0.1",
		"NOMAD_HOST_IP_admin":                       "127.0.0.1",
		"NOMAD_PORT_admin":                          "9000",
		"NOMAD_ALLOC_PORT_admin":                    "9000",
		"NOMAD_HOST_PORT_admin":                     "32000",

		// Env vars from the host.
		"LC_CTYPE": "C.UTF-8",

		// 0.9 style env map
		`env["taskEnvKey"]`:        "taskEnvVal",
		`env["NOMAD_ADDR_http"]`:   "127.0.0.1:80",
		`env["nested.task.key"]`:   "x",
		`env["invalid...taskkey"]`: "y",
		`env[".a"]`:                "a",
		`env["b."]`:                "b",
		`env["."]`:                 "c",
	}

	evalCtx := &hcl.EvalContext{
		Variables: values,
	}

	for k, expectedVal := range exp {
		t.Run(k, func(t *testing.T) {
			// Parse HCL containing the test key
			hclStr := fmt.Sprintf(`"${%s}"`, k)
			expr, diag := hclsyntax.ParseExpression([]byte(hclStr), "test.hcl", hcl.Pos{})
			require.Empty(t, diag)

			// Decode with the TaskEnv values
			out := ""
			diag = gohcl.DecodeExpression(expr, evalCtx, &out)
			require.Empty(t, diag)
			require.Equal(t, expectedVal, out,
				fmt.Sprintf("expected %q got %q", expectedVal, out))
		})
	}
}

func TestEnvironment_VaultToken(t *testing.T) {
	ci.Parallel(t)

	n := mock.Node()
	a := mock.Alloc()
	env := NewBuilder(n, a, a.Job.TaskGroups[0].Tasks[0], "global")
	env.SetVaultToken("123", "vault-namespace", false)

	{
		act := env.Build().All()
		if act[VaultToken] != "" {
			t.Fatalf("Unexpected environment variables: %s=%q", VaultToken, act[VaultToken])
		}
		if act[VaultNamespace] != "" {
			t.Fatalf("Unexpected environment variables: %s=%q", VaultNamespace, act[VaultNamespace])
		}
	}

	{
		act := env.SetVaultToken("123", "", true).Build().List()
		exp := "VAULT_TOKEN=123"
		found := false
		foundNs := false
		for _, entry := range act {
			if entry == exp {
				found = true
			}
			if strings.HasPrefix(entry, "VAULT_NAMESPACE=") {
				foundNs = true
			}
		}
		if !found {
			t.Fatalf("did not find %q in:\n%s", exp, strings.Join(act, "\n"))
		}
		if foundNs {
			t.Fatalf("found unwanted VAULT_NAMESPACE in:\n%s", strings.Join(act, "\n"))
		}
	}

	{
		act := env.SetVaultToken("123", "vault-namespace", true).Build().List()
		exp := "VAULT_TOKEN=123"
		expNs := "VAULT_NAMESPACE=vault-namespace"
		found := false
		foundNs := false
		for _, entry := range act {
			if entry == exp {
				found = true
			}
			if entry == expNs {
				foundNs = true
			}
		}
		if !found {
			t.Fatalf("did not find %q in:\n%s", exp, strings.Join(act, "\n"))
		}
		if !foundNs {
			t.Fatalf("did not find %q in:\n%s", expNs, strings.Join(act, "\n"))
		}
	}
}

func TestEnvironment_Envvars(t *testing.T) {
	ci.Parallel(t)

	envMap := map[string]string{"foo": "baz", "bar": "bang"}
	n := mock.Node()
	a := mock.Alloc()
	task := a.Job.TaskGroups[0].Tasks[0]
	task.Env = envMap
	net := &drivers.DriverNetwork{PortMap: portMap}
	act := NewBuilder(n, a, task, "global").SetDriverNetwork(net).Build().All()
	for k, v := range envMap {
		actV, ok := act[k]
		if !ok {
			t.Fatalf("missing %q in %#v", k, act)
		}
		if v != actV {
			t.Fatalf("expected %s=%q but found %q", k, v, actV)
		}
	}
}

// TestEnvironment_HookVars asserts hook env vars are LWW and deletes of later
// writes allow earlier hook's values to be visible.
func TestEnvironment_HookVars(t *testing.T) {
	ci.Parallel(t)

	n := mock.Node()
	a := mock.Alloc()
	builder := NewBuilder(n, a, a.Job.TaskGroups[0].Tasks[0], "global")

	// Add vars from two hooks and assert the second one wins on
	// conflicting keys.
	builder.SetHookEnv("hookA", map[string]string{
		"foo": "bar",
		"baz": "quux",
	})
	builder.SetHookEnv("hookB", map[string]string{
		"foo":   "123",
		"hookB": "wins",
	})

	{
		out := builder.Build().All()
		assert.Equal(t, "123", out["foo"])
		assert.Equal(t, "quux", out["baz"])
		assert.Equal(t, "wins", out["hookB"])
	}

	// Asserting overwriting hook vars allows the first hooks original
	// value to be used.
	builder.SetHookEnv("hookB", nil)
	{
		out := builder.Build().All()
		assert.Equal(t, "bar", out["foo"])
		assert.Equal(t, "quux", out["baz"])
		assert.NotContains(t, out, "hookB")
	}
}

// TestEnvironment_DeviceHookVars asserts device hook env vars are accessible
// separately.
func TestEnvironment_DeviceHookVars(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	n := mock.Node()
	a := mock.Alloc()
	builder := NewBuilder(n, a, a.Job.TaskGroups[0].Tasks[0], "global")

	// Add vars from two hooks and assert the second one wins on
	// conflicting keys.
	builder.SetHookEnv("hookA", map[string]string{
		"foo": "bar",
		"baz": "quux",
	})
	builder.SetDeviceHookEnv("devices", map[string]string{
		"hook": "wins",
	})

	b := builder.Build()
	deviceEnv := b.DeviceEnv()
	require.Len(deviceEnv, 1)
	require.Contains(deviceEnv, "hook")

	all := b.Map()
	require.Contains(all, "foo")
}

func TestEnvironment_Interpolate(t *testing.T) {
	ci.Parallel(t)

	n := mock.Node()
	n.Attributes["arch"] = "x86"
	n.NodeClass = "test class"
	a := mock.Alloc()
	task := a.Job.TaskGroups[0].Tasks[0]
	task.Env = map[string]string{"test": "${node.class}", "test2": "${attr.arch}"}
	env := NewBuilder(n, a, task, "global").Build()

	exp := []string{fmt.Sprintf("test=%s", n.NodeClass), fmt.Sprintf("test2=%s", n.Attributes["arch"])}
	found1, found2 := false, false
	for _, entry := range env.List() {
		switch entry {
		case exp[0]:
			found1 = true
		case exp[1]:
			found2 = true
		}
	}
	if !found1 || !found2 {
		t.Fatalf("expected to find %q and %q but got:\n%s",
			exp[0], exp[1], strings.Join(env.List(), "\n"))
	}
}

func TestEnvironment_AppendHostEnvvars(t *testing.T) {
	ci.Parallel(t)

	host := os.Environ()
	if len(host) < 2 {
		t.Skip("No host environment variables. Can't test")
	}
	skip := strings.Split(host[0], "=")[0]
	env := testEnvBuilder().
		SetHostEnvvars([]string{skip}).
		Build()

	act := env.Map()
	if len(act) < 1 {
		t.Fatalf("Host environment variables not properly set")
	}
	if _, ok := act[skip]; ok {
		t.Fatalf("Didn't filter environment variable %q", skip)
	}
}

// TestEnvironment_DashesInTaskName asserts dashes in port labels are properly
// converted to underscores in environment variables.
// See: https://github.com/hashicorp/nomad/issues/2405
func TestEnvironment_DashesInTaskName(t *testing.T) {
	ci.Parallel(t)

	a := mock.Alloc()
	task := a.Job.TaskGroups[0].Tasks[0]
	task.Env = map[string]string{
		"test-one-two":       "three-four",
		"NOMAD_test_one_two": "three-five",
	}
	envMap := NewBuilder(mock.Node(), a, task, "global").Build().Map()

	if envMap["test-one-two"] != "three-four" {
		t.Fatalf("Expected test-one-two=three-four in TaskEnv; found:\n%#v", envMap)
	}
	if envMap["NOMAD_test_one_two"] != "three-five" {
		t.Fatalf("Expected NOMAD_test_one_two=three-five in TaskEnv; found:\n%#v", envMap)
	}
}

// TestEnvironment_UpdateTask asserts env vars and task meta are updated when a
// task is updated.
func TestEnvironment_UpdateTask(t *testing.T) {
	ci.Parallel(t)

	a := mock.Alloc()
	a.Job.TaskGroups[0].Meta = map[string]string{"tgmeta": "tgmetaval"}
	task := a.Job.TaskGroups[0].Tasks[0]
	task.Name = "orig"
	task.Env = map[string]string{"env": "envval"}
	task.Meta = map[string]string{"taskmeta": "taskmetaval"}
	builder := NewBuilder(mock.Node(), a, task, "global")

	origMap := builder.Build().Map()
	if origMap["NOMAD_TASK_NAME"] != "orig" {
		t.Errorf("Expected NOMAD_TASK_NAME=orig but found %q", origMap["NOMAD_TASK_NAME"])
	}
	if origMap["NOMAD_META_taskmeta"] != "taskmetaval" {
		t.Errorf("Expected NOMAD_META_taskmeta=taskmetaval but found %q", origMap["NOMAD_META_taskmeta"])
	}
	if origMap["env"] != "envval" {
		t.Errorf("Expected env=envva but found %q", origMap["env"])
	}
	if origMap["NOMAD_META_tgmeta"] != "tgmetaval" {
		t.Errorf("Expected NOMAD_META_tgmeta=tgmetaval but found %q", origMap["NOMAD_META_tgmeta"])
	}

	a.Job.TaskGroups[0].Meta = map[string]string{"tgmeta2": "tgmetaval2"}
	task.Name = "new"
	task.Env = map[string]string{"env2": "envval2"}
	task.Meta = map[string]string{"taskmeta2": "taskmetaval2"}

	newMap := builder.UpdateTask(a, task).Build().Map()
	if newMap["NOMAD_TASK_NAME"] != "new" {
		t.Errorf("Expected NOMAD_TASK_NAME=new but found %q", newMap["NOMAD_TASK_NAME"])
	}
	if newMap["NOMAD_META_taskmeta2"] != "taskmetaval2" {
		t.Errorf("Expected NOMAD_META_taskmeta=taskmetaval but found %q", newMap["NOMAD_META_taskmeta2"])
	}
	if newMap["env2"] != "envval2" {
		t.Errorf("Expected env=envva but found %q", newMap["env2"])
	}
	if newMap["NOMAD_META_tgmeta2"] != "tgmetaval2" {
		t.Errorf("Expected NOMAD_META_tgmeta=tgmetaval but found %q", newMap["NOMAD_META_tgmeta2"])
	}
	if v, ok := newMap["NOMAD_META_taskmeta"]; ok {
		t.Errorf("Expected NOMAD_META_taskmeta to be unset but found: %q", v)
	}
}

// TestEnvironment_InterpolateEmptyOptionalMeta asserts that in a parameterized
// job, if an optional meta field is not set, it will get interpolated as an
// empty string.
func TestEnvironment_InterpolateEmptyOptionalMeta(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	a := mock.Alloc()
	a.Job.ParameterizedJob = &structs.ParameterizedJobConfig{
		MetaOptional: []string{"metaopt1", "metaopt2"},
	}
	a.Job.Dispatched = true
	task := a.Job.TaskGroups[0].Tasks[0]
	task.Meta = map[string]string{"metaopt1": "metaopt1val"}
	env := NewBuilder(mock.Node(), a, task, "global").Build()
	require.Equal("metaopt1val", env.ReplaceEnv("${NOMAD_META_metaopt1}"))
	require.Empty(env.ReplaceEnv("${NOMAD_META_metaopt2}"))
}

// TestEnvironment_Upsteams asserts that group.service.upstreams entries are
// added to the environment.
func TestEnvironment_Upstreams(t *testing.T) {
	ci.Parallel(t)

	// Add some upstreams to the mock alloc
	a := mock.Alloc()
	tg := a.Job.LookupTaskGroup(a.TaskGroup)
	tg.Services = []*structs.Service{
		// Services without Connect should be ignored
		{
			Name: "ignoreme",
		},
		// All upstreams from a service should be added
		{
			Name: "remote_service",
			Connect: &structs.ConsulConnect{
				SidecarService: &structs.ConsulSidecarService{
					Proxy: &structs.ConsulProxy{
						Upstreams: []structs.ConsulUpstream{
							{
								DestinationName: "foo-bar",
								LocalBindPort:   1234,
							},
							{
								DestinationName: "bar",
								LocalBindPort:   5678,
							},
						},
					},
				},
			},
		},
	}

	// Ensure the upstreams can be interpolated
	tg.Tasks[0].Env = map[string]string{
		"foo": "${NOMAD_UPSTREAM_ADDR_foo_bar}",
		"bar": "${NOMAD_UPSTREAM_PORT_foo-bar}",
	}

	env := NewBuilder(mock.Node(), a, tg.Tasks[0], "global").Build().Map()
	require.Equal(t, "127.0.0.1:1234", env["NOMAD_UPSTREAM_ADDR_foo_bar"])
	require.Equal(t, "127.0.0.1", env["NOMAD_UPSTREAM_IP_foo_bar"])
	require.Equal(t, "1234", env["NOMAD_UPSTREAM_PORT_foo_bar"])
	require.Equal(t, "127.0.0.1:5678", env["NOMAD_UPSTREAM_ADDR_bar"])
	require.Equal(t, "127.0.0.1", env["NOMAD_UPSTREAM_IP_bar"])
	require.Equal(t, "5678", env["NOMAD_UPSTREAM_PORT_bar"])
	require.Equal(t, "127.0.0.1:1234", env["foo"])
	require.Equal(t, "1234", env["bar"])
}

func TestEnvironment_SetPortMapEnvs(t *testing.T) {
	ci.Parallel(t)

	envs := map[string]string{
		"foo":            "bar",
		"NOMAD_PORT_ssh": "2342",
	}
	ports := map[string]int{
		"ssh":  22,
		"http": 80,
	}

	envs = SetPortMapEnvs(envs, ports)

	expected := map[string]string{
		"foo":             "bar",
		"NOMAD_PORT_ssh":  "22",
		"NOMAD_PORT_http": "80",
	}
	require.Equal(t, expected, envs)
}

func TestEnvironment_TasklessBuilder(t *testing.T) {
	ci.Parallel(t)

	node := mock.Node()
	alloc := mock.Alloc()
	alloc.Job.Meta["jobt"] = "foo"
	alloc.Job.TaskGroups[0].Meta["groupt"] = "bar"
	require := require.New(t)
	var taskEnv *TaskEnv
	require.NotPanics(func() {
		taskEnv = NewBuilder(node, alloc, nil, "global").SetAllocDir("/tmp/alloc").Build()
	})

	require.Equal("foo", taskEnv.ReplaceEnv("${NOMAD_META_jobt}"))
	require.Equal("bar", taskEnv.ReplaceEnv("${NOMAD_META_groupt}"))
}

func TestTaskEnv_ClientPath(t *testing.T) {
	ci.Parallel(t)

	builder := testEnvBuilder()
	builder.SetAllocDir("/tmp/testAlloc")
	builder.SetClientSharedAllocDir("/tmp/testAlloc/alloc")
	builder.SetClientTaskRoot("/tmp/testAlloc/testTask")
	builder.SetClientTaskLocalDir("/tmp/testAlloc/testTask/local")
	builder.SetClientTaskSecretsDir("/tmp/testAlloc/testTask/secrets")
	env := builder.Build()

	testCases := []struct {
		label        string
		input        string
		joinOnEscape bool
		escapes      bool
		expected     string
	}{
		{
			// this is useful behavior for exec-based tasks, allowing template or artifact
			// destination anywhere in the chroot
			label:        "join on escape if requested",
			input:        "/tmp",
			joinOnEscape: true,
			expected:     "/tmp/testAlloc/testTask/tmp",
			escapes:      false,
		},
		{
			// template source behavior does not perform unconditional join
			label:        "do not join on escape unless requested",
			input:        "/tmp",
			joinOnEscape: false,
			expected:     "/tmp",
			escapes:      true,
		},
		{
			// relative paths are always joined to the task root dir
			// escape from task root dir and shared alloc dir should be detected
			label:        "detect escape for relative paths",
			input:        "..",
			joinOnEscape: true,
			expected:     "/tmp/testAlloc",
			escapes:      true,
		},
		{
			// shared alloc dir should be available from ../alloc, for historical reasons
			// this is not an escape
			label:        "relative access to shared alloc dir",
			input:        "../alloc/somefile",
			joinOnEscape: true,
			expected:     "/tmp/testAlloc/alloc/somefile",
			escapes:      false,
		},
		{
			label:        "interpolate shared alloc dir",
			input:        "${NOMAD_ALLOC_DIR}/somefile",
			joinOnEscape: false,
			expected:     "/tmp/testAlloc/alloc/somefile",
			escapes:      false,
		},
		{
			label:        "interpolate task local dir",
			input:        "${NOMAD_TASK_DIR}/somefile",
			joinOnEscape: false,
			expected:     "/tmp/testAlloc/testTask/local/somefile",
			escapes:      false,
		},
		{
			label:        "interpolate task secrts dir",
			input:        "${NOMAD_SECRETS_DIR}/somefile",
			joinOnEscape: false,
			expected:     "/tmp/testAlloc/testTask/secrets/somefile",
			escapes:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.label, func(t *testing.T) {
			path, escapes := env.ClientPath(tc.input, tc.joinOnEscape)
			assert.Equal(t, tc.escapes, escapes, "escape check")
			assert.Equal(t, tc.expected, path, "interpolated path")
		})
	}
}
