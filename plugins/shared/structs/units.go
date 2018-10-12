package structs.

var (
	UnitIndex = make(map[string]*Unit, len(binarySIBytes)+len(decimalSIBytes)+len(binarySIByteRates)+len(decimalSIByteRates)+len(watts)+len(hertz))

	binarySIBytes = []*Unit{
		&Unit{
			Name:       "KiB",
			Base:       UnitByte,
			Multiplier: 1 << 10,
		},
		&Unit{
			Name:       "MiB",
			Base:       UnitByte,
			Multiplier: 1 << 20,
		},
		&Unit{
			Name:       "GiB",
			Base:       UnitByte,
			Multiplier: 1 << 30,
		},
		&Unit{
			Name:       "TiB",
			Base:       UnitByte,
			Multiplier: 1 << 40,
		},
		&Unit{
			Name:       "PiB",
			Base:       UnitByte,
			Multiplier: 1 << 50,
		},
		&Unit{
			Name:       "EiB",
			Base:       UnitByte,
			Multiplier: 1 << 60,
		},
	}

	decimalSIBytes = []*Unit{
		&Unit{
			Name:       "kB",
			Base:       UnitByte,
			Multiplier: Pow(1000, 1),
		},
		&Unit{
			Name:       "KB", // Alternative name for kB
			Base:       UnitByte,
			Multiplier: Pow(1000, 1),
		},
		&Unit{
			Name:       "MB",
			Base:       UnitByte,
			Multiplier: Pow(1000, 2),
		},
		&Unit{
			Name:       "GB",
			Base:       UnitByte,
			Multiplier: Pow(1000, 3),
		},
		&Unit{
			Name:       "TB",
			Base:       UnitByte,
			Multiplier: Pow(1000, 4),
		},
		&Unit{
			Name:       "PB",
			Base:       UnitByte,
			Multiplier: Pow(1000, 5),
		},
		&Unit{
			Name:       "EB",
			Base:       UnitByte,
			Multiplier: Pow(1000, 6),
		},
	}

	binarySIByteRates = []*Unit{
		&Unit{
			Name:       "KiB/s",
			Base:       UnitByteRate,
			Multiplier: 1 << 10,
		},
		&Unit{
			Name:       "MiB/s",
			Base:       UnitByteRate,
			Multiplier: 1 << 20,
		},
		&Unit{
			Name:       "GiB/s",
			Base:       UnitByteRate,
			Multiplier: 1 << 30,
		},
		&Unit{
			Name:       "TiB/s",
			Base:       UnitByteRate,
			Multiplier: 1 << 40,
		},
		&Unit{
			Name:       "PiB/s",
			Base:       UnitByteRate,
			Multiplier: 1 << 50,
		},
		&Unit{
			Name:       "EiB/s",
			Base:       UnitByteRate,
			Multiplier: 1 << 60,
		},
	}

	decimalSIByteRates = []*Unit{
		&Unit{
			Name:       "kB/s",
			Base:       UnitByteRate,
			Multiplier: Pow(1000, 1),
		},
		&Unit{
			Name:       "KB/s", // Alternative name for kB/s
			Base:       UnitByteRate,
			Multiplier: Pow(1000, 1),
		},
		&Unit{
			Name:       "MB/s",
			Base:       UnitByteRate,
			Multiplier: Pow(1000, 2),
		},
		&Unit{
			Name:       "GB/s",
			Base:       UnitByteRate,
			Multiplier: Pow(1000, 3),
		},
		&Unit{
			Name:       "TB/s",
			Base:       UnitByteRate,
			Multiplier: Pow(1000, 4),
		},
		&Unit{
			Name:       "PB/s",
			Base:       UnitByteRate,
			Multiplier: Pow(1000, 5),
		},
		&Unit{
			Name:       "EB/s",
			Base:       UnitByteRate,
			Multiplier: Pow(1000, 6),
		},
	}

	hertz = []*Unit{
		&Unit{
			Name:       "MHz",
			Base:       UnitHertz,
			Multiplier: Pow(1000, 1),
		},
		&Unit{
			Name:       "GHz",
			Base:       UnitHertz,
			Multiplier: Pow(1000, 3),
		},
	}

	watts = []*Unit{
		&Unit{
			Name:              "mW",
			Base:              UnitWatt,
			Multiplier:        Pow(10, 3),
			InverseMultiplier: true,
		},
		&Unit{
			Name:       "W",
			Base:       UnitWatt,
			Multiplier: 1,
		},
		&Unit{
			Name:       "kW",
			Base:       UnitWatt,
			Multiplier: Pow(10, 3),
		},
		&Unit{
			Name:       "MW",
			Base:       UnitWatt,
			Multiplier: Pow(10, 6),
		},
		&Unit{
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
		}
	}
}
