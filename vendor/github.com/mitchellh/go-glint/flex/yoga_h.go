package flex

var (
	// Undefined defines undefined value
	Undefined = NAN
)

// Size describes size
type Size struct {
	Width  float32
	Height float32
}

// Value describes value
type Value struct {
	Value float32
	Unit  Unit
}

var (
	// ValueUndefined defines undefined YGValue
	ValueUndefined = Value{Undefined, UnitUndefined}
	// ValueAuto defines auto YGValue
	ValueAuto = Value{Undefined, UnitAuto}
)

// MeasureFunc describes function for measuring
type MeasureFunc func(node *Node, width float32, widthMode MeasureMode, height float32, heightMode MeasureMode) Size

// BaselineFunc describes function for baseline
type BaselineFunc func(node *Node, width float32, height float32) float32

// PrintFunc defines function for printing
type PrintFunc func(node *Node)

// Logger defines logging function
type Logger func(config *Config, node *Node, level LogLevel, format string, args ...interface{}) int
