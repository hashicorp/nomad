package env

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
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
	env := testEnvBuilder()

	input := []string{fmt.Sprintf(`"${%v}"!`, envOneKey), fmt.Sprintf("${%s}${%s}", envOneKey, envTwoKey)}
	act := env.Build().ParseAndReplace(input)
	exp := []string{fmt.Sprintf(`"%s"!`, envOneVal), fmt.Sprintf("%s%s", envOneVal, envTwoVal)}

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("ParseAndReplace(%v) returned %#v; want %#v", input, act, exp)
	}
}

func TestEnvironment_ParseAndReplace_Meta(t *testing.T) {
	input := []string{fmt.Sprintf("${%v%v}", nodeMetaPrefix, metaKey)}
	exp := []string{metaVal}
	env := testEnvBuilder()
	act := env.Build().ParseAndReplace(input)

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("ParseAndReplace(%v) returned %#v; want %#v", input, act, exp)
	}
}

func TestEnvironment_ParseAndReplace_Attr(t *testing.T) {
	input := []string{fmt.Sprintf("${%v%v}", nodeAttributePrefix, attrKey)}
	exp := []string{attrVal}
	env := testEnvBuilder()
	act := env.Build().ParseAndReplace(input)

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("ParseAndReplace(%v) returned %#v; want %#v", input, act, exp)
	}
}

func TestEnvironment_ParseAndReplace_Node(t *testing.T) {
	input := []string{fmt.Sprintf("${%v}", nodeNameKey), fmt.Sprintf("${%v}", nodeClassKey)}
	exp := []string{nodeName, nodeClass}
	env := testEnvBuilder()
	act := env.Build().ParseAndReplace(input)

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("ParseAndReplace(%v) returned %#v; want %#v", input, act, exp)
	}
}

func TestEnvironment_ParseAndReplace_Mixed(t *testing.T) {
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
	input := fmt.Sprintf("${%v}${%v%v}", nodeNameKey, nodeAttributePrefix, attrKey)
	exp := fmt.Sprintf("%v%v", nodeName, attrVal)
	env := testEnvBuilder()
	act := env.Build().ReplaceEnv(input)

	if act != exp {
		t.Fatalf("ParseAndReplace(%v) returned %#v; want %#v", input, act, exp)
	}
}

func TestEnvironment_AsList(t *testing.T) {
	n := mock.Node()
	n.Meta = map[string]string{
		"metaKey": "metaVal",
	}
	a := mock.Alloc()
	a.Resources.Networks[0].ReservedPorts = append(a.Resources.Networks[0].ReservedPorts,
		structs.Port{Label: "ssh", Value: 22},
		structs.Port{Label: "other", Value: 1234},
	)
	a.TaskResources["web"].Networks[0].DynamicPorts[0].Value = 2000
	a.TaskResources["ssh"] = &structs.Resources{
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
	task := a.Job.TaskGroups[0].Tasks[0]
	task.Env = map[string]string{
		"taskEnvKey": "taskEnvVal",
	}
	task.Resources.Networks = []*structs.NetworkResource{
		&structs.NetworkResource{
			IP:            "127.0.0.1",
			ReservedPorts: []structs.Port{{Label: "http", Value: 80}},
			DynamicPorts:  []structs.Port{{Label: "https", Value: 8080}},
		},
	}
	env := NewBuilder(n, a, task, "global").SetPortMap(map[string]int{"https": 443})

	act := env.Build().List()
	exp := []string{
		"taskEnvKey=taskEnvVal",
		"NOMAD_ADDR_http=127.0.0.1:80",
		"NOMAD_PORT_http=80",
		"NOMAD_IP_http=127.0.0.1",
		"NOMAD_ADDR_https=127.0.0.1:443",
		"NOMAD_PORT_https=443",
		"NOMAD_IP_https=127.0.0.1",
		"NOMAD_HOST_PORT_http=80",
		"NOMAD_HOST_PORT_https=8080",
		"NOMAD_TASK_NAME=web",
		"NOMAD_ADDR_ssh_other=192.168.0.100:1234",
		"NOMAD_ADDR_ssh_ssh=192.168.0.100:22",
		"NOMAD_IP_ssh_other=192.168.0.100",
		"NOMAD_IP_ssh_ssh=192.168.0.100",
		"NOMAD_PORT_ssh_other=1234",
		"NOMAD_PORT_ssh_ssh=22",
		"NOMAD_CPU_LIMIT=500",
		"NOMAD_REGION=global",
		"NOMAD_MEMORY_LIMIT=256",
		"NOMAD_JOB_NAME=my-job",
		fmt.Sprintf("NOMAD_ALLOC_ID=%s", a.ID),
	}
	sort.Strings(act)
	sort.Strings(exp)
	if len(act) != len(exp) {
		t.Fatalf("wat: %d != %d, actual: %s\n\nexpected: %s\n",
			len(act), len(exp), strings.Join(act, "\n"), strings.Join(exp, "\n"))
	}
	for i := range act {
		if act[i] != exp[i] {
			t.Errorf("%d actual %q != %q expected", i, act[i], exp[i])
		}
	}
}

func TestEnvironment_VaultToken(t *testing.T) {
	n := mock.Node()
	a := mock.Alloc()
	env := NewBuilder(n, a, a.Job.TaskGroups[0].Tasks[0], "global")
	env.SetVaultToken("123", false)

	{
		act := env.Build().All()
		if act[VaultToken] != "" {
			t.Fatalf("Unexpected environment variables: %s=%q", VaultToken, act[VaultToken])
		}
	}

	{
		act := env.SetVaultToken("123", true).Build().List()
		exp := "VAULT_TOKEN=123"
		found := false
		for _, entry := range act {
			if entry == exp {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("did not find %q in:\n%s", exp, strings.Join(act, "\n"))
		}
	}
}

func TestEnvironment_Envvars(t *testing.T) {
	envMap := map[string]string{"foo": "baz", "bar": "bang"}
	n := mock.Node()
	a := mock.Alloc()
	task := a.Job.TaskGroups[0].Tasks[0]
	task.Env = envMap
	act := NewBuilder(n, a, task, "global").SetPortMap(portMap).Build().All()
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

func TestEnvironment_Interpolate(t *testing.T) {
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
	a := mock.Alloc()
	task := a.Job.TaskGroups[0].Tasks[0]
	task.Env = map[string]string{"test-one-two": "three-four"}
	envMap := NewBuilder(mock.Node(), a, task, "global").Build().Map()

	if envMap["test_one_two"] != "three-four" {
		t.Fatalf("Expected test_one_two=three-four in TaskEnv; found:\n%#v", envMap)
	}
}
