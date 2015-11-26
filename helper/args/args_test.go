package args

import (
	"fmt"
	"reflect"
	"testing"
)

const (
	ipKey   = "NOMAD_IP"
	ipVal   = "127.0.0.1"
	portKey = "NOMAD_PORT_WEB"
	portVal = ":80"
)

var (
	envVars = map[string]string{
		ipKey:   ipVal,
		portKey: portVal,
	}
)

func TestDriverArgs_ParseAndReplaceInvalidEnv(t *testing.T) {
	input := []string{"invalid", "$FOO"}
	exp := []string{"invalid", "$FOO"}
	act := ParseAndReplace(input, envVars)

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("ParseAndReplace(%v, %v) returned %#v; want %#v", input, envVars, act, exp)
	}
}

func TestDriverArgs_ParseAndReplaceValidEnv(t *testing.T) {
	input := []string{"nomad_ip", fmt.Sprintf(`"$%v"!`, ipKey)}
	exp := []string{"nomad_ip", fmt.Sprintf("\"%s\"!", ipVal)}
	act := ParseAndReplace(input, envVars)

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("ParseAndReplace(%v, %v) returned %#v; want %#v", input, envVars, act, exp)
	}
}

func TestDriverArgs_ParseAndReplaceChainedEnv(t *testing.T) {
	input := []string{"-foo", fmt.Sprintf("$%s$%s", ipKey, portKey)}
	exp := []string{"-foo", fmt.Sprintf("%s%s", ipVal, portVal)}
	act := ParseAndReplace(input, envVars)

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("ParseAndReplace(%v, %v) returned %#v; want %#v", input, envVars, act, exp)
	}
}
