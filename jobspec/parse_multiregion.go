package jobspec

import (
	"fmt"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/mitchellh/mapstructure"
)

func parseMultiregion(result *api.Multiregion, list *ast.ObjectList) error {

	list = list.Elem()
	if len(list.Items) > 1 {
		return fmt.Errorf("only one 'multiregion' block allowed")
	}
	if len(list.Items) == 0 {
		return nil
	}

	// Get our multiregion object and decode it
	obj := list.Items[0]
	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, obj.Val); err != nil {
		return err
	}

	// Value should be an object
	var listVal *ast.ObjectList
	if ot, ok := obj.Val.(*ast.ObjectType); ok {
		listVal = ot.List
	} else {
		return fmt.Errorf("multiregion should be an object")
	}

	// Check for invalid keys
	valid := []string{
		"strategy",
		"region",
	}
	if err := helper.CheckHCLKeys(obj.Val, valid); err != nil {
		return err
	}

	// If we have a strategy, then parse that
	if o := listVal.Filter("strategy"); len(o.Items) > 0 {
		if err := parseMultiregionStrategy(&result.Strategy, o); err != nil {
			return multierror.Prefix(err, "strategy ->")
		}
	}
	// If we have regions, then parse those
	if o := listVal.Filter("region"); len(o.Items) > 0 {
		if err := parseMultiregionRegions(result, o); err != nil {
			return multierror.Prefix(err, "regions ->")
		}
	} else {
		return fmt.Errorf("'multiregion' requires one or more 'region' blocks")
	}
	return nil
}

func parseMultiregionStrategy(final **api.MultiregionStrategy, list *ast.ObjectList) error {
	list = list.Elem()
	if len(list.Items) > 1 {
		return fmt.Errorf("only one 'strategy' block allowed")
	}

	// Get our job object
	obj := list.Items[0]

	// Check for invalid keys
	valid := []string{
		"max_parallel",
		"on_failure",
	}
	if err := helper.CheckHCLKeys(obj.Val, valid); err != nil {
		return err
	}

	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, obj.Val); err != nil {
		return err
	}

	var result api.MultiregionStrategy
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
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

func parseMultiregionRegions(result *api.Multiregion, list *ast.ObjectList) error {
	list = list.Children()
	if len(list.Items) == 0 {
		return nil
	}

	// Go through each object and turn it into an actual result.
	collection := make([]*api.MultiregionRegion, 0, len(list.Items))
	seen := make(map[string]struct{})
	for _, item := range list.Items {
		n := item.Keys[0].Token.Value().(string)

		// Make sure we haven't already found this
		if _, ok := seen[n]; ok {
			return fmt.Errorf("region '%s' defined more than once", n)
		}
		seen[n] = struct{}{}

		// We need this later
		var listVal *ast.ObjectList
		if ot, ok := item.Val.(*ast.ObjectType); ok {
			listVal = ot.List
		} else {
			return fmt.Errorf("region '%s': should be an object", n)
		}

		// Check for invalid keys
		valid := []string{
			"count",
			"datacenters",
			"meta",
		}
		if err := helper.CheckHCLKeys(listVal, valid); err != nil {
			return multierror.Prefix(err, fmt.Sprintf("'%s' ->", n))
		}

		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, item.Val); err != nil {
			return err
		}

		// Build the region with the basic decode
		var r api.MultiregionRegion
		r.Name = n
		dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			WeaklyTypedInput: true,
			Result:           &r,
		})
		if err != nil {
			return err
		}
		if err := dec.Decode(m); err != nil {
			return err
		}
		collection = append(collection, &r)
	}

	result.Regions = append(result.Regions, collection...)
	return nil
}
