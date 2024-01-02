// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"unicode"

	"github.com/hashicorp/nomad/helper/pointer"
)

const (
	// floatPrecision is the precision used before rounding. It is set to a high
	// number to give a high chance of correctly returning equality.
	floatPrecision = uint(256)
)

// BaseUnit is a unique base unit. All units that share the same base unit
// should be comparable.
type BaseUnit uint16

const (
	UnitScalar BaseUnit = iota
	UnitByte
	UnitByteRate
	UnitHertz
	UnitWatt
)

// Unit describes a unit and its multiplier over the base unit type
type Unit struct {
	// Name is the name of the unit (GiB, MB/s)
	Name string

	// Base is the base unit for the unit
	Base BaseUnit

	// Multiplier is the multiplier over the base unit (KiB multiplier is 1024)
	Multiplier int64

	// InverseMultiplier specifies that the multiplier is an inverse so:
	// Base / Multiplier. For example a mW is a W/1000.
	InverseMultiplier bool
}

// Comparable returns if two units are comparable
func (u *Unit) Comparable(o *Unit) bool {
	if u == nil || o == nil {
		return false
	}

	return u.Base == o.Base
}

// ParseAttribute takes a string and parses it into an attribute, pulling out
// units if they are specified as a suffix on a number.
func ParseAttribute(input string) *Attribute {
	ll := len(input)
	if ll == 0 {
		return &Attribute{String: pointer.Of(input)}
	}

	// Check if the string is a number ending with potential units
	var unit string
	numeric := input
	if unicode.IsLetter(rune(input[ll-1])) {
		// Try suffix matching
		for _, u := range lengthSortedUnits {
			if strings.HasSuffix(input, u) {
				unit = u
				break
			}
		}

		// Check if we know about the unit.
		if len(unit) != 0 {
			numeric = strings.TrimSpace(strings.TrimSuffix(input, unit))
		}
	}

	// Try to parse as an int
	i, err := strconv.ParseInt(numeric, 10, 64)
	if err == nil {
		return &Attribute{Int: pointer.Of(i), Unit: unit}
	}

	// Try to parse as a float
	f, err := strconv.ParseFloat(numeric, 64)
	if err == nil {
		return &Attribute{Float: pointer.Of(f), Unit: unit}
	}

	// Try to parse as a bool
	b, err := strconv.ParseBool(input)
	if err == nil {
		return &Attribute{Bool: pointer.Of(b)}
	}

	return &Attribute{String: pointer.Of(input)}
}

// Attribute is used to describe the value of an attribute, optionally
// specifying units
type Attribute struct {
	// Float is the float value for the attribute
	Float *float64

	// Int is the int value for the attribute
	Int *int64

	// String is the string value for the attribute
	String *string

	// Bool is the bool value for the attribute
	Bool *bool

	// Unit is the optional unit for the set int or float value
	Unit string
}

// NewStringAttribute returns a new string attribute.
func NewStringAttribute(s string) *Attribute {
	return &Attribute{
		String: pointer.Of(s),
	}
}

// NewBoolAttribute returns a new boolean attribute.
func NewBoolAttribute(b bool) *Attribute {
	return &Attribute{
		Bool: pointer.Of(b),
	}
}

// NewIntAttribute returns a new integer attribute. The unit is not checked
// to be valid.
func NewIntAttribute(i int64, unit string) *Attribute {
	return &Attribute{
		Int:  pointer.Of(i),
		Unit: unit,
	}
}

// NewFloatAttribute returns a new float attribute. The unit is not checked to
// be valid.
func NewFloatAttribute(f float64, unit string) *Attribute {
	return &Attribute{
		Float: pointer.Of(f),
		Unit:  unit,
	}
}

// GetString returns the string value of the attribute or false if the attribute
// doesn't contain a string.
func (a *Attribute) GetString() (value string, ok bool) {
	if a.String == nil {
		return "", false
	}

	return *a.String, true
}

// GetBool returns the boolean value of the attribute or false if the attribute
// doesn't contain a boolean.
func (a *Attribute) GetBool() (value bool, ok bool) {
	if a.Bool == nil {
		return false, false
	}

	return *a.Bool, true
}

// GetInt returns the integer value of the attribute or false if the attribute
// doesn't contain a integer.
func (a *Attribute) GetInt() (value int64, ok bool) {
	if a.Int == nil {
		return 0, false
	}

	return *a.Int, true
}

// GetFloat returns the float value of the attribute or false if the attribute
// doesn't contain a float.
func (a *Attribute) GetFloat() (value float64, ok bool) {
	if a.Float == nil {
		return 0.0, false
	}

	return *a.Float, true
}

// Copy returns a copied version of the attribute
func (a *Attribute) Copy() *Attribute {
	if a == nil {
		return nil
	}

	ca := &Attribute{
		Unit: a.Unit,
	}

	if a.Float != nil {
		ca.Float = pointer.Of(*a.Float)
	}
	if a.Int != nil {
		ca.Int = pointer.Of(*a.Int)
	}
	if a.Bool != nil {
		ca.Bool = pointer.Of(*a.Bool)
	}
	if a.String != nil {
		ca.String = pointer.Of(*a.String)
	}

	return ca
}

// GoString returns a string representation of the attribute
func (a *Attribute) GoString() string {
	if a == nil {
		return "nil attribute"
	}

	var b strings.Builder
	if a.Float != nil {
		b.WriteString(fmt.Sprintf("%v", *a.Float))
	} else if a.Int != nil {
		b.WriteString(fmt.Sprintf("%v", *a.Int))
	} else if a.Bool != nil {
		b.WriteString(fmt.Sprintf("%v", *a.Bool))
	} else if a.String != nil {
		b.WriteString(*a.String)
	}

	if a.Unit != "" {
		b.WriteString(a.Unit)
	}

	return b.String()
}

// Validate checks if the attribute is valid
func (a *Attribute) Validate() error {
	if a.Unit != "" {
		if _, ok := UnitIndex[a.Unit]; !ok {
			return fmt.Errorf("unrecognized unit %q", a.Unit)
		}

		// Check only int/float set
		if a.String != nil || a.Bool != nil {
			return fmt.Errorf("unit can not be specified on a boolean or string attribute")
		}
	}

	// Assert only one of the attributes is set
	set := 0
	if a.Float != nil {
		set++
	}
	if a.Int != nil {
		set++
	}
	if a.String != nil {
		set++
	}
	if a.Bool != nil {
		set++
	}

	if set == 0 {
		return fmt.Errorf("no attribute value set")
	} else if set > 1 {
		return fmt.Errorf("only one attribute value may be set")
	}

	return nil
}

// Comparable returns whether the two attributes are comparable
func (a *Attribute) Comparable(b *Attribute) bool {
	if a == nil || b == nil {
		return false
	}

	// First use the units to decide if comparison is possible
	aUnit := a.getTypedUnit()
	bUnit := b.getTypedUnit()
	if aUnit != nil && bUnit != nil {
		return aUnit.Comparable(bUnit)
	} else if aUnit != nil && bUnit == nil {
		return false
	} else if aUnit == nil && bUnit != nil {
		return false
	}

	if a.String != nil {
		return b.String != nil
	}

	if a.Bool != nil {
		return b.Bool != nil
	}

	return true
}

// Compare compares two attributes. If the returned boolean value is false, it
// means the values are not comparable, either because they are of different
// types (bool versus int) or the units are incompatible for comparison.
// The returned int will be 0 if a==b, -1 if a < b, and +1 if a > b for all
// values but bool. For bool it will be 0 if a==b or 1 if a!=b.
func (a *Attribute) Compare(b *Attribute) (int, bool) {
	if !a.Comparable(b) {
		return 0, false
	}

	return a.comparator()(b)
}

// comparator returns the comparator function for the attribute
func (a *Attribute) comparator() compareFn {
	if a.Bool != nil {
		return a.boolComparator
	}
	if a.String != nil {
		return a.stringComparator
	}
	if a.Int != nil || a.Float != nil {
		return a.numberComparator
	}

	return nullComparator
}

// boolComparator compares two boolean attributes
func (a *Attribute) boolComparator(b *Attribute) (int, bool) {
	if *a.Bool == *b.Bool {
		return 0, true
	}

	return 1, true
}

// stringComparator compares two string attributes
func (a *Attribute) stringComparator(b *Attribute) (int, bool) {
	return strings.Compare(*a.String, *b.String), true
}

// numberComparator compares two number attributes, having either Int or Float
// set.
func (a *Attribute) numberComparator(b *Attribute) (int, bool) {
	// If they are both integers we do perfect precision comparisons
	if a.Int != nil && b.Int != nil {
		return a.intComparator(b)
	}

	// Push both into the float space
	af := a.getBigFloat()
	bf := b.getBigFloat()
	if af == nil || bf == nil {
		return 0, false
	}

	return af.Cmp(bf), true
}

// intComparator compares two integer attributes.
func (a *Attribute) intComparator(b *Attribute) (int, bool) {
	ai := a.getInt()
	bi := b.getInt()

	if ai == bi {
		return 0, true
	} else if ai < bi {
		return -1, true
	} else {
		return 1, true
	}
}

// nullComparator always returns false and is used when no comparison function
// is possible
func nullComparator(*Attribute) (int, bool) {
	return 0, false
}

// compareFn is used to compare two attributes. It returns -1, 0, 1 for ordering
// and a boolean for if the comparison is possible.
type compareFn func(b *Attribute) (int, bool)

// getBigFloat returns a big.Float representation of the attribute, converting
// the value to the base unit if a unit is specified.
func (a *Attribute) getBigFloat() *big.Float {
	f := new(big.Float)
	f.SetPrec(floatPrecision)
	if a.Int != nil {
		f.SetInt64(*a.Int)
	} else if a.Float != nil {
		f.SetFloat64(*a.Float)
	} else {
		return nil
	}

	// Get the unit
	u := a.getTypedUnit()

	// If there is no unit just return the float
	if u == nil {
		return f
	}

	// Convert to the base unit
	multiplier := new(big.Float)
	multiplier.SetPrec(floatPrecision)
	multiplier.SetInt64(u.Multiplier)
	if u.InverseMultiplier {
		base := big.NewFloat(1.0)
		base.SetPrec(floatPrecision)
		multiplier = multiplier.Quo(base, multiplier)
	}

	f.Mul(f, multiplier)
	return f
}

// getInt returns an int representation of the attribute, converting
// the value to the base unit if a unit is specified.
func (a *Attribute) getInt() int64 {
	if a.Int == nil {
		return 0
	}

	i := *a.Int

	// Get the unit
	u := a.getTypedUnit()

	// If there is no unit just return the int
	if u == nil {
		return i
	}

	if u.InverseMultiplier {
		i /= u.Multiplier
	} else {
		i *= u.Multiplier
	}

	return i
}

// getTypedUnit returns the Unit for the attribute or nil if no unit exists.
func (a *Attribute) getTypedUnit() *Unit {
	return UnitIndex[a.Unit]
}
