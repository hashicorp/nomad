// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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
	dashKey   = "NOMAD-DASH"
	dashVal   = "dash"
)

var (
	envVars = map[string]string{
		ipKey:     ipVal,
		portKey:   portVal,
		periodKey: periodVal,
		dashKey:   dashVal,
	}
)

func TestArgs_ReplaceEnv_Invalid(t *testing.T) {
	input := "${FOO}"
	exp := input
	act := ReplaceEnv(input, envVars)

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("ReplaceEnv(%v, %v) returned %#v; want %#v", input, envVars, act, exp)
	}
}

func TestArgs_ReplaceEnv_Valid(t *testing.T) {
	input := fmt.Sprintf(`"${%v}"!`, ipKey)
	exp := fmt.Sprintf("\"%s\"!", ipVal)
	act := ReplaceEnv(input, envVars)

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("ReplaceEnv(%v, %v) returned %#v; want %#v", input, envVars, act, exp)
	}
}

func TestArgs_ReplaceEnv_Period(t *testing.T) {
	input := fmt.Sprintf(`"${%v}"!`, periodKey)
	exp := fmt.Sprintf("\"%s\"!", periodVal)
	act := ReplaceEnv(input, envVars)

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("ReplaceEnv(%v, %v) returned %#v; want %#v", input, envVars, act, exp)
	}
}

func TestArgs_ReplaceEnv_Dash(t *testing.T) {
	input := fmt.Sprintf(`"${%v}"!`, dashKey)
	exp := fmt.Sprintf("\"%s\"!", dashVal)
	act := ReplaceEnv(input, envVars)

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("ReplaceEnv(%v, %v) returned %#v; want %#v", input, envVars, act, exp)
	}
}

func TestArgs_ReplaceEnv_Chained(t *testing.T) {
	input := fmt.Sprintf("${%s}${%s}", ipKey, portKey)
	exp := fmt.Sprintf("%s%s", ipVal, portVal)
	act := ReplaceEnv(input, envVars)

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("ReplaceEnv(%v, %v) returned %#v; want %#v", input, envVars, act, exp)
	}
}

func TestArgs_ContainsEnv(t *testing.T) {
	positiveCases := []string{
		"test-${env_var}",
	}
	for _, c := range positiveCases {
		t.Run(fmt.Sprintf("positive case: %v", c), func(t *testing.T) {
			if !ContainsEnv(c) {
				t.Fatalf("ContainsEnv(%v) returned false; want true", c)
			}
		})
	}

	negativeCases := []string{
		"test",
		"test-$",
		"test-${asdf",
		"test-{asdf}",
		"$test",
	}
	for _, c := range negativeCases {
		t.Run(fmt.Sprintf("negative case: %v", c), func(t *testing.T) {
			if ContainsEnv(c) {
				t.Fatalf("ContainsEnv(%v) returned true; want false", c)
			}
		})
	}

}
