package flex

// port of YGEnums.h

// Align describes align flex attribute
type Align int

const (
	// AlignAuto is "auto"
	AlignAuto Align = iota
	// AlignFlexStart is "flex-start"
	AlignFlexStart
	// AlignCenter if "center"
	AlignCenter
	// AlignFlexEnd is "flex-end"
	AlignFlexEnd
	// AlignStretch is "strech"
	AlignStretch
	// AlignBaseline is "baseline"
	AlignBaseline
	// AlignSpaceBetween is "space-between"
	AlignSpaceBetween
	// AlignSpaceAround is "space-around"
	AlignSpaceAround
)

// Dimension represents dimention
type Dimension int

const (
	// DimensionWidth is width
	DimensionWidth Dimension = iota
	// DimensionHeight is height
	DimensionHeight
)

// Direction represents right-to-left or left-to-right direction
type Direction int

const (
	// DirectionInherit is "inherit"
	DirectionInherit Direction = iota
	// DirectionLTR is "ltr"
	DirectionLTR
	// DirectionRTL is "rtl"
	DirectionRTL
)

// Display is "display" property
type Display int

const (
	// DisplayFlex is "flex"
	DisplayFlex Display = iota
	// DisplayNone is "none"
	DisplayNone
)

// Edge represents an edge
type Edge int

const (
	// EdgeLeft is left edge
	EdgeLeft Edge = iota
	// EdgeTop is top edge
	EdgeTop
	// EdgeRight is right edge
	EdgeRight
	// EdgeBottom is bottom edge
	EdgeBottom
	// EdgeStart is start edge
	EdgeStart
	// EdgeEnd is end edge
	EdgeEnd
	// EdgeHorizontal is horizontal edge
	EdgeHorizontal
	// EdgeVertical is vertical edge
	EdgeVertical
	// EdgeAll is all edge
	EdgeAll
)

const (
	// EdgeCount is count of edges
	EdgeCount = 9
)

// ExperimentalFeature defines experimental features
type ExperimentalFeature int

const (
	// ExperimentalFeatureWebFlexBasis is web flex basis
	ExperimentalFeatureWebFlexBasis ExperimentalFeature = iota
)

const (
	experimentalFeatureCount = 1
)

// FlexDirection describes "flex-direction" property
type FlexDirection int

const (
	// FlexDirectionColumn is "column"
	FlexDirectionColumn FlexDirection = iota
	// FlexDirectionColumnReverse is "column-reverse"
	FlexDirectionColumnReverse
	// FlexDirectionRow is "row"
	FlexDirectionRow
	// FlexDirectionRowReverse is "row-reverse"
	FlexDirectionRowReverse
)

// Justify is "justify" property
type Justify int

const (
	// JustifyFlexStart is "flex-start"
	JustifyFlexStart Justify = iota
	// JustifyCenter is "center"
	JustifyCenter
	// JustifyFlexEnd is "flex-end"
	JustifyFlexEnd
	// JustifySpaceBetween is "space-between"
	JustifySpaceBetween
	// JustifySpaceAround is "space-around"
	JustifySpaceAround
)

// LogLevel represents log level
type LogLevel int

const (
	LogLevelError LogLevel = iota
	LogLevelWarn
	LogLevelInfo
	LogLevelDebug
	LogLevelVerbose
	LogLevelFatal
)

// MeasureMode defines measurement mode
type MeasureMode int

const (
	// MeasureModeUndefined is undefined
	MeasureModeUndefined MeasureMode = iota
	// MeasureModeExactly is exactly
	MeasureModeExactly
	// MeasureModeAtMost is at-most
	MeasureModeAtMost
)

const (
	measureModeCount = 3
)

// NodeType defines node type
type NodeType int

const (
	// NodeTypeDefault is default node
	NodeTypeDefault NodeType = iota
	// NodeTypeText is text node
	NodeTypeText
)

// Overflow describes "overflow" property
type Overflow int

const (
	// OverflowVisible is "visible"
	OverflowVisible Overflow = iota
	// OverflowHidden is "hidden"
	OverflowHidden
	// OverflowScroll is "scroll"
	OverflowScroll
)

// PositionType is "position" property
type PositionType int

const (
	// PositionTypeRelative is "relative"
	PositionTypeRelative PositionType = iota
	// PositionTypeAbsolute is "absolute"
	PositionTypeAbsolute
)

type PrintOptions int

const (
	PrintOptionsLayout PrintOptions = 1 << iota
	PrintOptionsStyle
	PrintOptionsChildren
)

// Unit is "unit" property
type Unit int

const (
	// UnitUndefined is "undefined"
	UnitUndefined Unit = iota
	// UnitPoint is "point"
	UnitPoint
	// UnitPercent is "percent"
	UnitPercent
	// UnitAuto is "auto"
	UnitAuto
)

// Wrap is "wrap" property
type Wrap int

const (
	// WrapNoWrap is "no-wrap"
	WrapNoWrap Wrap = iota
	// WrapWrap is "wrap"
	WrapWrap
	// WrapWrapReverse is "reverse"
	WrapWrapReverse
)

// AlignToString returns string version of Align enum
func AlignToString(value Align) string {
	switch value {
	case AlignAuto:
		return "auto"
	case AlignFlexStart:
		return "flex-start"
	case AlignCenter:
		return "center"
	case AlignFlexEnd:
		return "flex-end"
	case AlignStretch:
		return "stretch"
	case AlignBaseline:
		return "baseline"
	case AlignSpaceBetween:
		return "space-between"
	case AlignSpaceAround:
		return "space-around"
	}
	return "unknown"
}

// DimensionToString returns string version of Dimension enum
func DimensionToString(value Dimension) string {
	switch value {
	case DimensionWidth:
		return "width"
	case DimensionHeight:
		return "height"
	}
	return "unknown"
}

// DirectionToString returns string version of Direction enum
func DirectionToString(value Direction) string {
	switch value {
	case DirectionInherit:
		return "inherit"
	case DirectionLTR:
		return "ltr"
	case DirectionRTL:
		return "rtl"
	}
	return "unknown"
}

// DisplayToString returns string version of Display enum
func DisplayToString(value Display) string {
	switch value {
	case DisplayFlex:
		return "flex"
	case DisplayNone:
		return "none"
	}
	return "unknown"
}

// EdgeToString returns string version of Edge enum
func EdgeToString(value Edge) string {
	switch value {
	case EdgeLeft:
		return "left"
	case EdgeTop:
		return "top"
	case EdgeRight:
		return "right"
	case EdgeBottom:
		return "bottom"
	case EdgeStart:
		return "start"
	case EdgeEnd:
		return "end"
	case EdgeHorizontal:
		return "horizontal"
	case EdgeVertical:
		return "vertical"
	case EdgeAll:
		return "all"
	}
	return "unknown"
}

// ExperimentalFeatureToString returns string version of ExperimentalFeature enum
func ExperimentalFeatureToString(value ExperimentalFeature) string {
	switch value {
	case ExperimentalFeatureWebFlexBasis:
		return "web-flex-basis"
	}
	return "unknown"
}

// FlexDirectionToString returns string version of FlexDirection enum
func FlexDirectionToString(value FlexDirection) string {
	switch value {
	case FlexDirectionColumn:
		return "column"
	case FlexDirectionColumnReverse:
		return "column-reverse"
	case FlexDirectionRow:
		return "row"
	case FlexDirectionRowReverse:
		return "row-reverse"
	}
	return "unknown"
}

// JustifyToString returns string version of Justify enum
func JustifyToString(value Justify) string {
	switch value {
	case JustifyFlexStart:
		return "flex-start"
	case JustifyCenter:
		return "center"
	case JustifyFlexEnd:
		return "flex-end"
	case JustifySpaceBetween:
		return "space-between"
	case JustifySpaceAround:
		return "space-around"
	}
	return "unknown"
}

// LogLevelToString returns string version of LogLevel enum
func LogLevelToString(value LogLevel) string {
	switch value {
	case LogLevelError:
		return "error"
	case LogLevelWarn:
		return "warn"
	case LogLevelInfo:
		return "info"
	case LogLevelDebug:
		return "debug"
	case LogLevelVerbose:
		return "verbose"
	case LogLevelFatal:
		return "fatal"
	}
	return "unknown"
}

// MeasureModeToString returns string version of MeasureMode enum
func MeasureModeToString(value MeasureMode) string {
	switch value {
	case MeasureModeUndefined:
		return "undefined"
	case MeasureModeExactly:
		return "exactly"
	case MeasureModeAtMost:
		return "at-most"
	}
	return "unknown"
}

// NodeTypeToString returns string version of NodeType enum
func NodeTypeToString(value NodeType) string {
	switch value {
	case NodeTypeDefault:
		return "default"
	case NodeTypeText:
		return "text"
	}
	return "unknown"
}

// OverflowToString returns string version of Overflow enum
func OverflowToString(value Overflow) string {
	switch value {
	case OverflowVisible:
		return "visible"
	case OverflowHidden:
		return "hidden"
	case OverflowScroll:
		return "scroll"
	}
	return "unknown"
}

// PositionTypeToString returns string version of PositionType enum
func PositionTypeToString(value PositionType) string {
	switch value {
	case PositionTypeRelative:
		return "relative"
	case PositionTypeAbsolute:
		return "absolute"
	}
	return "unknown"
}

// PrintOptionsToString returns string version of PrintOptions enum
func PrintOptionsToString(value PrintOptions) string {
	switch value {
	case PrintOptionsLayout:
		return "layout"
	case PrintOptionsStyle:
		return "style"
	case PrintOptionsChildren:
		return "children"
	}
	return "unknown"
}

// UnitToString returns string version of Unit enum
func UnitToString(value Unit) string {
	switch value {
	case UnitUndefined:
		return "undefined"
	case UnitPoint:
		return "point"
	case UnitPercent:
		return "percent"
	case UnitAuto:
		return "auto"
	}
	return "unknown"
}

// WrapToString returns string version of Wrap enum
func WrapToString(value Wrap) string {
	switch value {
	case WrapNoWrap:
		return "no-wrap"
	case WrapWrap:
		return "wrap"
	case WrapWrapReverse:
		return "wrap-reverse"
	}
	return "unknown"
}
