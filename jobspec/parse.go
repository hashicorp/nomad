// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package jobspec

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/mapstructure"
)

var reDynamicPorts = regexp.MustCompile("^[a-zA-Z0-9_]+$")
var errPortLabel = fmt.Errorf("Port label does not conform to naming requirements %s", reDynamicPorts.String())

// Parse parses the job spec from the given io.Reader.
//
// Due to current internal limitations, the entire contents of the
// io.Reader will be copied into memory first before parsing.
func Parse(r io.Reader) (*api.Job, error) {
	// Copy the reader into an in-memory buffer first since HCL requires it.
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return nil, err
	}

	// Parse the buffer
	root, err := hcl.Parse(buf.String())
	if err != nil {
		return nil, fmt.Errorf("error parsing: %s", err)
	}
	buf.Reset()

	// Top-level item should be a list
	list, ok := root.Node.(*ast.ObjectList)
	if !ok {
		return nil, fmt.Errorf("error parsing: root should be an object")
	}

	// Check for invalid keys
	valid := []string{
		"job",
	}
	if err := checkHCLKeys(list, valid); err != nil {
		return nil, err
	}

	var job api.Job

	// Parse the job out
	matches := list.Filter("job")
	if len(matches.Items) == 0 {
		return nil, fmt.Errorf("'job' block not found")
	}
	if err := parseJob(&job, matches); err != nil {
		return nil, fmt.Errorf("error parsing 'job': %s", err)
	}

	return &job, nil
}

// ParseFile parses the given path as a job spec.
func ParseFile(path string) (*api.Job, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return Parse(f)
}

func parseReschedulePolicy(final **api.ReschedulePolicy, list *ast.ObjectList) error {
	list = list.Elem()
	if len(list.Items) > 1 {
		return fmt.Errorf("only one 'reschedule' block allowed")
	}

	// Get our job object
	obj := list.Items[0]

	// Check for invalid keys
	valid := []string{
		"attempts",
		"interval",
		"unlimited",
		"delay",
		"max_delay",
		"delay_function",
	}
	if err := checkHCLKeys(obj.Val, valid); err != nil {
		return err
	}

	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, obj.Val); err != nil {
		return err
	}

	var result api.ReschedulePolicy
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
		WeaklyTypedInput: true,
		Result:           &result,
	})
	if err != nil {
		return err
	}
	if err := dec.Decode(m); err != nil {
		return err
	}

	*final = &result
	return nil
}

func parseConstraints(result *[]*api.Constraint, list *ast.ObjectList) error {
	for _, o := range list.Elem().Items {
		// Check for invalid keys
		valid := []string{
			"attribute",
			"distinct_hosts",
			"distinct_property",
			"operator",
			"regexp",
			"set_contains",
			"value",
			"version",
			"semver",
		}
		if err := checkHCLKeys(o.Val, valid); err != nil {
			return err
		}

		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, o.Val); err != nil {
			return err
		}

		m["LTarget"] = m["attribute"]
		m["RTarget"] = m["value"]
		m["Operand"] = m["operator"]

		// If "version" is provided, set the operand
		// to "version" and the value to the "RTarget"
		if constraint, ok := m[api.ConstraintVersion]; ok {
			m["Operand"] = api.ConstraintVersion
			m["RTarget"] = constraint
		}

		// If "semver" is provided, set the operand
		// to "semver" and the value to the "RTarget"
		if constraint, ok := m[api.ConstraintSemver]; ok {
			m["Operand"] = api.ConstraintSemver
			m["RTarget"] = constraint
		}

		// If "regexp" is provided, set the operand
		// to "regexp" and the value to the "RTarget"
		if constraint, ok := m[api.ConstraintRegex]; ok {
			m["Operand"] = api.ConstraintRegex
			m["RTarget"] = constraint
		}

		// If "set_contains" is provided, set the operand
		// to "set_contains" and the value to the "RTarget"
		if constraint, ok := m[api.ConstraintSetContains]; ok {
			m["Operand"] = api.ConstraintSetContains
			m["RTarget"] = constraint
		}

		if value, ok := m[api.ConstraintDistinctHosts]; ok {
			enabled, err := parseBool(value)
			if err != nil {
				return fmt.Errorf("distinct_hosts should be set to true or false; %v", err)
			}

			// If it is not enabled, skip the constraint.
			if !enabled {
				continue
			}

			m["Operand"] = api.ConstraintDistinctHosts
			m["RTarget"] = strconv.FormatBool(enabled)
		}

		if property, ok := m[api.ConstraintDistinctProperty]; ok {
			m["Operand"] = api.ConstraintDistinctProperty
			m["LTarget"] = property
		}

		// Build the constraint
		var c api.Constraint
		if err := mapstructure.WeakDecode(m, &c); err != nil {
			return err
		}
		if c.Operand == "" {
			c.Operand = "="
		}

		*result = append(*result, &c)
	}

	return nil
}

func parseAffinities(result *[]*api.Affinity, list *ast.ObjectList) error {
	for _, o := range list.Elem().Items {
		// Check for invalid keys
		valid := []string{
			"attribute",
			"operator",
			"regexp",
			"set_contains",
			"set_contains_any",
			"set_contains_all",
			"value",
			"version",
			"semver",
			"weight",
		}
		if err := checkHCLKeys(o.Val, valid); err != nil {
			return err
		}

		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, o.Val); err != nil {
			return err
		}

		m["LTarget"] = m["attribute"]
		m["RTarget"] = m["value"]
		m["Operand"] = m["operator"]

		// If "version" is provided, set the operand
		// to "version" and the value to the "RTarget"
		if affinity, ok := m[api.ConstraintVersion]; ok {
			m["Operand"] = api.ConstraintVersion
			m["RTarget"] = affinity
		}

		// If "semver" is provided, set the operand
		// to "semver" and the value to the "RTarget"
		if affinity, ok := m[api.ConstraintSemver]; ok {
			m["Operand"] = api.ConstraintSemver
			m["RTarget"] = affinity
		}

		// If "regexp" is provided, set the operand
		// to "regexp" and the value to the "RTarget"
		if affinity, ok := m[api.ConstraintRegex]; ok {
			m["Operand"] = api.ConstraintRegex
			m["RTarget"] = affinity
		}

		// If "set_contains_any" is provided, set the operand
		// to "set_contains_any" and the value to the "RTarget"
		if affinity, ok := m[api.ConstraintSetContainsAny]; ok {
			m["Operand"] = api.ConstraintSetContainsAny
			m["RTarget"] = affinity
		}

		// If "set_contains_all" is provided, set the operand
		// to "set_contains_all" and the value to the "RTarget"
		if affinity, ok := m[api.ConstraintSetContainsAll]; ok {
			m["Operand"] = api.ConstraintSetContainsAll
			m["RTarget"] = affinity
		}

		// set_contains is a synonym of set_contains_all
		if affinity, ok := m[api.ConstraintSetContains]; ok {
			m["Operand"] = api.ConstraintSetContains
			m["RTarget"] = affinity
		}

		// Build the affinity
		var a api.Affinity
		if err := mapstructure.WeakDecode(m, &a); err != nil {
			return err
		}
		if a.Operand == "" {
			a.Operand = "="
		}

		*result = append(*result, &a)
	}

	return nil
}

func parseSpread(result *[]*api.Spread, list *ast.ObjectList) error {
	for _, o := range list.Elem().Items {
		// Check for invalid keys
		valid := []string{
			"attribute",
			"weight",
			"target",
		}
		if err := checkHCLKeys(o.Val, valid); err != nil {
			return err
		}

		// We need this later
		var listVal *ast.ObjectList
		if ot, ok := o.Val.(*ast.ObjectType); ok {
			listVal = ot.List
		} else {
			return fmt.Errorf("spread should be an object")
		}

		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, o.Val); err != nil {
			return err
		}
		delete(m, "target")
		// Build spread
		var s api.Spread
		if err := mapstructure.WeakDecode(m, &s); err != nil {
			return err
		}

		// Parse spread target
		if o := listVal.Filter("target"); len(o.Items) > 0 {
			if err := parseSpreadTarget(&s.SpreadTarget, o); err != nil {
				return multierror.Prefix(err, "target ->")
			}
		}

		*result = append(*result, &s)
	}

	return nil
}

func parseSpreadTarget(result *[]*api.SpreadTarget, list *ast.ObjectList) error {
	seen := make(map[string]struct{})
	for _, item := range list.Items {
		if len(item.Keys) != 1 {
			return fmt.Errorf("missing spread target")
		}
		n := item.Keys[0].Token.Value().(string)

		// Make sure we haven't already found this
		if _, ok := seen[n]; ok {
			return fmt.Errorf("target '%s' defined more than once", n)
		}
		seen[n] = struct{}{}

		// We need this later
		var listVal *ast.ObjectList
		if ot, ok := item.Val.(*ast.ObjectType); ok {
			listVal = ot.List
		} else {
			return fmt.Errorf("target should be an object")
		}

		// Check for invalid keys
		valid := []string{
			"percent",
			"value",
		}
		if err := checkHCLKeys(listVal, valid); err != nil {
			return multierror.Prefix(err, fmt.Sprintf("'%s' ->", n))
		}

		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, item.Val); err != nil {
			return err
		}

		// Decode spread target
		var g api.SpreadTarget
		g.Value = n
		if err := mapstructure.WeakDecode(m, &g); err != nil {
			return err
		}
		*result = append(*result, &g)
	}
	return nil
}

// parseBool takes an interface value and tries to convert it to a boolean and
// returns an error if the type can't be converted.
func parseBool(value interface{}) (bool, error) {
	var enabled bool
	var err error
	switch data := value.(type) {
	case string:
		enabled, err = strconv.ParseBool(data)
	case bool:
		enabled = data
	default:
		err = fmt.Errorf("%v couldn't be converted to boolean value", value)
	}

	return enabled, err
}

func parseUpdate(result **api.UpdateStrategy, list *ast.ObjectList) error {
	list = list.Elem()
	if len(list.Items) > 1 {
		return fmt.Errorf("only one 'update' block allowed")
	}

	// Get our resource object
	o := list.Items[0]

	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, o.Val); err != nil {
		return err
	}

	// Check for invalid keys
	valid := []string{
		"stagger",
		"max_parallel",
		"health_check",
		"min_healthy_time",
		"healthy_deadline",
		"progress_deadline",
		"auto_revert",
		"auto_promote",
		"canary",
	}
	if err := checkHCLKeys(o.Val, valid); err != nil {
		return err
	}

	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
		WeaklyTypedInput: true,
		Result:           result,
	})
	if err != nil {
		return err
	}
	return dec.Decode(m)
}

func parseMigrate(result **api.MigrateStrategy, list *ast.ObjectList) error {
	list = list.Elem()
	if len(list.Items) > 1 {
		return fmt.Errorf("only one 'migrate' block allowed")
	}

	// Get our resource object
	o := list.Items[0]

	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, o.Val); err != nil {
		return err
	}

	// Check for invalid keys
	valid := []string{
		"max_parallel",
		"health_check",
		"min_healthy_time",
		"healthy_deadline",
	}
	if err := checkHCLKeys(o.Val, valid); err != nil {
		return err
	}

	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
		WeaklyTypedInput: true,
		Result:           result,
	})
	if err != nil {
		return err
	}
	return dec.Decode(m)
}

func parseVault(result *api.Vault, list *ast.ObjectList) error {
	list = list.Elem()
	if len(list.Items) == 0 {
		return nil
	}
	if len(list.Items) > 1 {
		return fmt.Errorf("only one 'vault' block allowed per task")
	}

	// Get our resource object
	o := list.Items[0]

	// We need this later
	var listVal *ast.ObjectList
	if ot, ok := o.Val.(*ast.ObjectType); ok {
		listVal = ot.List
	} else {
		return fmt.Errorf("vault: should be an object")
	}

	// Check for invalid keys
	valid := []string{
		"namespace",
		"policies",
		"env",
		"change_mode",
		"change_signal",
	}
	if err := checkHCLKeys(listVal, valid); err != nil {
		return multierror.Prefix(err, "vault ->")
	}

	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, o.Val); err != nil {
		return err
	}

	if err := mapstructure.WeakDecode(m, result); err != nil {
		return err
	}

	return nil
}
