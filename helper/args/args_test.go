package args

import (
	"fmt"
	"reflect"
	"testing"
)

const (
	ipKey     = "NOMAD_IP"
	ipVal     = "127.0.0.1"
	portKey   = "NOMAD_PORT_WEB"
	portVal   = ":80"
	periodKey = "NOMAD.PERIOD"
	periodVal = "period"
)

var (
	envVars = map[string]string{
		ipKey:     ipVal,
		portKey:   portVal,
		periodKey: periodVal,
	}
)

func TestArgs_ReplaceEnv_Invalid(t *testing.T) {
	input := "$FOO"
	exp := "$FOO"
	act := ReplaceEnv(input, envVars)

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("ReplaceEnv(%v, %v) returned %#v; want %#v", input, envVars, act, exp)
	}
}

func TestArgs_ReplaceEnv_Valid(t *testing.T) {
	input := fmt.Sprintf(`"$%v"!`, ipKey)
	exp := fmt.Sprintf("\"%s\"!", ipVal)
	act := ReplaceEnv(input, envVars)

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("ReplaceEnv(%v, %v) returned %#v; want %#v", input, envVars, act, exp)
	}
}

func TestArgs_ReplaceEnv_Period(t *testing.T) {
	input := fmt.Sprintf(`"$%v"!`, periodKey)
	exp := fmt.Sprintf("\"%s\"!", periodVal)
	act := ReplaceEnv(input, envVars)

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("ReplaceEnv(%v, %v) returned %#v; want %#v", input, envVars, act, exp)
	}
}

func TestArgs_ReplaceEnv_Chained(t *testing.T) {
	input := fmt.Sprintf("$%s$%s", ipKey, portKey)
	exp := fmt.Sprintf("%s%s", ipVal, portVal)
	act := ReplaceEnv(input, envVars)

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("ReplaceEnv(%v, %v) returned %#v; want %#v", input, envVars, act, exp)
	}
}
