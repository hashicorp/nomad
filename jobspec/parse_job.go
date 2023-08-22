// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package jobspec

import (
	"fmt"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/mapstructure"
)

func parseJob(result *api.Job, list *ast.ObjectList) error {
	if len(list.Items) != 1 {
		return fmt.Errorf("only one 'job' block allowed")
	}
	list = list.Children()
	if len(list.Items) != 1 {
		return fmt.Errorf("'job' block missing name")
	}

	// Get our job object
	obj := list.Items[0]

	// Decode the full thing into a map[string]interface for ease
	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, obj.Val); err != nil {
		return err
	}
	delete(m, "constraint")
	delete(m, "affinity")
	delete(m, "meta")
	delete(m, "migrate")
	delete(m, "parameterized")
	delete(m, "periodic")
	delete(m, "reschedule")
	delete(m, "update")
	delete(m, "vault")
	delete(m, "spread")
	delete(m, "multiregion")

	// Set the ID and name to the object key
	result.ID = stringToPtr(obj.Keys[0].Token.Value().(string))
	result.Name = stringToPtr(*result.ID)

	// Decode the rest
	if err := mapstructure.WeakDecode(m, result); err != nil {
		return err
	}

	// Value should be an object
	var listVal *ast.ObjectList
	if ot, ok := obj.Val.(*ast.ObjectType); ok {
		listVal = ot.List
	} else {
		return fmt.Errorf("job '%s' value: should be an object", *result.ID)
	}

	// Check for invalid keys
	valid := []string{
		"all_at_once",
		"constraint",
		"affinity",
		"spread",
		"datacenters",
		"group",
		"id",
		"meta",
		"migrate",
		"name",
		"namespace",
		"parameterized",
		"periodic",
		"priority",
		"region",
		"reschedule",
		"task",
		"type",
		"update",
		"vault",
		"vault_token",
		"consul_token",
		"multiregion",
	}
	if err := checkHCLKeys(listVal, valid); err != nil {
		return multierror.Prefix(err, "job:")
	}

	// Parse constraints
	if o := listVal.Filter("constraint"); len(o.Items) > 0 {
		if err := parseConstraints(&result.Constraints, o); err != nil {
			return multierror.Prefix(err, "constraint ->")
		}
	}

	// Parse affinities
	if o := listVal.Filter("affinity"); len(o.Items) > 0 {
		if err := parseAffinities(&result.Affinities, o); err != nil {
			return multierror.Prefix(err, "affinity ->")
		}
	}

	// If we have an update strategy, then parse that
	if o := listVal.Filter("update"); len(o.Items) > 0 {
		if err := parseUpdate(&result.Update, o); err != nil {
			return multierror.Prefix(err, "update ->")
		}
	}

	// If we have a periodic definition, then parse that
	if o := listVal.Filter("periodic"); len(o.Items) > 0 {
		if err := parsePeriodic(&result.Periodic, o); err != nil {
			return multierror.Prefix(err, "periodic ->")
		}
	}

	// Parse spread
	if o := listVal.Filter("spread"); len(o.Items) > 0 {
		if err := parseSpread(&result.Spreads, o); err != nil {
			return multierror.Prefix(err, "spread ->")
		}
	}

	// If we have a parameterized definition, then parse that
	if o := listVal.Filter("parameterized"); len(o.Items) > 0 {
		if err := parseParameterizedJob(&result.ParameterizedJob, o); err != nil {
			return multierror.Prefix(err, "parameterized ->")
		}
	}

	// If we have a reschedule block, then parse that
	if o := listVal.Filter("reschedule"); len(o.Items) > 0 {
		if err := parseReschedulePolicy(&result.Reschedule, o); err != nil {
			return multierror.Prefix(err, "reschedule ->")
		}
	}

	// If we have a migration strategy, then parse that
	if o := listVal.Filter("migrate"); len(o.Items) > 0 {
		if err := parseMigrate(&result.Migrate, o); err != nil {
			return multierror.Prefix(err, "migrate ->")
		}
	}

	// If we have a multiregion block, then parse that
	if o := listVal.Filter("multiregion"); len(o.Items) > 0 {
		var mr api.Multiregion
		if err := parseMultiregion(&mr, o); err != nil {
			return multierror.Prefix(err, "multiregion ->")
		}
		result.Multiregion = &mr
	}

	// Parse out meta fields. These are in HCL as a list so we need
	// to iterate over them and merge them.
	if metaO := listVal.Filter("meta"); len(metaO.Items) > 0 {
		for _, o := range metaO.Elem().Items {
			var m map[string]interface{}
			if err := hcl.DecodeObject(&m, o.Val); err != nil {
				return err
			}
			if err := mapstructure.WeakDecode(m, &result.Meta); err != nil {
				return err
			}
		}
	}

	// If we have tasks outside, create TaskGroups for them
	if o := listVal.Filter("task"); len(o.Items) > 0 {
		var tasks []*api.Task
		if err := parseTasks(&tasks, o); err != nil {
			return multierror.Prefix(err, "task:")
		}

		result.TaskGroups = make([]*api.TaskGroup, len(tasks), len(tasks)*2)
		for i, t := range tasks {
			result.TaskGroups[i] = &api.TaskGroup{
				Name:  stringToPtr(t.Name),
				Tasks: []*api.Task{t},
			}
		}
	}

	// Parse the task groups
	if o := listVal.Filter("group"); len(o.Items) > 0 {
		if err := parseGroups(result, o); err != nil {
			return multierror.Prefix(err, "group:")
		}
	}

	// If we have a vault block, then parse that
	if o := listVal.Filter("vault"); len(o.Items) > 0 {
		jobVault := &api.Vault{
			Env:         boolToPtr(true),
			DisableFile: boolToPtr(false),
			ChangeMode:  stringToPtr("restart"),
		}

		if err := parseVault(jobVault, o); err != nil {
			return multierror.Prefix(err, "vault ->")
		}

		// Go through the task groups/tasks and if they don't have a Vault block, set it
		for _, tg := range result.TaskGroups {
			for _, task := range tg.Tasks {
				if task.Vault == nil {
					task.Vault = jobVault
				}
			}
		}
	}

	return nil
}

func parsePeriodic(result **api.PeriodicConfig, list *ast.ObjectList) error {
	list = list.Elem()
	if len(list.Items) > 1 {
		return fmt.Errorf("only one 'periodic' block allowed per job")
	}

	// Get our resource object
	o := list.Items[0]

	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, o.Val); err != nil {
		return err
	}

	// Check for invalid keys
	valid := []string{
		"enabled",
		"cron",
		"crons",
		"prohibit_overlap",
		"time_zone",
	}
	if err := checkHCLKeys(o.Val, valid); err != nil {
		return err
	}

	if value, ok := m["enabled"]; ok {
		enabled, err := parseBool(value)
		if err != nil {
			return fmt.Errorf("periodic.enabled should be set to true or false; %v", err)
		}
		m["Enabled"] = enabled
	}

	// If "cron" is provided, set the type to "cron" and store the spec.
	if cron, ok := m["cron"]; ok {
		m["SpecType"] = api.PeriodicSpecCron
		m["Spec"] = cron
	}

	// If "crons" is provided, set the type to "cron" and store the spec.
	if cron, ok := m["crons"]; ok {
		m["SpecType"] = api.PeriodicSpecCron
		m["Specs"] = cron
	}

	// Build the constraint
	var p api.PeriodicConfig
	if err := mapstructure.WeakDecode(m, &p); err != nil {
		return err
	}
	*result = &p
	return nil
}

func parseParameterizedJob(result **api.ParameterizedJobConfig, list *ast.ObjectList) error {
	list = list.Elem()
	if len(list.Items) > 1 {
		return fmt.Errorf("only one 'parameterized' block allowed per job")
	}

	// Get our resource object
	o := list.Items[0]

	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, o.Val); err != nil {
		return err
	}

	// Check for invalid keys
	valid := []string{
		"payload",
		"meta_required",
		"meta_optional",
	}
	if err := checkHCLKeys(o.Val, valid); err != nil {
		return err
	}

	// Build the parameterized job block
	var d api.ParameterizedJobConfig
	if err := mapstructure.WeakDecode(m, &d); err != nil {
		return err
	}

	*result = &d
	return nil
}
