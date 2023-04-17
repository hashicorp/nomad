// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package structs

// StatObject is a collection of statistics either exposed at the top
// level or via nested StatObjects.
type StatObject struct {
	// Nested is a mapping of object name to a nested stats object.
	Nested map[string]*StatObject

	// Attributes is a mapping of statistic name to its value.
	Attributes map[string]*StatValue
}

// StatValue exposes the values of a particular statistic. The value may be of
// type float, integer, string or boolean. Numeric types can be exposed as a
// single value or as a fraction.
type StatValue struct {
	// FloatNumeratorVal exposes a floating point value. If denominator is set
	// it is assumed to be a fractional value, otherwise it is a scalar.
	FloatNumeratorVal   *float64 `json:",omitempty"`
	FloatDenominatorVal *float64 `json:",omitempty"`

	// IntNumeratorVal exposes a int value. If denominator is set it is assumed
	// to be a fractional value, otherwise it is a scalar.
	IntNumeratorVal   *int64 `json:",omitempty"`
	IntDenominatorVal *int64 `json:",omitempty"`

	// StringVal exposes a string value. These are likely annotations.
	StringVal *string `json:",omitempty"`

	// BoolVal exposes a boolean statistic.
	BoolVal *bool `json:",omitempty"`

	// Unit gives the unit type: °F, %, MHz, MB, etc.
	Unit string `json:",omitempty"`

	// Desc provides a human readable description of the statistic.
	Desc string `json:",omitempty"`
}
