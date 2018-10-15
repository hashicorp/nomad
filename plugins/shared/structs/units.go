package structs

import "sort"

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
			Name:       "KiB",
			Base:       UnitByte,
			Multiplier: 1 << 10,
		},
		{
			Name:       "MiB",
			Base:       UnitByte,
			Multiplier: 1 << 20,
		},
		{
			Name:       "GiB",
			Base:       UnitByte,
			Multiplier: 1 << 30,
		},
		{
			Name:       "TiB",
			Base:       UnitByte,
			Multiplier: 1 << 40,
		},
		{
			Name:       "PiB",
			Base:       UnitByte,
			Multiplier: 1 << 50,
		},
		{
			Name:       "EiB",
			Base:       UnitByte,
			Multiplier: 1 << 60,
		},
	}

	decimalSIBytes = []*Unit{
		{
			Name:       "kB",
			Base:       UnitByte,
			Multiplier: Pow(1000, 1),
		},
		{
			Name:       "KB", // Alternative name for kB
			Base:       UnitByte,
			Multiplier: Pow(1000, 1),
		},
		{
			Name:       "MB",
			Base:       UnitByte,
			Multiplier: Pow(1000, 2),
		},
		{
			Name:       "GB",
			Base:       UnitByte,
			Multiplier: Pow(1000, 3),
		},
		{
			Name:       "TB",
			Base:       UnitByte,
			Multiplier: Pow(1000, 4),
		},
		{
			Name:       "PB",
			Base:       UnitByte,
			Multiplier: Pow(1000, 5),
		},
		{
			Name:       "EB",
			Base:       UnitByte,
			Multiplier: Pow(1000, 6),
		},
	}

	binarySIByteRates = []*Unit{
		{
			Name:       "KiB/s",
			Base:       UnitByteRate,
			Multiplier: 1 << 10,
		},
		{
			Name:       "MiB/s",
			Base:       UnitByteRate,
			Multiplier: 1 << 20,
		},
		{
			Name:       "GiB/s",
			Base:       UnitByteRate,
			Multiplier: 1 << 30,
		},
		{
			Name:       "TiB/s",
			Base:       UnitByteRate,
			Multiplier: 1 << 40,
		},
		{
			Name:       "PiB/s",
			Base:       UnitByteRate,
			Multiplier: 1 << 50,
		},
		{
			Name:       "EiB/s",
			Base:       UnitByteRate,
			Multiplier: 1 << 60,
		},
	}

	decimalSIByteRates = []*Unit{
		{
			Name:       "kB/s",
			Base:       UnitByteRate,
			Multiplier: Pow(1000, 1),
		},
		{
			Name:       "KB/s", // Alternative name for kB/s
			Base:       UnitByteRate,
			Multiplier: Pow(1000, 1),
		},
		{
			Name:       "MB/s",
			Base:       UnitByteRate,
			Multiplier: Pow(1000, 2),
		},
		{
			Name:       "GB/s",
			Base:       UnitByteRate,
			Multiplier: Pow(1000, 3),
		},
		{
			Name:       "TB/s",
			Base:       UnitByteRate,
			Multiplier: Pow(1000, 4),
		},
		{
			Name:       "PB/s",
			Base:       UnitByteRate,
			Multiplier: Pow(1000, 5),
		},
		{
			Name:       "EB/s",
			Base:       UnitByteRate,
			Multiplier: Pow(1000, 6),
		},
	}

	hertz = []*Unit{
		{
			Name:       "MHz",
			Base:       UnitHertz,
			Multiplier: Pow(1000, 1),
		},
		{
			Name:       "GHz",
			Base:       UnitHertz,
			Multiplier: Pow(1000, 3),
		},
	}

	watts = []*Unit{
		{
			Name:              "mW",
			Base:              UnitWatt,
			Multiplier:        Pow(10, 3),
			InverseMultiplier: true,
		},
		{
			Name:       "W",
			Base:       UnitWatt,
			Multiplier: 1,
		},
		{
			Name:       "kW",
			Base:       UnitWatt,
			Multiplier: Pow(10, 3),
		},
		{
			Name:       "MW",
			Base:       UnitWatt,
			Multiplier: Pow(10, 6),
		},
		{
			Name:       "GW",
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
