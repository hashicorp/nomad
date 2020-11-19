package components

import (
	"container/ring"
	"context"
	"math"
	"sync"

	"github.com/mitchellh/go-glint"
)

// SparklineComponent renders a sparkline graph.
type SparklineComponent struct {
	sync.Mutex

	// If set, this will style the peak value.
	PeakStyle []glint.StyleOption

	values *ring.Ring
}

// Sparkline creates a SparklineComponent with the given set of initial values.
// These initial values will also specify the max width for this sparkline
// unless values are replaced with Set.
func Sparkline(values []uint) *SparklineComponent {
	var c SparklineComponent
	c.Set(values)
	return &c
}

// Set sets the full set of values to the given slice. This will also reset
// the size of the sparkline to this length.
func (c *SparklineComponent) Set(values []uint) {
	c.Lock()
	defer c.Unlock()
	c.values = ring.New(len(values))
	for _, v := range values {
		c.values.Value = v
		c.values = c.values.Next()
	}
}

// Append adds the given values to the end of the values buffer. The buffer
// size is determined by the values list given in Sparkline or Set. This will
// overwrite the oldest values.
func (c *SparklineComponent) Append(values ...uint) {
	c.Lock()
	defer c.Unlock()
	for _, v := range values {
		c.values.Value = v
		c.values = c.values.Next()
	}
}

func (c *SparklineComponent) valuesSlice() []uint {
	result := make([]uint, c.values.Len())
	for i := range result {
		result[i] = c.values.Value.(uint)
		c.values = c.values.Next()
	}

	return result
}

func (c *SparklineComponent) Body(context.Context) glint.Component {
	c.Lock()
	defer c.Unlock()

	values := c.valuesSlice()

	// If we have nothing we render nothing
	if len(values) == 0 {
		return nil
	}

	// Find the max
	max := values[0]
	if len(values) > 1 {
		for _, v := range values[1:] {
			if v > max {
				max = v
			}
		}
	}

	// Build each symbol
	peak := false
	parts := make([]glint.Component, len(values))
	for i, v := range values {
		symbolIdx := int(math.Ceil(float64(v) / float64(max) * float64(len(sparklineSymbols)-1)))
		parts[i] = glint.Text(string(sparklineSymbols[symbolIdx]))

		if len(c.PeakStyle) > 0 && v == max && !peak {
			peak = true
			parts[i] = glint.Style(parts[i], c.PeakStyle...)
		}
	}

	// Render them in a row
	return glint.Layout(parts...).Row()
}

var sparklineSymbols = []rune{
	'\u2581',
	'\u2582',
	'\u2583',
	'\u2584',
	'\u2585',
	'\u2586',
	'\u2587',
	'\u2588',
}
