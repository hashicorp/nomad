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

func parseGroups(result *api.Job, list *ast.ObjectList) error {
	list = list.Children()
	if len(list.Items) == 0 {
		return nil
	}

	// Go through each object and turn it into an actual result.
	collection := make([]*api.TaskGroup, 0, len(list.Items))
	seen := make(map[string]struct{})
	for _, item := range list.Items {
		n := item.Keys[0].Token.Value().(string)

		// Make sure we haven't already found this
		if _, ok := seen[n]; ok {
			return fmt.Errorf("group '%s' defined more than once", n)
		}
		seen[n] = struct{}{}

		// We need this later
		var listVal *ast.ObjectList
		if ot, ok := item.Val.(*ast.ObjectType); ok {
			listVal = ot.List
		} else {
			return fmt.Errorf("group '%s': should be an object", n)
		}

		// Check for invalid keys
		valid := []string{
			"count",
			"constraint",
			"consul",
			"affinity",
			"restart",
			"meta",
			"task",
			"ephemeral_disk",
			"update",
			"reschedule",
			"vault",
			"migrate",
			"spread",
			"shutdown_delay",
			"network",
			"service",
			"volume",
			"scaling",
			"stop_after_client_disconnect",
			"max_client_disconnect",
		}
		if err := checkHCLKeys(listVal, valid); err != nil {
			return multierror.Prefix(err, fmt.Sprintf("'%s' ->", n))
		}

		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, item.Val); err != nil {
			return err
		}

		delete(m, "constraint")
		delete(m, "consul")
		delete(m, "affinity")
		delete(m, "meta")
		delete(m, "task")
		delete(m, "restart")
		delete(m, "ephemeral_disk")
		delete(m, "update")
		delete(m, "vault")
		delete(m, "migrate")
		delete(m, "spread")
		delete(m, "network")
		delete(m, "service")
		delete(m, "volume")
		delete(m, "scaling")

		// Build the group with the basic decode
		var g api.TaskGroup
		g.Name = stringToPtr(n)
		dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
			WeaklyTypedInput: true,
			Result:           &g,
		})

		if err != nil {
			return err
		}
		if err := dec.Decode(m); err != nil {
			return err
		}

		// Parse constraints
		if o := listVal.Filter("constraint"); len(o.Items) > 0 {
			if err := parseConstraints(&g.Constraints, o); err != nil {
				return multierror.Prefix(err, fmt.Sprintf("'%s', constraint ->", n))
			}
		}

		// Parse consul
		if o := listVal.Filter("consul"); len(o.Items) > 0 {
			if err := parseConsul(&g.Consul, o); err != nil {
				return multierror.Prefix(err, fmt.Sprintf("'%s', consul ->", n))
			}
		}

		// Parse affinities
		if o := listVal.Filter("affinity"); len(o.Items) > 0 {
			if err := parseAffinities(&g.Affinities, o); err != nil {
				return multierror.Prefix(err, fmt.Sprintf("'%s', affinity ->", n))
			}
		}

		// Parse restart policy
		if o := listVal.Filter("restart"); len(o.Items) > 0 {
			if err := parseRestartPolicy(&g.RestartPolicy, o); err != nil {
				return multierror.Prefix(err, fmt.Sprintf("'%s', restart ->", n))
			}
		}

		// Parse spread
		if o := listVal.Filter("spread"); len(o.Items) > 0 {
			if err := parseSpread(&g.Spreads, o); err != nil {
				return multierror.Prefix(err, "spread ->")
			}
		}

		// Parse network
		if o := listVal.Filter("network"); len(o.Items) > 0 {
			networks, err := ParseNetwork(o)
			if err != nil {
				return err
			}
			g.Networks = []*api.NetworkResource{networks}
		}

		// Parse reschedule policy
		if o := listVal.Filter("reschedule"); len(o.Items) > 0 {
			if err := parseReschedulePolicy(&g.ReschedulePolicy, o); err != nil {
				return multierror.Prefix(err, fmt.Sprintf("'%s', reschedule ->", n))
			}
		}
		// Parse ephemeral disk
		if o := listVal.Filter("ephemeral_disk"); len(o.Items) > 0 {
			g.EphemeralDisk = &api.EphemeralDisk{}
			if err := parseEphemeralDisk(&g.EphemeralDisk, o); err != nil {
				return multierror.Prefix(err, fmt.Sprintf("'%s', ephemeral_disk ->", n))
			}
		}

		// If we have an update strategy, then parse that
		if o := listVal.Filter("update"); len(o.Items) > 0 {
			if err := parseUpdate(&g.Update, o); err != nil {
				return multierror.Prefix(err, "update ->")
			}
		}

		// If we have a migration strategy, then parse that
		if o := listVal.Filter("migrate"); len(o.Items) > 0 {
			if err := parseMigrate(&g.Migrate, o); err != nil {
				return multierror.Prefix(err, "migrate ->")
			}
		}

		// Parse out meta fields. These are in HCL as a list so we need
		// to iterate over them and merge them.
		if metaO := listVal.Filter("meta"); len(metaO.Items) > 0 {
			for _, o := range metaO.Elem().Items {
				var m map[string]interface{}
				if err := hcl.DecodeObject(&m, o.Val); err != nil {
					return err
				}
				if err := mapstructure.WeakDecode(m, &g.Meta); err != nil {
					return err
				}
			}
		}

		// Parse any volume declarations
		if o := listVal.Filter("volume"); len(o.Items) > 0 {
			if err := parseVolumes(&g.Volumes, o); err != nil {
				return multierror.Prefix(err, "volume ->")
			}
		}

		// Parse scaling policy
		if o := listVal.Filter("scaling"); len(o.Items) > 0 {
			if err := parseGroupScalingPolicy(&g.Scaling, o); err != nil {
				return multierror.Prefix(err, "scaling ->")
			}
		}

		// Parse tasks
		if o := listVal.Filter("task"); len(o.Items) > 0 {
			if err := parseTasks(&g.Tasks, o); err != nil {
				return multierror.Prefix(err, fmt.Sprintf("'%s', task:", n))
			}
		}

		// If we have a vault block, then parse that
		if o := listVal.Filter("vault"); len(o.Items) > 0 {
			tgVault := &api.Vault{
				Env:         boolToPtr(true),
				DisableFile: boolToPtr(false),
				ChangeMode:  stringToPtr("restart"),
			}

			if err := parseVault(tgVault, o); err != nil {
				return multierror.Prefix(err, fmt.Sprintf("'%s', vault ->", n))
			}

			// Go through the tasks and if they don't have a Vault block, set it
			for _, task := range g.Tasks {
				if task.Vault == nil {
					task.Vault = tgVault
				}
			}
		}

		if o := listVal.Filter("service"); len(o.Items) > 0 {
			if err := parseGroupServices(&g, o); err != nil {
				return multierror.Prefix(err, fmt.Sprintf("'%s',", n))
			}
		}
		collection = append(collection, &g)
	}

	result.TaskGroups = append(result.TaskGroups, collection...)
	return nil
}

func parseConsul(result **api.Consul, list *ast.ObjectList) error {
	list = list.Elem()
	if len(list.Items) > 1 {
		return fmt.Errorf("only one 'consul' block allowed")
	}

	// Get our consul object
	obj := list.Items[0]

	// Check for invalid keys
	valid := []string{
		"namespace",
	}
	if err := checkHCLKeys(obj.Val, valid); err != nil {
		return err
	}

	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, obj.Val); err != nil {
		return err
	}

	var consul api.Consul
	if err := mapstructure.WeakDecode(m, &consul); err != nil {
		return err
	}
	*result = &consul

	return nil
}

func parseEphemeralDisk(result **api.EphemeralDisk, list *ast.ObjectList) error {
	list = list.Elem()
	if len(list.Items) > 1 {
		return fmt.Errorf("only one 'ephemeral_disk' block allowed")
	}

	// Get our ephemeral_disk object
	obj := list.Items[0]

	// Check for invalid keys
	valid := []string{
		"sticky",
		"size",
		"migrate",
	}
	if err := checkHCLKeys(obj.Val, valid); err != nil {
		return err
	}

	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, obj.Val); err != nil {
		return err
	}

	var ephemeralDisk api.EphemeralDisk
	if err := mapstructure.WeakDecode(m, &ephemeralDisk); err != nil {
		return err
	}
	*result = &ephemeralDisk

	return nil
}

func parseRestartPolicy(final **api.RestartPolicy, list *ast.ObjectList) error {
	list = list.Elem()
	if len(list.Items) > 1 {
		return fmt.Errorf("only one 'restart' block allowed")
	}

	// Get our job object
	obj := list.Items[0]

	// Check for invalid keys
	valid := []string{
		"attempts",
		"interval",
		"delay",
		"mode",
		"render_templates",
	}
	if err := checkHCLKeys(obj.Val, valid); err != nil {
		return err
	}

	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, obj.Val); err != nil {
		return err
	}

	var result api.RestartPolicy
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

func parseVolumes(out *map[string]*api.VolumeRequest, list *ast.ObjectList) error {
	hcl.DecodeObject(out, list)

	for k, v := range *out {
		err := unusedKeys(v)
		if err != nil {
			return err
		}
		// This is supported by `hcl:",key"`, but that only works if we start at the
		// parent ast.ObjectItem
		v.Name = k
	}

	return nil
}

func parseGroupScalingPolicy(out **api.ScalingPolicy, list *ast.ObjectList) error {
	if len(list.Items) > 1 {
		return fmt.Errorf("only one 'scaling' block allowed")
	}
	item := list.Items[0]
	if len(item.Keys) != 0 {
		return fmt.Errorf("task group scaling policy should not have a name")
	}
	p, err := parseScalingPolicy(item)
	if err != nil {
		return err
	}

	// group-specific validation
	if p.Max == nil {
		return fmt.Errorf("missing 'max'")
	}
	if p.Type == "" {
		p.Type = "horizontal"
	} else if p.Type != "horizontal" {
		return fmt.Errorf("task group scaling policy had invalid type: %q", p.Type)
	}
	*out = p
	return nil
}

func parseScalingPolicy(item *ast.ObjectItem) (*api.ScalingPolicy, error) {
	// We need this later
	var listVal *ast.ObjectList
	if ot, ok := item.Val.(*ast.ObjectType); ok {
		listVal = ot.List
	} else {
		return nil, fmt.Errorf("should be an object")
	}

	valid := []string{
		"min",
		"max",
		"policy",
		"enabled",
		"type",
	}
	if err := checkHCLKeys(item.Val, valid); err != nil {
		return nil, err
	}

	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, item.Val); err != nil {
		return nil, err
	}
	delete(m, "policy")

	var result api.ScalingPolicy
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		Result:           &result,
	})
	if err != nil {
		return nil, err
	}
	if err := dec.Decode(m); err != nil {
		return nil, err
	}

	// If we have policy, then parse that
	if o := listVal.Filter("policy"); len(o.Items) > 0 {
		if len(o.Elem().Items) > 1 {
			return nil, fmt.Errorf("only one 'policy' block allowed per 'scaling' block")
		}
		p := o.Elem().Items[0]
		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, p.Val); err != nil {
			return nil, err
		}
		if err := mapstructure.WeakDecode(m, &result.Policy); err != nil {
			return nil, err
		}
	}

	return &result, nil
}
