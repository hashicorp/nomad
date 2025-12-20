// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"math/rand"
	"slices"
	"strings"

	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/nomad/helper"
)

type CustomResource struct {
	// TODO(tgross): we need this old ",key" construction for HCL v1 parsing of
	// the client config; should this even be the same struct as the config?
	Name    string `hcl:",key"`
	Version uint64 // optional
	Type    CustomResourceType
	Scope   CustomResourceScope

	Quantity int64  // for countable or capped-ratio
	Range    string // for dynamic or static
	Items    []any  // for dynamic or static
	Meta     map[string]string

	Constraints []*Constraint
}

type CustomResourceScope string

const (
	CustomResourceScopeGroup CustomResourceScope = "group"
	CustomResourceScopeTask  CustomResourceScope = "task"
)

type CustomResourceType string

const (
	CustomResourceTypeRatio           CustomResourceType = "ratio"        // ex. weight
	CustomResourceTypeCappedRatio     CustomResourceType = "capped-ratio" // ex. resource.cpu
	CustomResourceTypeCountable       CustomResourceType = "countable"    // ex. memory, disk
	CustomResourceTypeDynamicInstance CustomResourceType = "dynamic"      // ex. ports, cores
	CustomResourceTypeStaticInstance  CustomResourceType = "static"       // ex. ports, devices
)

// Copy returns a deep clone of the CustomResource
func (cr *CustomResource) Copy() *CustomResource {
	ncr := new(CustomResource)
	*ncr = *cr

	ncr.Items = slices.Clone(cr.Items)
	ncr.Meta = maps.Clone(cr.Meta)
	ncr.Constraints = helper.CopySlice(cr.Constraints)
	return ncr
}

func (cr *CustomResource) Validate() error {
	// TODO(tgross): what do we need to do to validate these?
	return nil
}

func (cr *CustomResource) Equal(or *CustomResource) bool {
	if cr.Name != or.Name ||
		cr.Quantity != or.Quantity ||
		cr.Type != or.Type ||
		cr.Scope != or.Scope ||
		cr.Range != or.Range ||
		len(cr.Items) != len(or.Items) ||
		len(cr.Constraints) != len(or.Constraints) ||
		!slices.Equal(cr.Items, or.Items) ||
		!maps.Equal(cr.Meta, or.Meta) ||
		!slices.EqualFunc(cr.Constraints, or.Constraints, func(l, r *Constraint) bool {
			return l.Equal(r)
		}) {
		return false
	}

	return true
}

var (
	ErrInvalidCustomResourceComparison = errors.New("custom resources could not be compared")
	ErrCustomResourceExhausted         = errors.New("custom resources exhausted")
	ErrSubtractFromNothing             = errors.New("custom resource request cannot be subtracted from non-existant base")
)

func (cr *CustomResource) Superset(other *CustomResource) (bool, string) {

	if cr == nil || other == nil {
		return false, fmt.Sprintf("custom resource: %s", cr.Name)
	}
	err := cr.compatible(other)
	if err != nil {
		return false, err.Error() // not ideal, but this matches the other Comparable APIs
	}

	switch cr.Type {
	case CustomResourceTypeRatio:
		// fallthrough: ratios always fit?

	case CustomResourceTypeCappedRatio, CustomResourceTypeCountable:
		if cr.Quantity-other.Quantity < 0 {
			return false, fmt.Sprintf("custom resource: %s", cr.Name)
		}

	case CustomResourceTypeDynamicInstance, CustomResourceTypeStaticInstance:
		items := set.From(cr.Items)
		if !items.ContainsSlice(other.Items) {
			return false, fmt.Sprintf("custom resource: %s", cr.Name)
		}
	}

	return true, ""
}

// Add mutates the CustomResource by the delta
func (cr *CustomResource) Add(delta *CustomResource) error {
	if cr == nil || delta == nil {
		return nil
	}
	err := cr.compatible(delta)
	if err != nil {
		return err
	}

	switch cr.Type {
	case CustomResourceTypeRatio:
		return nil // ratios don't sum up

	case CustomResourceTypeCappedRatio, CustomResourceTypeCountable:
		cr.Quantity += delta.Quantity

	case CustomResourceTypeDynamicInstance, CustomResourceTypeStaticInstance:
		items := set.From(cr.Items)
		cr.Items = items.Union(set.From(delta.Items)).Slice()
		if len(cr.Items) > 1 {
			slices.SortFunc(cr.Items, func(a, b any) int {
				switch a.(type) {
				case string:
					if _, ok := b.(string); !ok {
						return 0
					}
					return strings.Compare(a.(string), b.(string))
				case int:
					if _, ok := b.(int); !ok {
						return 0
					}
					return cmp.Compare(a.(int), b.(int))
				}
				return 0
			})
		}
	}

	return nil
}

// Subtract mutates the CustomResource by the delta
func (cr *CustomResource) Subtract(delta *CustomResource) error {
	if cr == nil || delta == nil {
		return nil
	}
	err := cr.compatible(delta)
	if err != nil {
		return err
	}

	switch cr.Type {
	case CustomResourceTypeRatio:
		return nil // ratios don't sum up

	case CustomResourceTypeCappedRatio, CustomResourceTypeCountable:
		quantity := cr.Quantity - delta.Quantity
		cr.Quantity = max(quantity, 0)

	case CustomResourceTypeDynamicInstance, CustomResourceTypeStaticInstance:
		items := set.From(cr.Items)
		items.RemoveSet(set.From(delta.Items))
		cr.Items = items.Slice()
	}

	return nil
}

func (cr *CustomResource) compatible(delta *CustomResource) error {
	// TODO(tgross): these are programmer errors, I think?
	if cr.Name != delta.Name {
		return fmt.Errorf("%w: resource names %q and %q mismatch",
			ErrInvalidCustomResourceComparison, cr.Name, delta.Name)
	}
	if cr.Version < delta.Version && delta.Version > 0 {
		// requests for version 0 (default) can be considered compatible with
		// any instance of the resource
		return fmt.Errorf("%w: resource request %d for %q is newer than available version %d",
			ErrInvalidCustomResourceComparison, delta.Version, delta.Name, cr.Version)
	}
	if cr.Type != delta.Type {
		return fmt.Errorf("%w: resource types %q and %q for %q are not the same",
			ErrInvalidCustomResourceComparison, cr.Type, delta.Type, delta.Name)
	}
	if cr.Scope != delta.Scope {
		return fmt.Errorf("%w: resource scopes %q and %q for %q are not the same",
			ErrInvalidCustomResourceComparison, cr.Scope, delta.Scope, delta.Name)
	}

	return nil
}

// CustomResources is a convenience wrapper around a slice of CustomResources
type CustomResources []*CustomResource

func (cr CustomResources) Copy() CustomResources {
	return helper.CopySlice(cr)
}

func (cr *CustomResources) Select(available CustomResources) error {
NEXT:
	for _, r := range *cr {
		for _, base := range available {
			if base.Name == r.Name && base.Version == r.Version {
				if r.Type != CustomResourceTypeDynamicInstance {
					if ok, exhausted := base.Superset(r); !ok {
						return fmt.Errorf("%w: %s", ErrCustomResourceExhausted, exhausted)
					}
				} else {
					// for dynamic instances, we need to select items for the
					// quantity and mutate the Items of this CustomResource with
					// the selected items
					if r.Quantity > int64(len(base.Items)) {
						return fmt.Errorf("%w: %s", ErrCustomResourceExhausted, r.Name)
					}

					// selecting randomly by shuffling and slicing can be
					// expensive, so this tries random picks similar to how we
					// do port selection if we know the ratio of quantity to
					// items is sparse. these values are selected by hunch, so
					// this is a place for fine-tuning for sure
					if len(base.Items) > 64 && r.Quantity < 8 {
						r.Items = make([]any, 0, r.Quantity)
						for int64(len(r.Items)) < r.Quantity {
							item := base.Items[rand.Intn(len(base.Items))]
							if !slices.Contains(r.Items, item) {
								r.Items = append(r.Items, item)
							}
							// TODO(tgross): should we cap attempts?
						}
					} else {
						baseItems := slices.Clone(base.Items)
						rand.Shuffle(len(baseItems), func(i, j int) {
							baseItems[i], baseItems[j] = baseItems[j], baseItems[i]
						})
						items := base.Items[:r.Quantity]
						r.Items = items
					}
				}

				continue NEXT
			}
		}
	}

	return nil
}

func (cr CustomResources) CopySharedOnly() CustomResources {
	out := make([]*CustomResource, 0, len(cr))
	for _, r := range cr {
		if r.Scope == CustomResourceScopeGroup {
			out = append(out, r)
		}
	}

	return out
}

func (cr CustomResources) CopyTaskOnly() CustomResources {
	out := make([]*CustomResource, 0, len(cr))
	for _, r := range cr {
		if r.Scope == CustomResourceScopeTask || r.Scope == "" {
			out = append(out, r)
		}
	}

	return out
}

// Merge combines other custom resources, overwriting the resources with
// matching names and versions
func (cr *CustomResources) Merge(other CustomResources) {
	out := *cr

NEXT:
	for _, ocr := range other {
		for i, base := range *cr {
			if ocr.Name == base.Name && ocr.Version == base.Version {
				out[i] = ocr
				continue NEXT
			}
		}
		out = append(out, ocr)
	}

	*cr = out
}

func (cr *CustomResources) Add(delta *CustomResources) error {
	if cr == nil || delta == nil {
		return nil
	}

	// it's possible to get an error after we've mutated one *CR in the slice,
	// so copy to make the function succeed entirely or not at all
	out := cr.Copy()
	seen := []int{}
NEXT:
	for _, r := range out {
		for i, o := range *delta {
			if r.Name == o.Name {
				seen = append(seen, i)
				err := r.Add(o)
				if err != nil {
					return err
				}
				continue NEXT
			}
		}
	}
	for i, o := range *delta {
		if !slices.Contains(seen, i) {
			out = append(out, o)
		}
	}
	*cr = out
	return nil
}

func (cr CustomResources) Superset(other CustomResources) (bool, string) {
	if other == nil {
		return true, ""
	}
	if cr == nil {
		return false, ErrInvalidCustomResourceComparison.Error()
	}
	for _, ocr := range other {
		for _, base := range cr {
			if ocr.Name != base.Name {
				continue
			}
			if ok, name := base.Superset(ocr); !ok {
				return false, name
			}
		}
	}

	return true, ""
}

func (cr *CustomResources) Subtract(delta *CustomResources) error {
	if cr == nil || delta == nil {
		return nil
	}

	// it's possible to get an error after we've mutated one *CR in the slice,
	// so copy to make the function succeed entirely or not at all
	out := cr.Copy()
	seen := []int{}
NEXT:
	for _, r := range out {
		for i, o := range *delta {
			if r.Name == o.Name {
				seen = append(seen, i)
				err := r.Subtract(o)
				if err != nil {
					return err
				}
				continue NEXT
			}
		}
	}
	for i := range *delta {
		if !slices.Contains(seen, i) {
			return ErrSubtractFromNothing
		}
	}
	*cr = out
	return nil
}
