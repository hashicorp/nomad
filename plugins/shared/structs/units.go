// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package structs

import "sort"

const (
	// Binary SI Byte Units
	UnitKiB = "KiB"
	UnitMiB = "MiB"
	UnitGiB = "GiB"
	UnitTiB = "TiB"
	UnitPiB = "PiB"
	UnitEiB = "EiB"

	// Decimal SI Byte Units
	UnitkB = "kB"
	UnitKB = "KB"
	UnitMB = "MB"
	UnitGB = "GB"
	UnitTB = "TB"
	UnitPB = "PB"
	UnitEB = "EB"

	// Binary SI Byte Rates
	UnitKiBPerS = "KiB/s"
	UnitMiBPerS = "MiB/s"
	UnitGiBPerS = "GiB/s"
	UnitTiBPerS = "TiB/s"
	UnitPiBPerS = "PiB/s"
	UnitEiBPerS = "EiB/s"

	// Decimal SI Byte Rates
	UnitkBPerS = "kB/s"
	UnitKBPerS = "KB/s"
	UnitMBPerS = "MB/s"
	UnitGBPerS = "GB/s"
	UnitTBPerS = "TB/s"
	UnitPBPerS = "PB/s"
	UnitEBPerS = "EB/s"

	// Hertz units
	UnitMHz = "MHz"
	UnitGHz = "GHz"

	// Watts units
	UnitmW = "mW"
	UnitW  = "W"
	UnitkW = "kW"
	UnitMW = "MW"
	UnitGW = "GW"
)

var (
	// numUnits is the number of known units
	numUnits = len(binarySIBytes) + len(decimalSIBytes) + len(binarySIByteRates) + len(decimalSIByteRates) + len(watts) + len(hertz)

	// UnitIndex is a map of unit name to unit
	UnitIndex = make(map[string]*Unit, numUnits)

	// lengthSortedUnits is a list of unit names sorted by length with longest
	// first
	lengthSortedUnits = make([]string, 0, numUnits)

	binarySIBytes = []*Unit{
		{
			Name:       UnitKiB,
			Base:       UnitByte,
			Multiplier: 1 << 10,
		},
		{
			Name:       UnitMiB,
			Base:       UnitByte,
			Multiplier: 1 << 20,
		},
		{
			Name:       UnitGiB,
			Base:       UnitByte,
			Multiplier: 1 << 30,
		},
		{
			Name:       UnitTiB,
			Base:       UnitByte,
			Multiplier: 1 << 40,
		},
		{
			Name:       UnitPiB,
			Base:       UnitByte,
			Multiplier: 1 << 50,
		},
		{
			Name:       UnitEiB,
			Base:       UnitByte,
			Multiplier: 1 << 60,
		},
	}

	decimalSIBytes = []*Unit{
		{
			Name:       UnitkB,
			Base:       UnitByte,
			Multiplier: Pow(1000, 1),
		},
		{
			Name:       UnitKB, // Alternative name for kB
			Base:       UnitByte,
			Multiplier: Pow(1000, 1),
		},
		{
			Name:       UnitMB,
			Base:       UnitByte,
			Multiplier: Pow(1000, 2),
		},
		{
			Name:       UnitGB,
			Base:       UnitByte,
			Multiplier: Pow(1000, 3),
		},
		{
			Name:       UnitTB,
			Base:       UnitByte,
			Multiplier: Pow(1000, 4),
		},
		{
			Name:       UnitPB,
			Base:       UnitByte,
			Multiplier: Pow(1000, 5),
		},
		{
			Name:       UnitEB,
			Base:       UnitByte,
			Multiplier: Pow(1000, 6),
		},
	}

	binarySIByteRates = []*Unit{
		{
			Name:       UnitKiBPerS,
			Base:       UnitByteRate,
			Multiplier: 1 << 10,
		},
		{
			Name:       UnitMiBPerS,
			Base:       UnitByteRate,
			Multiplier: 1 << 20,
		},
		{
			Name:       UnitGiBPerS,
			Base:       UnitByteRate,
			Multiplier: 1 << 30,
		},
		{
			Name:       UnitTiBPerS,
			Base:       UnitByteRate,
			Multiplier: 1 << 40,
		},
		{
			Name:       UnitPiBPerS,
			Base:       UnitByteRate,
			Multiplier: 1 << 50,
		},
		{
			Name:       UnitEiBPerS,
			Base:       UnitByteRate,
			Multiplier: 1 << 60,
		},
	}

	decimalSIByteRates = []*Unit{
		{
			Name:       UnitkBPerS,
			Base:       UnitByteRate,
			Multiplier: Pow(1000, 1),
		},
		{
			Name:       UnitKBPerS,
			Base:       UnitByteRate,
			Multiplier: Pow(1000, 1),
		},
		{
			Name:       UnitMBPerS,
			Base:       UnitByteRate,
			Multiplier: Pow(1000, 2),
		},
		{
			Name:       UnitGBPerS,
			Base:       UnitByteRate,
			Multiplier: Pow(1000, 3),
		},
		{
			Name:       UnitTBPerS,
			Base:       UnitByteRate,
			Multiplier: Pow(1000, 4),
		},
		{
			Name:       UnitPBPerS,
			Base:       UnitByteRate,
			Multiplier: Pow(1000, 5),
		},
		{
			Name:       UnitEBPerS,
			Base:       UnitByteRate,
			Multiplier: Pow(1000, 6),
		},
	}

	hertz = []*Unit{
		{
			Name:       UnitMHz,
			Base:       UnitHertz,
			Multiplier: Pow(1000, 2),
		},
		{
			Name:       UnitGHz,
			Base:       UnitHertz,
			Multiplier: Pow(1000, 3),
		},
	}

	watts = []*Unit{
		{
			Name:              UnitmW,
			Base:              UnitWatt,
			Multiplier:        Pow(10, 3),
			InverseMultiplier: true,
		},
		{
			Name:       UnitW,
			Base:       UnitWatt,
			Multiplier: 1,
		},
		{
			Name:       UnitkW,
			Base:       UnitWatt,
			Multiplier: Pow(10, 3),
		},
		{
			Name:       UnitMW,
			Base:       UnitWatt,
			Multiplier: Pow(10, 6),
		},
		{
			Name:       UnitGW,
			Base:       UnitWatt,
			Multiplier: Pow(10, 9),
		},
	}
)

func init() {
	// Build the index
	for _, units := range [][]*Unit{binarySIBytes, decimalSIBytes, binarySIByteRates, decimalSIByteRates, watts, hertz} {
		for _, unit := range units {
			UnitIndex[unit.Name] = unit
			lengthSortedUnits = append(lengthSortedUnits, unit.Name)
		}
	}

	sort.Slice(lengthSortedUnits, func(i, j int) bool {
		return len(lengthSortedUnits[i]) >= len(lengthSortedUnits[j])
	})
}
