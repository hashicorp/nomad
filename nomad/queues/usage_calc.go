package queues

import (
	"math"
	"time"
)

func addUsage(total, addedUsage map[string]float64, multiplier float64) {
	for resource, amount := range addedUsage {
		total[resource] += amount * multiplier
	}
}

func decayMultiplier(ts, createdAt time.Time, halfLife time.Duration) float64 {
	elapsed := ts.Sub(createdAt).Seconds()
	return math.Pow(0.5, elapsed/halfLife.Seconds())
}

func weightedUsage(usage, weights map[string]float64) float64 {
	total := 0.0
	for resource, amount := range usage {
		if weight, ok := weights[resource]; ok && weight > 0 {
			amount = amount * weight
		}
		total += amount
	}

	return total
}

func totalUsage(usage map[string]float64) float64 {
	total := 0.0
	for _, amount := range usage {
		total += amount
	}
	return total
}
