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
	input := []string{
		"foo=bar",
		"BAZ=baM",
		"bar=emb=edded",      // This can be done in multiple OSes.
		"=ExitCode=00000000", // A Windows cmd.exe annoyance
	}
	env, err := ParseFromList(input)
	if err != nil {
		t.Fatalf("ParseFromList(%#v) failed: %v", input, err)
	}

	exp := map[string]string{
		"foo":       "bar",
		"BAZ":       "baM",
		"bar":       "emb=edded",
		"=ExitCode": "00000000",
	}

	if len(env) != len(exp) {
		t.Errorf("ParseFromList(%#v) has length %v; want %v", input, len(env), len(exp))
	}

	for k, v := range exp {
		if actV, ok := env[k]; !ok {
			t.Errorf("ParseFromList(%#v) doesn't contain expected %v", input, k)
		} else if actV != v {
			t.Errorf("ParseFromList(%#v) has incorrect value for %v; got %v; want %v", input, k, actV, v)
		}
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
