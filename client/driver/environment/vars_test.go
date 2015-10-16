package environment

import (
	"reflect"
	"sort"
	"testing"
)

func TestEnvironment_AsList(t *testing.T) {
	env := NewTaskEnivornment()
	env.SetTaskIp("127.0.0.1")
	env.SetPorts(map[string]int{"http": 80})
	env.SetMeta(map[string]string{"foo": "baz"})

	act := env.List()
	exp := []string{"NOMAD_IP=127.0.0.1", "NOMAD_PORT_http=80", "NOMAD_META_FOO=baz"}
	sort.Strings(act)
	sort.Strings(exp)
	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("env.List() returned %v; want %v", act, exp)
	}
}

func TestEnvironment_ParseFromList(t *testing.T) {
	input := []string{"foo=bar", "BAZ=baM"}
	env, err := ParseFromList(input)
	if err != nil {
		t.Fatalf("ParseFromList(%#v) failed: %v", input, err)
	}

	exp := map[string]string{
		"foo": "baz",
		"BAZ": "baM",
	}
	if !reflect.DeepEqual(env, exp) {
		t.Fatalf("ParseFromList(%#v) returned %v; want %v", input, env, exp)
	}
}

func TestEnvironment_ClearEnvvars(t *testing.T) {
	env := NewTaskEnivornment()
	env.SetTaskIp("127.0.0.1")
	env.SetEnvvars(map[string]string{"foo": "baz", "bar": "bang"})

	act := env.List()
	exp := []string{"NOMAD_IP=127.0.0.1", "bar=bang", "foo=baz"}
	sort.Strings(act)
	sort.Strings(exp)
	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("env.List() returned %v; want %v", act, exp)
	}

	// Clear the environent variables.
	env.ClearEnvvars()

	act = env.List()
	exp = []string{"NOMAD_IP=127.0.0.1"}
	sort.Strings(act)
	sort.Strings(exp)
	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("env.List() returned %v; want %v", act, exp)
	}
}
