package structs

import (
	"fmt"
	"regexp"
	"strconv"
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
	Multiplier uint64

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

// Attribute is used to describe the value of an attribute, optionally
// specifying units
type Attribute struct {
	// Float is the float value for the attribute
	Float float64

	// Int is the int value for the attribute
	Int int64

	// String is the string value for the attribute
	String string

	// Bool is the bool value for the attribute
	Bool bool

	// Unit is the optional unit for the set int or float value
	Unit string
}

// Validate checks if the attribute is valid
func (a *Attribute) Validate() error {
	if a.Unit != "" {
		if _, ok := UnitIndex[a.Unit]; !ok {
			return fmt.Errorf("unrecognized unit %q", a.Unit)
		}
	}

	return nil
}

var (
	// numericWithUnits matches only if it is a integer or float ending with
	// units. It has two capture groups, one for the numeric value and one for
	// the unit value
	numericWithUnits = regexp.MustCompile(`^([-]?(?:[0-9]+|[0-9]+\.[0-9]+|\.[0-9]+))\s*([a-zA-Z]+\/?[a-zA-z]+|[a-zA-Z])$`)
)

func ParseAttribute(input string) *Attribute {
	// Try to parse as a bool
	b, err := strconv.ParseBool(input)
	if err == nil {
		return &Attribute{Bool: b}
	}

	// Try to parse as a number.

	// Check if the string is a number ending with potential units
	if matches := numericWithUnits.FindStringSubmatch(input); len(matches) == 3 {
		numeric := matches[1]
		unit := matches[2]

		// Check if we know about the unit. If we don't we can only treat this
		// as a string
		if _, ok := UnitIndex[unit]; !ok {
			return &Attribute{String: input}
		}

		// Try to parse as an int
		i, err := strconv.ParseInt(numeric, 10, 64)
		if err == nil {
			return &Attribute{Int: i, Unit: unit}
		}

		// Try to parse as a float
		f, err := strconv.ParseFloat(numeric, 64)
		if err == nil {
			return &Attribute{Float: f, Unit: unit}
		}
	}

	// Try to parse as an int
	i, err := strconv.ParseInt(input, 10, 64)
	if err == nil {
		return &Attribute{Int: i}
	}

	// Try to parse as a float
	f, err := strconv.ParseFloat(input, 64)
	if err == nil {
		return &Attribute{Float: f}
	}

	return &Attribute{String: input}
}
