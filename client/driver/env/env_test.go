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
	// Networks that tests can rely on
	networks = []*structs.NetworkResource{
		&structs.NetworkResource{
			IP:            "127.0.0.1",
			ReservedPorts: []structs.Port{{Label: "http", Value: 80}},
			DynamicPorts:  []structs.Port{{Label: "https", Value: 8080}},
		},
	}
	portMap = map[string]int{
		"https": 443,
	}
)

func testTaskEnvironment() *TaskEnvironment {
	n := mock.Node()
	n.Attributes = map[string]string{
		attrKey: attrVal,
	}
	n.Meta = map[string]string{
		metaKey: metaVal,
	}
	n.Name = nodeName
	n.NodeClass = nodeClass

	envvars := map[string]string{
		envOneKey: envOneVal,
		envTwoKey: envTwoVal,
	}
	return NewTaskEnvironment(n).SetEnvvars(envvars).Build()
}

func TestEnvironment_ParseAndReplace_Env(t *testing.T) {
	env := testTaskEnvironment()

	input := []string{fmt.Sprintf(`"${%v}"!`, envOneKey), fmt.Sprintf("${%s}${%s}", envOneKey, envTwoKey)}
	act := env.ParseAndReplace(input)
	exp := []string{fmt.Sprintf(`"%s"!`, envOneVal), fmt.Sprintf("%s%s", envOneVal, envTwoVal)}

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("ParseAndReplace(%v) returned %#v; want %#v", input, act, exp)
	}
}

func TestEnvironment_ParseAndReplace_Meta(t *testing.T) {
	input := []string{fmt.Sprintf("${%v%v}", nodeMetaPrefix, metaKey)}
	exp := []string{metaVal}
	env := testTaskEnvironment()
	act := env.ParseAndReplace(input)

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("ParseAndReplace(%v) returned %#v; want %#v", input, act, exp)
	}
}

func TestEnvironment_ParseAndReplace_Attr(t *testing.T) {
	input := []string{fmt.Sprintf("${%v%v}", nodeAttributePrefix, attrKey)}
	exp := []string{attrVal}
	env := testTaskEnvironment()
	act := env.ParseAndReplace(input)

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("ParseAndReplace(%v) returned %#v; want %#v", input, act, exp)
	}
}

func TestEnvironment_ParseAndReplace_Node(t *testing.T) {
	input := []string{fmt.Sprintf("${%v}", nodeNameKey), fmt.Sprintf("${%v}", nodeClassKey)}
	exp := []string{nodeName, nodeClass}
	env := testTaskEnvironment()
	act := env.ParseAndReplace(input)

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
	env := testTaskEnvironment()
	act := env.ParseAndReplace(input)

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("ParseAndReplace(%v) returned %#v; want %#v", input, act, exp)
	}
}

func TestEnvironment_ReplaceEnv_Mixed(t *testing.T) {
	input := fmt.Sprintf("${%v}${%v%v}", nodeNameKey, nodeAttributePrefix, attrKey)
	exp := fmt.Sprintf("%v%v", nodeName, attrVal)
	env := testTaskEnvironment()
	act := env.ReplaceEnv(input)

	if act != exp {
		t.Fatalf("ParseAndReplace(%v) returned %#v; want %#v", input, act, exp)
	}
}

func TestEnvironment_AsList(t *testing.T) {
	n := mock.Node()
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
	env := NewTaskEnvironment(n).
		SetNetworks(networks).
		SetPortMap(portMap).
		SetTaskMeta(map[string]string{"foo": "baz"}).
		SetAlloc(a).
		SetTaskName("taskA").Build()

	act := env.EnvList()
	exp := []string{
		"NOMAD_ADDR_http=127.0.0.1:80",
		"NOMAD_PORT_http=80",
		"NOMAD_IP_http=127.0.0.1",
		"NOMAD_ADDR_https=127.0.0.1:443",
		"NOMAD_PORT_https=443",
		"NOMAD_IP_https=127.0.0.1",
		"NOMAD_HOST_PORT_http=80",
		"NOMAD_HOST_PORT_https=8080",
		"NOMAD_META_FOO=baz",
		"NOMAD_META_foo=baz",
		"NOMAD_ADDR_web_main=192.168.0.100:5000",
		"NOMAD_ADDR_web_http=192.168.0.100:2000",
		"NOMAD_PORT_web_main=5000",
		"NOMAD_PORT_web_http=2000",
		"NOMAD_IP_web_main=192.168.0.100",
		"NOMAD_IP_web_http=192.168.0.100",
		"NOMAD_TASK_NAME=taskA",
		"NOMAD_ADDR_ssh_other=192.168.0.100:1234",
		"NOMAD_ADDR_ssh_ssh=192.168.0.100:22",
		"NOMAD_IP_ssh_other=192.168.0.100",
		"NOMAD_IP_ssh_ssh=192.168.0.100",
		"NOMAD_PORT_ssh_other=1234",
		"NOMAD_PORT_ssh_ssh=22",
		fmt.Sprintf("NOMAD_ALLOC_ID=%s", a.ID),
	}
	sort.Strings(act)
	sort.Strings(exp)
	if len(act) != len(exp) {
		t.Fatalf("wat: %d != %d", len(act), len(exp))
	}
	for i := range act {
		if act[i] != exp[i] {
			t.Errorf("%d %q != %q", i, act[i], exp[i])
		}
	}
}

func TestEnvironment_VaultToken(t *testing.T) {
	n := mock.Node()
	env := NewTaskEnvironment(n).SetVaultToken("123", false).Build()

	act := env.EnvList()
	if len(act) != 0 {
		t.Fatalf("Unexpected environment variables: %v", act)
	}

	env = env.SetVaultToken("123", true).Build()
	act = env.EnvList()
	exp := []string{"VAULT_TOKEN=123"}
	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("env.List() returned %v; want %v", act, exp)
	}
}

func TestEnvironment_ClearEnvvars(t *testing.T) {
	n := mock.Node()
	env := NewTaskEnvironment(n).
		SetNetworks(networks).
		SetPortMap(portMap).
		SetEnvvars(map[string]string{"foo": "baz", "bar": "bang"}).Build()

	act := env.EnvList()
	exp := []string{
		"NOMAD_ADDR_http=127.0.0.1:80",
		"NOMAD_PORT_http=80",
		"NOMAD_IP_http=127.0.0.1",
		"NOMAD_ADDR_https=127.0.0.1:443",
		"NOMAD_PORT_https=443",
		"NOMAD_IP_https=127.0.0.1",
		"NOMAD_HOST_PORT_http=80",
		"NOMAD_HOST_PORT_https=8080",
		"bar=bang",
		"foo=baz",
	}
	sort.Strings(act)
	sort.Strings(exp)
	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("env.List() returned %v; want %v", act, exp)
	}

	// Clear the environent variables.
	env.ClearEnvvars().Build()

	act = env.EnvList()
	exp = []string{
		"NOMAD_ADDR_http=127.0.0.1:80",
		"NOMAD_PORT_http=80",
		"NOMAD_IP_http=127.0.0.1",
		"NOMAD_ADDR_https=127.0.0.1:443",
		"NOMAD_PORT_https=443",
		"NOMAD_IP_https=127.0.0.1",
		"NOMAD_HOST_PORT_https=8080",
		"NOMAD_HOST_PORT_http=80",
	}
	sort.Strings(act)
	sort.Strings(exp)
	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("env.List() returned %v; want %v", act, exp)
	}
}

func TestEnvironment_Interpolate(t *testing.T) {
	env := testTaskEnvironment().
		SetEnvvars(map[string]string{"test": "${node.class}", "test2": "${attr.arch}"}).
		Build()

	act := env.EnvList()
	exp := []string{fmt.Sprintf("test=%s", nodeClass), fmt.Sprintf("test2=%s", attrVal)}
	sort.Strings(act)
	sort.Strings(exp)
	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("env.List() returned %v; want %v", act, exp)
	}
}

func TestEnvironment_AppendHostEnvvars(t *testing.T) {
	host := os.Environ()
	if len(host) < 2 {
		t.Skip("No host environment variables. Can't test")
	}
	skip := strings.Split(host[0], "=")[0]
	env := testTaskEnvironment().
		AppendHostEnvvars([]string{skip}).
		Build()

	act := env.EnvMap()
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
	env := testTaskEnvironment()
	env.SetNetworks([]*structs.NetworkResource{
		{
			Device: "eth0",
			DynamicPorts: []structs.Port{
				{
					Label: "just-some-dashes",
					Value: 9000,
				},
			},
		},
	})
	env.Build()

	if env.TaskEnv["NOMAD_PORT_just_some_dashes"] != "9000" {
		t.Fatalf("Expected NOMAD_PORT_just_some_dashes=9000 in TaskEnv; found:\n%#v", env.TaskEnv)
	}
}
