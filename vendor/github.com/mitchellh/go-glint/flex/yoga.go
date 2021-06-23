package flex

import (
	"fmt"
	"os"
)

// CachedMeasurement describes measurements
type CachedMeasurement struct {
	availableWidth    float32
	availableHeight   float32
	widthMeasureMode  MeasureMode
	heightMeasureMode MeasureMode

	computedWidth  float32
	computedHeight float32
}

// This value was chosen based on empiracle data. Even the most complicated
// layouts should not require more than 16 entries to fit within the cache.
const maxCachedResultCount = 16

// Layout describes position information after layout is finished
type Layout struct {
	Position   [4]float32
	Dimensions [2]float32
	Margin     [6]float32
	Border     [6]float32
	Padding    [6]float32
	Direction  Direction

	computedFlexBasisGeneration int
	computedFlexBasis           float32
	HadOverflow                 bool

	// Instead of recomputing the entire layout every single time, we
	// cache some information to break early when nothing changed
	generationCount     int
	lastParentDirection Direction

	nextCachedMeasurementsIndex int
	cachedMeasurements          [maxCachedResultCount]CachedMeasurement

	measuredDimensions [2]float32

	cachedLayout CachedMeasurement
}

// Style describes CSS flexbox style of the node
type Style struct {
	Direction      Direction
	FlexDirection  FlexDirection
	JustifyContent Justify
	AlignContent   Align
	AlignItems     Align
	AlignSelf      Align
	PositionType   PositionType
	FlexWrap       Wrap
	Overflow       Overflow
	Display        Display
	Flex           float32
	FlexGrow       float32
	FlexShrink     float32
	FlexBasis      Value
	Margin         [EdgeCount]Value
	Position       [EdgeCount]Value
	Padding        [EdgeCount]Value
	Border         [EdgeCount]Value
	Dimensions     [2]Value
	MinDimensions  [2]Value
	MaxDimensions  [2]Value

	// Yoga specific properties, not compatible with flexbox specification
	AspectRatio float32
}

// Config describes a configuration
type Config struct {
	experimentalFeatures      [experimentalFeatureCount + 1]bool
	UseWebDefaults            bool
	UseLegacyStretchBehaviour bool
	PointScaleFactor          float32
	Logger                    Logger
	Context                   interface{}
}

// Node describes a an element
type Node struct {
	Style     Style
	Layout    Layout
	lineIndex int

	Parent   *Node
	Children []*Node

	NextChild *Node

	Measure  MeasureFunc
	Baseline BaselineFunc
	Print    PrintFunc
	Config   *Config
	Context  interface{}

	IsDirty      bool
	hasNewLayout bool
	NodeType     NodeType

	resolvedDimensions [2]*Value
}

var (
	undefinedValue = Value{
		Value: Undefined,
		Unit:  UnitUndefined,
	}

	autoValue = Value{
		Value: Undefined,
		Unit:  UnitAuto,
	}

	defaultEdgeValuesUnit = [EdgeCount]Value{
		undefinedValue,
		undefinedValue,
		undefinedValue,
		undefinedValue,
		undefinedValue,
		undefinedValue,
		undefinedValue,
		undefinedValue,
		undefinedValue,
	}

	defaultDimensionValues = [2]float32{
		Undefined,
		Undefined,
	}

	defaultDimensionValuesUnit = [2]Value{
		undefinedValue,
		undefinedValue,
	}

	defaultDimensionValuesAutoUnit = [2]Value{
		autoValue,
		autoValue,
	}
)

const (
	defaultFlexGrow      float32 = 0
	defaultFlexShrink    float32 = 0
	webDefaultFlexShrink float32 = 1
)

var (
	nodeDefaults = Node{
		Parent:             nil,
		Children:           nil,
		hasNewLayout:       true,
		IsDirty:            false,
		NodeType:           NodeTypeDefault,
		resolvedDimensions: [2]*Value{&ValueUndefined, &ValueUndefined},
		Style: Style{
			Flex:           Undefined,
			FlexGrow:       Undefined,
			FlexShrink:     Undefined,
			FlexBasis:      autoValue,
			JustifyContent: JustifyFlexStart,
			AlignItems:     AlignStretch,
			AlignContent:   AlignFlexStart,
			Direction:      DirectionInherit,
			FlexDirection:  FlexDirectionColumn,
			Overflow:       OverflowVisible,
			Display:        DisplayFlex,
			Dimensions:     defaultDimensionValuesAutoUnit,
			MinDimensions:  defaultDimensionValuesUnit,
			MaxDimensions:  defaultDimensionValuesUnit,
			Position:       defaultEdgeValuesUnit,
			Margin:         defaultEdgeValuesUnit,
			Padding:        defaultEdgeValuesUnit,
			Border:         defaultEdgeValuesUnit,
			AspectRatio:    Undefined,
		},
		Layout: Layout{
			Dimensions:                  defaultDimensionValues,
			lastParentDirection:         Direction(-1),
			nextCachedMeasurementsIndex: 0,
			computedFlexBasis:           Undefined,
			HadOverflow:                 false,
			measuredDimensions:          defaultDimensionValues,
			cachedLayout: CachedMeasurement{
				widthMeasureMode:  MeasureMode(-1),
				heightMeasureMode: MeasureMode(-1),
				computedWidth:     -1,
				computedHeight:    -1,
			},
		},
	}

	configDefaults = Config{
		experimentalFeatures: [experimentalFeatureCount + 1]bool{
			false,
			false,
		},
		UseWebDefaults:   false,
		PointScaleFactor: 1,
		Logger:           DefaultLog,
		Context:          nil,
	}

	// ValueZero defines a zero value
	ValueZero = Value{Value: 0, Unit: UnitPoint}
)

func valueEq(v1, v2 Value) bool {
	if v1.Unit != v2.Unit {
		return false
	}
	return feq(v1.Value, v2.Value)
}

// DefaultLog is default logging function
func DefaultLog(config *Config, node *Node, level LogLevel, format string,
	args ...interface{}) int {
	switch level {
	case LogLevelError, LogLevelFatal:
		n, _ := fmt.Fprintf(os.Stderr, format, args...)
		return n
	case LogLevelWarn, LogLevelInfo, LogLevelDebug, LogLevelVerbose:
		fallthrough
	default:
		n, _ := fmt.Printf(format, args...)
		return n
	}
}

func computedEdgeValue(edges []Value, edge Edge, defaultValue *Value) *Value {
	if edges[edge].Unit != UnitUndefined {
		return &edges[edge]
	}

	isVertEdge := edge == EdgeTop || edge == EdgeBottom
	if isVertEdge && edges[EdgeVertical].Unit != UnitUndefined {
		return &edges[EdgeVertical]
	}

	isHorizEdge := (edge == EdgeLeft || edge == EdgeRight || edge == EdgeStart || edge == EdgeEnd)
	if isHorizEdge && edges[EdgeHorizontal].Unit != UnitUndefined {
		return &edges[EdgeHorizontal]
	}

	if edges[EdgeAll].Unit != UnitUndefined {
		return &edges[EdgeAll]
	}

	if edge == EdgeStart || edge == EdgeEnd {
		return &ValueUndefined
	}

	return defaultValue
}

func resolveValue(value *Value, parentSize float32) float32 {
	switch value.Unit {
	case UnitUndefined, UnitAuto:
		return Undefined
	case UnitPoint:
		return value.Value
	case UnitPercent:
		return value.Value * parentSize / 100
	}
	return Undefined
}

func resolveValueMargin(value *Value, parentSize float32) float32 {
	if value.Unit == UnitAuto {
		return 0
	}
	return resolveValue(value, parentSize)
}

// NewNodeWithConfig creates new node with config
func NewNodeWithConfig(config *Config) *Node {
	node := nodeDefaults

	if config.UseWebDefaults {
		node.Style.FlexDirection = FlexDirectionRow
		node.Style.AlignContent = AlignStretch
	}
	node.Config = config
	return &node
}

// NewNode creates a new node
func NewNode() *Node {
	return NewNodeWithConfig(&configDefaults)
}

// Reset resets a node
func (node *Node) Reset() {
	assertWithNode(node, len(node.Children) == 0, "Cannot reset a node which still has children attached")
	assertWithNode(node, node.Parent == nil, "Cannot reset a node still attached to a parent")

	node.Children = nil

	config := node.Config
	*node = nodeDefaults
	if config.UseWebDefaults {
		node.Style.FlexDirection = FlexDirectionRow
		node.Style.AlignContent = AlignStretch
	}
	node.Config = config
}

// ConfigGetDefault returns default config, only for C#
func ConfigGetDefault() *Config {
	return &configDefaults
}

// NewConfig creates new config
func NewConfig() *Config {
	config := &Config{}
	assertCond(config != nil, "Could not allocate memory for config")

	*config = configDefaults
	return config
}

// ConfigCopy copies a config
func ConfigCopy(dest *Config, src *Config) {
	*dest = *src
}

func nodeMarkDirtyInternal(node *Node) {
	if !node.IsDirty {
		node.IsDirty = true
		node.Layout.computedFlexBasis = Undefined
		if node.Parent != nil {
			nodeMarkDirtyInternal(node.Parent)
		}
	}
}

// SetMeasureFunc sets measure function
func (node *Node) SetMeasureFunc(measureFunc MeasureFunc) {
	if measureFunc == nil {
		node.Measure = nil
		// TODO: t18095186 Move nodeType to opt-in function and mark appropriate places in Litho
		node.NodeType = NodeTypeDefault
	} else {
		assertWithNode(
			node,
			len(node.Children) == 0,
			"Cannot set measure function: Nodes with measure functions cannot have children.")
		node.Measure = measureFunc
		// TODO: t18095186 Move nodeType to opt-in function and mark appropriate places in Litho
		node.NodeType = NodeTypeText
	}
}

// InsertChild inserts a child
func (node *Node) InsertChild(child *Node, idx int) {
	assertWithNode(node, child.Parent == nil, "Child already has a parent, it must be removed first.")
	assertWithNode(node, node.Measure == nil, "Cannot add child: Nodes with measure functions cannot have children.")

	a := node.Children
	// https://github.com/golang/go/wiki/SliceTricks
	a = append(a[:idx], append([]*Node{child}, a[idx:]...)...)

	node.Children = a

	child.Parent = node
	nodeMarkDirtyInternal(node)
}

func (node *Node) deleteChild(child *Node) *Node {
	a := node.Children
	n := len(a)
	for i := 0; i < n; i++ {
		if a[i] == child {
			removed := a[i]
			copy(a[i:], a[i+1:])
			a[len(a)-1] = nil // or the zero value of T
			a = a[:len(a)-1]
			node.Children = a
			return removed
		}
	}
	return nil
}

// RemoveChild removes child node
func (node *Node) RemoveChild(child *Node) {
	if node.deleteChild(child) != nil {
		child.Layout = nodeDefaults.Layout // layout is no longer valid
		child.Parent = nil
		nodeMarkDirtyInternal(node)
	}
}

// GetChild returns a child at a given index
func (node *Node) GetChild(idx int) *Node {
	if idx < len(node.Children) {
		return node.Children[idx]
	}
	return nil
}

// MarkDirty marks node as dirty
func (node *Node) MarkDirty() {
	assertWithNode(node, node.Measure != nil,
		"Only leaf nodes with custom measure functions should manually mark themselves as dirty")
	nodeMarkDirtyInternal(node)
}

func styleEq(s1, s2 *Style) bool {
	if s1.Direction != s2.Direction ||
		s1.FlexDirection != s2.FlexDirection ||
		s1.JustifyContent != s2.JustifyContent ||
		s1.AlignContent != s2.AlignContent ||
		s1.AlignItems != s2.AlignItems ||
		s1.AlignSelf != s2.AlignSelf ||
		s1.PositionType != s2.PositionType ||
		s1.FlexWrap != s2.FlexWrap ||
		s1.Overflow != s2.Overflow ||
		s1.Display != s2.Display ||
		!feq(s1.Flex, s2.Flex) ||
		!feq(s1.FlexGrow, s2.FlexGrow) ||
		!feq(s1.FlexShrink, s2.FlexShrink) ||
		!valueEq(s1.FlexBasis, s2.FlexBasis) {
		return false
	}
	for i := 0; i < EdgeCount; i++ {
		if !valueEq(s1.Margin[i], s2.Margin[i]) ||
			!valueEq(s1.Position[i], s2.Position[i]) ||
			!valueEq(s1.Padding[i], s2.Padding[i]) ||
			!valueEq(s1.Border[i], s2.Border[i]) {
			return false
		}
	}
	for i := 0; i < 2; i++ {
		if !valueEq(s1.Dimensions[i], s2.Dimensions[i]) ||
			!valueEq(s1.MinDimensions[i], s2.MinDimensions[i]) ||
			!valueEq(s1.MaxDimensions[i], s2.MaxDimensions[i]) {
			return false
		}
	}
	return true
}

// NodeCopyStyle copies style
func NodeCopyStyle(dstNode *Node, srcNode *Node) {
	if !styleEq(&dstNode.Style, &srcNode.Style) {
		dstNode.Style = srcNode.Style
		nodeMarkDirtyInternal(dstNode)
	}
}

func resolveFlexGrow(node *Node) float32 {
	// Root nodes flexGrow should always be 0
	if node.Parent == nil {
		return 0
	}
	if !FloatIsUndefined(node.Style.FlexGrow) {
		return node.Style.FlexGrow
	}
	if !FloatIsUndefined(node.Style.Flex) && node.Style.Flex > 0 {
		return node.Style.Flex
	}
	return defaultFlexGrow
}

// StyleGetFlexGrow gets flex grow
func (node *Node) StyleGetFlexGrow() float32 {
	if FloatIsUndefined(node.Style.FlexGrow) {
		return defaultFlexGrow
	}
	return node.Style.FlexGrow
}

// StyleGetFlexShrink gets flex shrink
func (node *Node) StyleGetFlexShrink() float32 {
	if FloatIsUndefined(node.Style.FlexShrink) {
		if node.Config.UseWebDefaults {
			return webDefaultFlexShrink
		}
		return defaultFlexShrink
	}
	return node.Style.FlexShrink
}

func nodeResolveFlexShrink(node *Node) float32 {
	// Root nodes flexShrink should always be 0
	if node.Parent == nil {
		return 0
	}
	if !FloatIsUndefined(node.Style.FlexShrink) {
		return node.Style.FlexShrink
	}
	if !node.Config.UseWebDefaults && !FloatIsUndefined(node.Style.Flex) &&
		node.Style.Flex < 0 {
		return -node.Style.Flex
	}
	if node.Config.UseWebDefaults {
		return webDefaultFlexShrink
	}
	return defaultFlexShrink
}

func nodeResolveFlexBasisPtr(node *Node) *Value {
	style := &node.Style
	if style.FlexBasis.Unit != UnitAuto && style.FlexBasis.Unit != UnitUndefined {
		return &style.FlexBasis
	}
	if !FloatIsUndefined(style.Flex) && style.Flex > 0 {
		if node.Config.UseWebDefaults {
			return &ValueAuto
		}
		return &ValueZero
	}
	return &ValueAuto
}

// see yoga_props.go

var (
	currentGenerationCount = 0
)

// FloatIsUndefined returns true if value is undefined
func FloatIsUndefined(value float32) bool {
	return IsNaN(value)
}

// ValueEqual returns true if values are equal
func ValueEqual(a Value, b Value) bool {
	if a.Unit != b.Unit {
		return false
	}

	if a.Unit == UnitUndefined {
		return true
	}

	return fabs(a.Value-b.Value) < 0.0001
}

func resolveDimensions(node *Node) {
	for dim := DimensionWidth; dim <= DimensionHeight; dim++ {
		if node.Style.MaxDimensions[dim].Unit != UnitUndefined &&
			ValueEqual(node.Style.MaxDimensions[dim], node.Style.MinDimensions[dim]) {
			node.resolvedDimensions[dim] = &node.Style.MaxDimensions[dim]
		} else {
			node.resolvedDimensions[dim] = &node.Style.Dimensions[dim]
		}
	}
}

// FloatsEqual returns true if floats are approx. equal
func FloatsEqual(a float32, b float32) bool {
	if FloatIsUndefined(a) {
		return FloatIsUndefined(b)
	}
	return fabs(a-b) < 0.0001
}

// see print.go

var (
	leading  = [4]Edge{EdgeTop, EdgeBottom, EdgeLeft, EdgeRight}
	trailing = [4]Edge{EdgeBottom, EdgeTop, EdgeRight, EdgeLeft}
	pos      = [4]Edge{EdgeTop, EdgeBottom, EdgeLeft, EdgeRight}
	dim      = [4]Dimension{DimensionHeight, DimensionHeight, DimensionWidth, DimensionWidth}
)

func init() {
	leading[FlexDirectionColumn] = EdgeTop
	leading[FlexDirectionColumnReverse] = EdgeBottom
	leading[FlexDirectionRow] = EdgeLeft
	leading[FlexDirectionRowReverse] = EdgeRight

	trailing[FlexDirectionColumn] = EdgeBottom
	trailing[FlexDirectionColumnReverse] = EdgeTop
	trailing[FlexDirectionRow] = EdgeRight
	trailing[FlexDirectionRowReverse] = EdgeLeft

	pos[FlexDirectionColumn] = EdgeTop
	pos[FlexDirectionColumnReverse] = EdgeBottom
	pos[FlexDirectionRow] = EdgeLeft
	pos[FlexDirectionRowReverse] = EdgeRight

	dim[FlexDirectionColumn] = DimensionHeight
	dim[FlexDirectionColumnReverse] = DimensionHeight
	dim[FlexDirectionRow] = DimensionWidth
	dim[FlexDirectionRowReverse] = DimensionWidth
}

func flexDirectionIsRow(flexDirection FlexDirection) bool {
	return flexDirection == FlexDirectionRow || flexDirection == FlexDirectionRowReverse
}

func flexDirectionIsColumn(flexDirection FlexDirection) bool {
	return flexDirection == FlexDirectionColumn || flexDirection == FlexDirectionColumnReverse
}

func nodeLeadingMargin(node *Node, axis FlexDirection, widthSize float32) float32 {
	if flexDirectionIsRow(axis) && node.Style.Margin[EdgeStart].Unit != UnitUndefined {
		return resolveValueMargin(&node.Style.Margin[EdgeStart], widthSize)
	}

	v := computedEdgeValue(node.Style.Margin[:], leading[axis], &ValueZero)
	return resolveValueMargin(v, widthSize)
}

func nodeTrailingMargin(node *Node, axis FlexDirection, widthSize float32) float32 {
	if flexDirectionIsRow(axis) && node.Style.Margin[EdgeEnd].Unit != UnitUndefined {
		return resolveValueMargin(&node.Style.Margin[EdgeEnd], widthSize)
	}

	return resolveValueMargin(computedEdgeValue(node.Style.Margin[:], trailing[axis], &ValueZero),
		widthSize)
}

func nodeLeadingPadding(node *Node, axis FlexDirection, widthSize float32) float32 {
	if flexDirectionIsRow(axis) && node.Style.Padding[EdgeStart].Unit != UnitUndefined &&
		resolveValue(&node.Style.Padding[EdgeStart], widthSize) >= 0 {
		return resolveValue(&node.Style.Padding[EdgeStart], widthSize)
	}

	return fmaxf(resolveValue(computedEdgeValue(node.Style.Padding[:], leading[axis], &ValueZero), widthSize), 0)
}

func nodeTrailingPadding(node *Node, axis FlexDirection, widthSize float32) float32 {
	if flexDirectionIsRow(axis) && node.Style.Padding[EdgeEnd].Unit != UnitUndefined &&
		resolveValue(&node.Style.Padding[EdgeEnd], widthSize) >= 0 {
		return resolveValue(&node.Style.Padding[EdgeEnd], widthSize)
	}

	return fmaxf(resolveValue(computedEdgeValue(node.Style.Padding[:], trailing[axis], &ValueZero), widthSize), 0)
}

func nodeLeadingBorder(node *Node, axis FlexDirection) float32 {
	if flexDirectionIsRow(axis) && node.Style.Border[EdgeStart].Unit != UnitUndefined &&
		node.Style.Border[EdgeStart].Value >= 0 {
		return node.Style.Border[EdgeStart].Value
	}

	return fmaxf(computedEdgeValue(node.Style.Border[:], leading[axis], &ValueZero).Value, 0)
}

func nodeTrailingBorder(node *Node, axis FlexDirection) float32 {
	if flexDirectionIsRow(axis) && node.Style.Border[EdgeEnd].Unit != UnitUndefined &&
		node.Style.Border[EdgeEnd].Value >= 0 {
		return node.Style.Border[EdgeEnd].Value
	}

	return fmaxf(computedEdgeValue(node.Style.Border[:], trailing[axis], &ValueZero).Value, 0)
}

func nodeLeadingPaddingAndBorder(node *Node, axis FlexDirection, widthSize float32) float32 {
	return nodeLeadingPadding(node, axis, widthSize) + nodeLeadingBorder(node, axis)
}

func nodeTrailingPaddingAndBorder(node *Node, axis FlexDirection, widthSize float32) float32 {
	return nodeTrailingPadding(node, axis, widthSize) + nodeTrailingBorder(node, axis)
}

func nodeMarginForAxis(node *Node, axis FlexDirection, widthSize float32) float32 {
	leading := nodeLeadingMargin(node, axis, widthSize)
	trailing := nodeTrailingMargin(node, axis, widthSize)
	return leading + trailing
}

func nodePaddingAndBorderForAxis(node *Node, axis FlexDirection, widthSize float32) float32 {
	return nodeLeadingPaddingAndBorder(node, axis, widthSize) +
		nodeTrailingPaddingAndBorder(node, axis, widthSize)
}

func nodeAlignItem(node *Node, child *Node) Align {
	align := child.Style.AlignSelf
	if child.Style.AlignSelf == AlignAuto {
		align = node.Style.AlignItems
	}
	if align == AlignBaseline && flexDirectionIsColumn(node.Style.FlexDirection) {
		return AlignFlexStart
	}
	return align
}

func nodeResolveDirection(node *Node, parentDirection Direction) Direction {
	if node.Style.Direction == DirectionInherit {
		if parentDirection > DirectionInherit {
			return parentDirection
		}
		return DirectionLTR
	}
	return node.Style.Direction
}

// Baseline retuns baseline
func Baseline(node *Node) float32 {
	if node.Baseline != nil {
		baseline := node.Baseline(node, node.Layout.measuredDimensions[DimensionWidth], node.Layout.measuredDimensions[DimensionHeight])
		assertWithNode(node, !FloatIsUndefined(baseline), "Expect custom baseline function to not return NaN")
		return baseline
	}

	var baselineChild *Node
	childCount := len(node.Children)
	for i := 0; i < childCount; i++ {
		child := node.GetChild(i)
		if child.lineIndex > 0 {
			break
		}
		if child.Style.PositionType == PositionTypeAbsolute {
			continue
		}
		if nodeAlignItem(node, child) == AlignBaseline {
			baselineChild = child
			break
		}

		if baselineChild == nil {
			baselineChild = child
		}
	}

	if baselineChild == nil {
		return node.Layout.measuredDimensions[DimensionHeight]
	}

	baseline := Baseline(baselineChild)
	return baseline + baselineChild.Layout.Position[EdgeTop]
}

func resolveFlexDirection(flexDirection FlexDirection, direction Direction) FlexDirection {
	if direction == DirectionRTL {
		if flexDirection == FlexDirectionRow {
			return FlexDirectionRowReverse
		} else if flexDirection == FlexDirectionRowReverse {
			return FlexDirectionRow
		}
	}
	return flexDirection
}

func flexDirectionCross(flexDirection FlexDirection, direction Direction) FlexDirection {
	if flexDirectionIsColumn(flexDirection) {
		return resolveFlexDirection(FlexDirectionRow, direction)
	}
	return FlexDirectionColumn
}

func nodeIsFlex(node *Node) bool {
	return (node.Style.PositionType == PositionTypeRelative &&
		(resolveFlexGrow(node) != 0 || nodeResolveFlexShrink(node) != 0))
}

func isBaselineLayout(node *Node) bool {
	if flexDirectionIsColumn(node.Style.FlexDirection) {
		return false
	}
	if node.Style.AlignItems == AlignBaseline {
		return true
	}
	childCount := len(node.Children)
	for i := 0; i < childCount; i++ {
		child := node.GetChild(i)
		if child.Style.PositionType == PositionTypeRelative &&
			child.Style.AlignSelf == AlignBaseline {
			return true
		}
	}

	return false
}

func nodeDimWithMargin(node *Node, axis FlexDirection, widthSize float32) float32 {
	return node.Layout.measuredDimensions[dim[axis]] + nodeLeadingMargin(node, axis, widthSize) +
		nodeTrailingMargin(node, axis, widthSize)
}

func nodeIsStyleDimDefined(node *Node, axis FlexDirection, parentSize float32) bool {
	v := node.resolvedDimensions[dim[axis]]
	isNotDefined := (v.Unit == UnitAuto ||
		v.Unit == UnitUndefined ||
		(v.Unit == UnitPoint && v.Value < 0) ||
		(v.Unit == UnitPercent && (v.Value < 0 || FloatIsUndefined(parentSize))))
	return !isNotDefined
}

func nodeIsLayoutDimDefined(node *Node, axis FlexDirection) bool {
	value := node.Layout.measuredDimensions[dim[axis]]
	return !FloatIsUndefined(value) && value >= 0
}

func nodeIsLeadingPosDefined(node *Node, axis FlexDirection) bool {
	return (flexDirectionIsRow(axis) &&
		computedEdgeValue(node.Style.Position[:], EdgeStart, &ValueUndefined).Unit !=
			UnitUndefined) ||
		computedEdgeValue(node.Style.Position[:], leading[axis], &ValueUndefined).Unit !=
			UnitUndefined
}

func nodeIsTrailingPosDefined(node *Node, axis FlexDirection) bool {
	return (flexDirectionIsRow(axis) &&
		computedEdgeValue(node.Style.Position[:], EdgeEnd, &ValueUndefined).Unit !=
			UnitUndefined) ||
		computedEdgeValue(node.Style.Position[:], trailing[axis], &ValueUndefined).Unit !=
			UnitUndefined
}

func nodeLeadingPosition(node *Node, axis FlexDirection, axisSize float32) float32 {
	if flexDirectionIsRow(axis) {
		leadingPosition := computedEdgeValue(node.Style.Position[:], EdgeStart, &ValueUndefined)
		if leadingPosition.Unit != UnitUndefined {
			return resolveValue(leadingPosition, axisSize)
		}
	}

	leadingPosition := computedEdgeValue(node.Style.Position[:], leading[axis], &ValueUndefined)

	if leadingPosition.Unit == UnitUndefined {
		return 0
	}
	return resolveValue(leadingPosition, axisSize)
}

func nodeTrailingPosition(node *Node, axis FlexDirection, axisSize float32) float32 {
	if flexDirectionIsRow(axis) {
		trailingPosition := computedEdgeValue(node.Style.Position[:], EdgeEnd, &ValueUndefined)
		if trailingPosition.Unit != UnitUndefined {
			return resolveValue(trailingPosition, axisSize)
		}
	}

	trailingPosition := computedEdgeValue(node.Style.Position[:], trailing[axis], &ValueUndefined)

	if trailingPosition.Unit == UnitUndefined {
		return 0
	}
	return resolveValue(trailingPosition, axisSize)
}

func nodeBoundAxisWithinMinAndMax(node *Node, axis FlexDirection, value float32, axisSize float32) float32 {
	min := Undefined
	max := Undefined

	if flexDirectionIsColumn(axis) {
		min = resolveValue(&node.Style.MinDimensions[DimensionHeight], axisSize)
		max = resolveValue(&node.Style.MaxDimensions[DimensionHeight], axisSize)
	} else if flexDirectionIsRow(axis) {
		min = resolveValue(&node.Style.MinDimensions[DimensionWidth], axisSize)
		max = resolveValue(&node.Style.MaxDimensions[DimensionWidth], axisSize)
	}

	boundValue := value

	if !FloatIsUndefined(max) && max >= 0 && boundValue > max {
		boundValue = max
	}

	if !FloatIsUndefined(min) && min >= 0 && boundValue < min {
		boundValue = min
	}

	return boundValue
}

func marginLeadingValue(node *Node, axis FlexDirection) *Value {
	if flexDirectionIsRow(axis) && node.Style.Margin[EdgeStart].Unit != UnitUndefined {
		return &node.Style.Margin[EdgeStart]
	}
	return &node.Style.Margin[leading[axis]]
}

func marginTrailingValue(node *Node, axis FlexDirection) *Value {
	if flexDirectionIsRow(axis) && node.Style.Margin[EdgeEnd].Unit != UnitUndefined {
		return &node.Style.Margin[EdgeEnd]
	}
	return &node.Style.Margin[trailing[axis]]

}

// nodeBoundAxis is like nodeBoundAxisWithinMinAndMax but also ensures that
// the value doesn't go below the padding and border amount.
func nodeBoundAxis(node *Node, axis FlexDirection, value float32, axisSize float32, widthSize float32) float32 {
	return fmaxf(nodeBoundAxisWithinMinAndMax(node, axis, value, axisSize),
		nodePaddingAndBorderForAxis(node, axis, widthSize))
}

func nodeSetChildTrailingPosition(node *Node, child *Node, axis FlexDirection) {
	size := child.Layout.measuredDimensions[dim[axis]]
	child.Layout.Position[trailing[axis]] =
		node.Layout.measuredDimensions[dim[axis]] - size - child.Layout.Position[pos[axis]]
}

// If both left and right are defined, then use left. Otherwise return
// +left or -right depending on which is defined.
func nodeRelativePosition(node *Node, axis FlexDirection, axisSize float32) float32 {
	if nodeIsLeadingPosDefined(node, axis) {
		return nodeLeadingPosition(node, axis, axisSize)
	}
	return -nodeTrailingPosition(node, axis, axisSize)
}

func constrainMaxSizeForMode(node *Node, axis FlexDirection, parentAxisSize float32, parentWidth float32, mode *MeasureMode, size *float32) {
	maxSize := resolveValue(&node.Style.MaxDimensions[dim[axis]], parentAxisSize) +
		nodeMarginForAxis(node, axis, parentWidth)
	switch *mode {
	case MeasureModeExactly, MeasureModeAtMost:
		if FloatIsUndefined(maxSize) || *size < maxSize {
			// TODO: this is redundant, but what is in original code
			//*size = *size
		} else {
			*size = maxSize
		}
	case MeasureModeUndefined:
		if !FloatIsUndefined(maxSize) {
			*mode = MeasureModeAtMost
			*size = maxSize
		}
	}
}

func nodeSetPosition(node *Node, direction Direction, mainSize float32, crossSize float32, parentWidth float32) {
	/* Root nodes should be always layouted as LTR, so we don't return negative values. */
	directionRespectingRoot := DirectionLTR
	if node.Parent != nil {
		directionRespectingRoot = direction
	}

	mainAxis := resolveFlexDirection(node.Style.FlexDirection, directionRespectingRoot)
	crossAxis := flexDirectionCross(mainAxis, directionRespectingRoot)

	relativePositionMain := nodeRelativePosition(node, mainAxis, mainSize)
	relativePositionCross := nodeRelativePosition(node, crossAxis, crossSize)

	pos := &node.Layout.Position
	pos[leading[mainAxis]] = nodeLeadingMargin(node, mainAxis, parentWidth) + relativePositionMain
	pos[trailing[mainAxis]] = nodeTrailingMargin(node, mainAxis, parentWidth) + relativePositionMain
	pos[leading[crossAxis]] = nodeLeadingMargin(node, crossAxis, parentWidth) + relativePositionCross
	pos[trailing[crossAxis]] = nodeTrailingMargin(node, crossAxis, parentWidth) + relativePositionCross
}

func nodeComputeFlexBasisForChild(node *Node,
	child *Node,
	width float32,
	widthMode MeasureMode,
	height float32,
	parentWidth float32,
	parentHeight float32,
	heightMode MeasureMode,
	direction Direction,
	config *Config) {
	mainAxis := resolveFlexDirection(node.Style.FlexDirection, direction)
	isMainAxisRow := flexDirectionIsRow(mainAxis)
	mainAxisSize := height
	mainAxisParentSize := parentHeight
	if isMainAxisRow {
		mainAxisSize = width
		mainAxisParentSize = parentWidth
	}

	var childWidth float32
	var childHeight float32
	var childWidthMeasureMode MeasureMode
	var childHeightMeasureMode MeasureMode

	resolvedFlexBasis := resolveValue(nodeResolveFlexBasisPtr(child), mainAxisParentSize)
	isRowStyleDimDefined := nodeIsStyleDimDefined(child, FlexDirectionRow, parentWidth)
	isColumnStyleDimDefined := nodeIsStyleDimDefined(child, FlexDirectionColumn, parentHeight)

	if !FloatIsUndefined(resolvedFlexBasis) && !FloatIsUndefined(mainAxisSize) {
		if FloatIsUndefined(child.Layout.computedFlexBasis) ||
			(child.Config.IsExperimentalFeatureEnabled(ExperimentalFeatureWebFlexBasis) &&
				child.Layout.computedFlexBasisGeneration != currentGenerationCount) {
			child.Layout.computedFlexBasis =
				fmaxf(resolvedFlexBasis, nodePaddingAndBorderForAxis(child, mainAxis, parentWidth))
		}
	} else if isMainAxisRow && isRowStyleDimDefined {
		// The width is definite, so use that as the flex basis.
		child.Layout.computedFlexBasis =
			fmaxf(resolveValue(child.resolvedDimensions[DimensionWidth], parentWidth),
				nodePaddingAndBorderForAxis(child, FlexDirectionRow, parentWidth))
	} else if !isMainAxisRow && isColumnStyleDimDefined {
		// The height is definite, so use that as the flex basis.
		child.Layout.computedFlexBasis =
			fmaxf(resolveValue(child.resolvedDimensions[DimensionHeight], parentHeight),
				nodePaddingAndBorderForAxis(child, FlexDirectionColumn, parentWidth))
	} else {
		// Compute the flex basis and hypothetical main size (i.e. the clamped
		// flex basis).
		childWidth = Undefined
		childHeight = Undefined
		childWidthMeasureMode = MeasureModeUndefined
		childHeightMeasureMode = MeasureModeUndefined

		marginRow := nodeMarginForAxis(child, FlexDirectionRow, parentWidth)
		marginColumn := nodeMarginForAxis(child, FlexDirectionColumn, parentWidth)

		if isRowStyleDimDefined {
			childWidth =
				resolveValue(child.resolvedDimensions[DimensionWidth], parentWidth) + marginRow
			childWidthMeasureMode = MeasureModeExactly
		}
		if isColumnStyleDimDefined {
			childHeight =
				resolveValue(child.resolvedDimensions[DimensionHeight], parentHeight) + marginColumn
			childHeightMeasureMode = MeasureModeExactly
		}

		// The W3C spec doesn't say anything about the 'overflow' property,
		// but all major browsers appear to implement the following logic.
		if (!isMainAxisRow && node.Style.Overflow == OverflowScroll) ||
			node.Style.Overflow != OverflowScroll {
			if FloatIsUndefined(childWidth) && !FloatIsUndefined(width) {
				childWidth = width
				childWidthMeasureMode = MeasureModeAtMost
			}
		}

		if (isMainAxisRow && node.Style.Overflow == OverflowScroll) ||
			node.Style.Overflow != OverflowScroll {
			if FloatIsUndefined(childHeight) && !FloatIsUndefined(height) {
				childHeight = height
				childHeightMeasureMode = MeasureModeAtMost
			}
		}

		// If child has no defined size in the cross axis and is set to stretch,
		// set the cross
		// axis to be measured exactly with the available inner width
		if !isMainAxisRow && !FloatIsUndefined(width) && !isRowStyleDimDefined &&
			widthMode == MeasureModeExactly && nodeAlignItem(node, child) == AlignStretch {
			childWidth = width
			childWidthMeasureMode = MeasureModeExactly
		}
		if isMainAxisRow && !FloatIsUndefined(height) && !isColumnStyleDimDefined &&
			heightMode == MeasureModeExactly && nodeAlignItem(node, child) == AlignStretch {
			childHeight = height
			childHeightMeasureMode = MeasureModeExactly
		}

		if !FloatIsUndefined(child.Style.AspectRatio) {
			if !isMainAxisRow && childWidthMeasureMode == MeasureModeExactly {
				child.Layout.computedFlexBasis =
					fmaxf((childWidth-marginRow)/child.Style.AspectRatio,
						nodePaddingAndBorderForAxis(child, FlexDirectionColumn, parentWidth))
				return
			} else if isMainAxisRow && childHeightMeasureMode == MeasureModeExactly {
				child.Layout.computedFlexBasis =
					fmaxf((childHeight-marginColumn)*child.Style.AspectRatio,
						nodePaddingAndBorderForAxis(child, FlexDirectionRow, parentWidth))
				return
			}
		}

		constrainMaxSizeForMode(
			child, FlexDirectionRow, parentWidth, parentWidth, &childWidthMeasureMode, &childWidth)
		constrainMaxSizeForMode(child,
			FlexDirectionColumn,
			parentHeight,
			parentWidth,
			&childHeightMeasureMode,
			&childHeight)

		// Measure the child
		layoutNodeInternal(child,
			childWidth,
			childHeight,
			direction,
			childWidthMeasureMode,
			childHeightMeasureMode,
			parentWidth,
			parentHeight,
			false,
			"measure",
			config)

		child.Layout.computedFlexBasis =
			fmaxf(child.Layout.measuredDimensions[dim[mainAxis]],
				nodePaddingAndBorderForAxis(child, mainAxis, parentWidth))
	}

	child.Layout.computedFlexBasisGeneration = currentGenerationCount
}

func nodeAbsoluteLayoutChild(node *Node, child *Node, width float32, widthMode MeasureMode, height float32, direction Direction, config *Config) {
	mainAxis := resolveFlexDirection(node.Style.FlexDirection, direction)
	crossAxis := flexDirectionCross(mainAxis, direction)
	isMainAxisRow := flexDirectionIsRow(mainAxis)

	childWidth := Undefined
	childHeight := Undefined
	childWidthMeasureMode := MeasureModeUndefined
	childHeightMeasureMode := MeasureModeUndefined

	marginRow := nodeMarginForAxis(child, FlexDirectionRow, width)
	marginColumn := nodeMarginForAxis(child, FlexDirectionColumn, width)

	if nodeIsStyleDimDefined(child, FlexDirectionRow, width) {
		childWidth = resolveValue(child.resolvedDimensions[DimensionWidth], width) + marginRow
	} else {
		// If the child doesn't have a specified width, compute the width based
		// on the left/right
		// offsets if they're defined.
		if nodeIsLeadingPosDefined(child, FlexDirectionRow) &&
			nodeIsTrailingPosDefined(child, FlexDirectionRow) {
			childWidth = node.Layout.measuredDimensions[DimensionWidth] -
				(nodeLeadingBorder(node, FlexDirectionRow) +
					nodeTrailingBorder(node, FlexDirectionRow)) -
				(nodeLeadingPosition(child, FlexDirectionRow, width) +
					nodeTrailingPosition(child, FlexDirectionRow, width))
			childWidth = nodeBoundAxis(child, FlexDirectionRow, childWidth, width, width)
		}
	}

	if nodeIsStyleDimDefined(child, FlexDirectionColumn, height) {
		childHeight =
			resolveValue(child.resolvedDimensions[DimensionHeight], height) + marginColumn
	} else {
		// If the child doesn't have a specified height, compute the height
		// based on the top/bottom
		// offsets if they're defined.
		if nodeIsLeadingPosDefined(child, FlexDirectionColumn) &&
			nodeIsTrailingPosDefined(child, FlexDirectionColumn) {
			childHeight = node.Layout.measuredDimensions[DimensionHeight] -
				(nodeLeadingBorder(node, FlexDirectionColumn) +
					nodeTrailingBorder(node, FlexDirectionColumn)) -
				(nodeLeadingPosition(child, FlexDirectionColumn, height) +
					nodeTrailingPosition(child, FlexDirectionColumn, height))
			childHeight = nodeBoundAxis(child, FlexDirectionColumn, childHeight, height, width)
		}
	}

	// Exactly one dimension needs to be defined for us to be able to do aspect ratio
	// calculation. One dimension being the anchor and the other being flexible.
	if FloatIsUndefined(childWidth) != FloatIsUndefined(childHeight) {
		if !FloatIsUndefined(child.Style.AspectRatio) {
			if FloatIsUndefined(childWidth) {
				childWidth =
					marginRow + fmaxf((childHeight-marginColumn)*child.Style.AspectRatio,
						nodePaddingAndBorderForAxis(child, FlexDirectionColumn, width))
			} else if FloatIsUndefined(childHeight) {
				childHeight =
					marginColumn + fmaxf((childWidth-marginRow)/child.Style.AspectRatio,
						nodePaddingAndBorderForAxis(child, FlexDirectionRow, width))
			}
		}
	}

	// If we're still missing one or the other dimension, measure the content.
	if FloatIsUndefined(childWidth) || FloatIsUndefined(childHeight) {
		childWidthMeasureMode = MeasureModeExactly
		if FloatIsUndefined(childWidth) {
			childWidthMeasureMode = MeasureModeUndefined
		}
		childHeightMeasureMode = MeasureModeExactly
		if FloatIsUndefined(childHeight) {
			childHeightMeasureMode = MeasureModeUndefined
		}

		// If the size of the parent is defined then try to rain the absolute child to that size
		// as well. This allows text within the absolute child to wrap to the size of its parent.
		// This is the same behavior as many browsers implement.
		if !isMainAxisRow && FloatIsUndefined(childWidth) && widthMode != MeasureModeUndefined &&
			width > 0 {
			childWidth = width
			childWidthMeasureMode = MeasureModeAtMost
		}

		layoutNodeInternal(child,
			childWidth,
			childHeight,
			direction,
			childWidthMeasureMode,
			childHeightMeasureMode,
			childWidth,
			childHeight,
			false,
			"abs-measure",
			config)
		childWidth = child.Layout.measuredDimensions[DimensionWidth] +
			nodeMarginForAxis(child, FlexDirectionRow, width)
		childHeight = child.Layout.measuredDimensions[DimensionHeight] +
			nodeMarginForAxis(child, FlexDirectionColumn, width)
	}

	layoutNodeInternal(child,
		childWidth,
		childHeight,
		direction,
		MeasureModeExactly,
		MeasureModeExactly,
		childWidth,
		childHeight,
		true,
		"abs-layout",
		config)

	if nodeIsTrailingPosDefined(child, mainAxis) && !nodeIsLeadingPosDefined(child, mainAxis) {
		axisSize := height
		if isMainAxisRow {
			axisSize = width
		}
		child.Layout.Position[leading[mainAxis]] = node.Layout.measuredDimensions[dim[mainAxis]] -
			child.Layout.measuredDimensions[dim[mainAxis]] -
			nodeTrailingBorder(node, mainAxis) -
			nodeTrailingMargin(child, mainAxis, width) -
			nodeTrailingPosition(child, mainAxis, axisSize)
	} else if !nodeIsLeadingPosDefined(child, mainAxis) &&
		node.Style.JustifyContent == JustifyCenter {
		child.Layout.Position[leading[mainAxis]] = (node.Layout.measuredDimensions[dim[mainAxis]] -
			child.Layout.measuredDimensions[dim[mainAxis]]) /
			2.0
	} else if !nodeIsLeadingPosDefined(child, mainAxis) &&
		node.Style.JustifyContent == JustifyFlexEnd {
		child.Layout.Position[leading[mainAxis]] = (node.Layout.measuredDimensions[dim[mainAxis]] -
			child.Layout.measuredDimensions[dim[mainAxis]])
	}

	if nodeIsTrailingPosDefined(child, crossAxis) &&
		!nodeIsLeadingPosDefined(child, crossAxis) {
		axisSize := width
		if isMainAxisRow {
			axisSize = height
		}

		child.Layout.Position[leading[crossAxis]] = node.Layout.measuredDimensions[dim[crossAxis]] -
			child.Layout.measuredDimensions[dim[crossAxis]] -
			nodeTrailingBorder(node, crossAxis) -
			nodeTrailingMargin(child, crossAxis, width) -
			nodeTrailingPosition(child, crossAxis, axisSize)
	} else if !nodeIsLeadingPosDefined(child, crossAxis) &&
		nodeAlignItem(node, child) == AlignCenter {
		child.Layout.Position[leading[crossAxis]] =
			(node.Layout.measuredDimensions[dim[crossAxis]] -
				child.Layout.measuredDimensions[dim[crossAxis]]) /
				2.0
	} else if !nodeIsLeadingPosDefined(child, crossAxis) &&
		((nodeAlignItem(node, child) == AlignFlexEnd) != (node.Style.FlexWrap == WrapWrapReverse)) {
		child.Layout.Position[leading[crossAxis]] = (node.Layout.measuredDimensions[dim[crossAxis]] -
			child.Layout.measuredDimensions[dim[crossAxis]])
	}
}

// nodeWithMeasureFuncSetMeasuredDimensions sets measure dimensions for node with measure func
func nodeWithMeasureFuncSetMeasuredDimensions(node *Node, availableWidth float32, availableHeight float32, widthMeasureMode MeasureMode, heightMeasureMode MeasureMode, parentWidth float32, parentHeight float32) {
	assertWithNode(node, node.Measure != nil, "Expected node to have custom measure function")

	paddingAndBorderAxisRow := nodePaddingAndBorderForAxis(node, FlexDirectionRow, availableWidth)
	paddingAndBorderAxisColumn := nodePaddingAndBorderForAxis(node, FlexDirectionColumn, availableWidth)
	marginAxisRow := nodeMarginForAxis(node, FlexDirectionRow, availableWidth)
	marginAxisColumn := nodeMarginForAxis(node, FlexDirectionColumn, availableWidth)

	// We want to make sure we don't call measure with negative size
	innerWidth := fmaxf(0, availableWidth-marginAxisRow-paddingAndBorderAxisRow)
	if FloatIsUndefined(availableWidth) {
		innerWidth = availableWidth
	}
	innerHeight := fmaxf(0, availableHeight-marginAxisColumn-paddingAndBorderAxisColumn)
	if FloatIsUndefined(availableHeight) {
		innerHeight = availableHeight
	}

	if widthMeasureMode == MeasureModeExactly && heightMeasureMode == MeasureModeExactly {
		// Don't bother sizing the text if both dimensions are already defined.
		node.Layout.measuredDimensions[DimensionWidth] = nodeBoundAxis(
			node, FlexDirectionRow, availableWidth-marginAxisRow, parentWidth, parentWidth)
		node.Layout.measuredDimensions[DimensionHeight] = nodeBoundAxis(
			node, FlexDirectionColumn, availableHeight-marginAxisColumn, parentHeight, parentWidth)
	} else {
		// Measure the text under the current raints.
		measuredSize := node.Measure(node, innerWidth, widthMeasureMode, innerHeight, heightMeasureMode)

		width := availableWidth - marginAxisRow
		if widthMeasureMode == MeasureModeUndefined ||
			widthMeasureMode == MeasureModeAtMost {
			width = measuredSize.Width + paddingAndBorderAxisRow

		}

		node.Layout.measuredDimensions[DimensionWidth] = nodeBoundAxis(node, FlexDirectionRow, width, availableWidth, availableWidth)

		height := availableHeight - marginAxisColumn
		if heightMeasureMode == MeasureModeUndefined ||
			heightMeasureMode == MeasureModeAtMost {
			height = measuredSize.Height + paddingAndBorderAxisColumn
		}

		node.Layout.measuredDimensions[DimensionHeight] = nodeBoundAxis(node, FlexDirectionColumn, height, availableHeight, availableWidth)
	}
}

// nodeEmptyContainerSetMeasuredDimensions sets measure dimensions for empty container
// For nodes with no children, use the available values if they were provided,
// or the minimum size as indicated by the padding and border sizes.
func nodeEmptyContainerSetMeasuredDimensions(node *Node, availableWidth float32, availableHeight float32, widthMeasureMode MeasureMode, heightMeasureMode MeasureMode, parentWidth float32, parentHeight float32) {
	paddingAndBorderAxisRow := nodePaddingAndBorderForAxis(node, FlexDirectionRow, parentWidth)
	paddingAndBorderAxisColumn := nodePaddingAndBorderForAxis(node, FlexDirectionColumn, parentWidth)
	marginAxisRow := nodeMarginForAxis(node, FlexDirectionRow, parentWidth)
	marginAxisColumn := nodeMarginForAxis(node, FlexDirectionColumn, parentWidth)

	width := availableWidth - marginAxisRow
	if widthMeasureMode == MeasureModeUndefined || widthMeasureMode == MeasureModeAtMost {
		width = paddingAndBorderAxisRow
	}
	node.Layout.measuredDimensions[DimensionWidth] = nodeBoundAxis(node, FlexDirectionRow, width, parentWidth, parentWidth)

	height := availableHeight - marginAxisColumn
	if heightMeasureMode == MeasureModeUndefined || heightMeasureMode == MeasureModeAtMost {
		height = paddingAndBorderAxisColumn
	}
	node.Layout.measuredDimensions[DimensionHeight] = nodeBoundAxis(node, FlexDirectionColumn, height, parentHeight, parentWidth)
}

func nodeFixedSizeSetMeasuredDimensions(node *Node,
	availableWidth float32,
	availableHeight float32,
	widthMeasureMode MeasureMode,
	heightMeasureMode MeasureMode,
	parentWidth float32,
	parentHeight float32) bool {
	if (widthMeasureMode == MeasureModeAtMost && availableWidth <= 0) ||
		(heightMeasureMode == MeasureModeAtMost && availableHeight <= 0) ||
		(widthMeasureMode == MeasureModeExactly && heightMeasureMode == MeasureModeExactly) {
		marginAxisColumn := nodeMarginForAxis(node, FlexDirectionColumn, parentWidth)
		marginAxisRow := nodeMarginForAxis(node, FlexDirectionRow, parentWidth)

		width := availableWidth - marginAxisRow
		if FloatIsUndefined(availableWidth) || (widthMeasureMode == MeasureModeAtMost && availableWidth < 0) {
			width = 0
		}
		node.Layout.measuredDimensions[DimensionWidth] =
			nodeBoundAxis(node, FlexDirectionRow, width, parentWidth, parentWidth)

		height := availableHeight - marginAxisColumn
		if FloatIsUndefined(availableHeight) || (heightMeasureMode == MeasureModeAtMost && availableHeight < 0) {
			height = 0
		}
		node.Layout.measuredDimensions[DimensionHeight] =
			nodeBoundAxis(node, FlexDirectionColumn, height, parentHeight, parentWidth)

		return true
	}

	return false
}

// zeroOutLayoutRecursivly zeros out layout recursively
func zeroOutLayoutRecursivly(node *Node) {
	node.Layout.Dimensions[DimensionHeight] = 0
	node.Layout.Dimensions[DimensionWidth] = 0
	node.Layout.Position[EdgeTop] = 0
	node.Layout.Position[EdgeBottom] = 0
	node.Layout.Position[EdgeLeft] = 0
	node.Layout.Position[EdgeRight] = 0
	node.Layout.cachedLayout.availableHeight = 0
	node.Layout.cachedLayout.availableWidth = 0
	node.Layout.cachedLayout.heightMeasureMode = MeasureModeExactly
	node.Layout.cachedLayout.widthMeasureMode = MeasureModeExactly
	node.Layout.cachedLayout.computedWidth = 0
	node.Layout.cachedLayout.computedHeight = 0
	node.hasNewLayout = true
	childCount := len(node.Children)
	for i := 0; i < childCount; i++ {
		child := node.Children[i]
		zeroOutLayoutRecursivly(child)
	}
}

// This is the main routine that implements a subset of the flexbox layout
// algorithm
// described in the W3C YG documentation: https://www.w3.org/TR/YG3-flexbox/.
//
// Limitations of this algorithm, compared to the full standard:
//  * Display property is always assumed to be 'flex' except for Text nodes,
//  which
//    are assumed to be 'inline-flex'.
//  * The 'zIndex' property (or any form of z ordering) is not supported. Nodes
//  are
//    stacked in document order.
//  * The 'order' property is not supported. The order of flex items is always
//  defined
//    by document order.
//  * The 'visibility' property is always assumed to be 'visible'. Values of
//  'collapse'
//    and 'hidden' are not supported.
//  * There is no support for forced breaks.
//  * It does not support vertical inline directions (top-to-bottom or
//  bottom-to-top text).
//
// Deviations from standard:
//  * Section 4.5 of the spec indicates that all flex items have a default
//  minimum
//    main size. For text blocks, for example, this is the width of the widest
//    word.
//    Calculating the minimum width is expensive, so we forego it and assume a
//    default
//    minimum main size of 0.
//  * Min/Max sizes in the main axis are not honored when resolving flexible
//  lengths.
//  * The spec indicates that the default value for 'flexDirection' is 'row',
//  but
//    the algorithm below assumes a default of 'column'.
//
// Input parameters:
//    - node: current node to be sized and layed out
//    - availableWidth & availableHeight: available size to be used for sizing
//    the node
//      or Undefined if the size is not available; interpretation depends on
//      layout
//      flags
//    - parentDirection: the inline (text) direction within the parent
//    (left-to-right or
//      right-to-left)
//    - widthMeasureMode: indicates the sizing rules for the width (see below
//    for explanation)
//    - heightMeasureMode: indicates the sizing rules for the height (see below
//    for explanation)
//    - performLayout: specifies whether the caller is interested in just the
//    dimensions
//      of the node or it requires the entire node and its subtree to be layed
//      out
//      (with final positions)
//
// Details:
//    This routine is called recursively to lay out subtrees of flexbox
//    elements. It uses the
//    information in node.style, which is treated as a read-only input. It is
//    responsible for
//    setting the layout.direction and layout.measuredDimensions fields for the
//    input node as well
//    as the layout.position and layout.lineIndex fields for its child nodes.
//    The
//    layout.measuredDimensions field includes any border or padding for the
//    node but does
//    not include margins.
//
//    The spec describes four different layout modes: "fill available", "max
//    content", "min
//    content",
//    and "fit content". Of these, we don't use "min content" because we don't
//    support default
//    minimum main sizes (see above for details). Each of our measure modes maps
//    to a layout mode
//    from the spec (https://www.w3.org/TR/YG3-sizing/#terms):
//      - YGMeasureModeUndefined: max content
//      - YGMeasureModeExactly: fill available
//      - YGMeasureModeAtMost: fit content
//
//    When calling nodelayoutImpl and YGLayoutNodeInternal, if the caller passes
//    an available size of
//    undefined then it must also pass a measure mode of YGMeasureModeUndefined
//    in that dimension.
func nodelayoutImpl(node *Node, availableWidth float32, availableHeight float32,
	parentDirection Direction, widthMeasureMode MeasureMode,
	heightMeasureMode MeasureMode, parentWidth float32, parentHeight float32,
	performLayout bool, config *Config) {
	// assertWithNode(node, YGFloatIsUndefined(availableWidth) ? widthMeasureMode == YGMeasureModeUndefined : true, "availableWidth is indefinite so widthMeasureMode must be YGMeasureModeUndefined");
	//assertWithNode(node, YGFloatIsUndefined(availableHeight) ? heightMeasureMode == YGMeasureModeUndefined : true, "availableHeight is indefinite so heightMeasureMode must be YGMeasureModeUndefined");

	// Set the resolved resolution in the node's layout.
	direction := nodeResolveDirection(node, parentDirection)
	node.Layout.Direction = direction

	flexRowDirection := resolveFlexDirection(FlexDirectionRow, direction)
	flexColumnDirection := resolveFlexDirection(FlexDirectionColumn, direction)

	node.Layout.Margin[EdgeStart] = nodeLeadingMargin(node, flexRowDirection, parentWidth)
	node.Layout.Margin[EdgeEnd] = nodeTrailingMargin(node, flexRowDirection, parentWidth)
	node.Layout.Margin[EdgeTop] = nodeLeadingMargin(node, flexColumnDirection, parentWidth)
	node.Layout.Margin[EdgeBottom] = nodeTrailingMargin(node, flexColumnDirection, parentWidth)

	node.Layout.Border[EdgeStart] = nodeLeadingBorder(node, flexRowDirection)
	node.Layout.Border[EdgeEnd] = nodeTrailingBorder(node, flexRowDirection)
	node.Layout.Border[EdgeTop] = nodeLeadingBorder(node, flexColumnDirection)
	node.Layout.Border[EdgeBottom] = nodeTrailingBorder(node, flexColumnDirection)

	node.Layout.Padding[EdgeStart] = nodeLeadingPadding(node, flexRowDirection, parentWidth)
	node.Layout.Padding[EdgeEnd] = nodeTrailingPadding(node, flexRowDirection, parentWidth)
	node.Layout.Padding[EdgeTop] = nodeLeadingPadding(node, flexColumnDirection, parentWidth)
	node.Layout.Padding[EdgeBottom] = nodeTrailingPadding(node, flexColumnDirection, parentWidth)

	if node.Measure != nil {
		nodeWithMeasureFuncSetMeasuredDimensions(node, availableWidth, availableHeight, widthMeasureMode, heightMeasureMode, parentWidth, parentHeight)
		return
	}

	childCount := len(node.Children)
	if childCount == 0 {
		nodeEmptyContainerSetMeasuredDimensions(node, availableWidth, availableHeight, widthMeasureMode, heightMeasureMode, parentWidth, parentHeight)
		return
	}

	// If we're not being asked to perform a full layout we can skip the algorithm if we already know
	// the size
	if !performLayout && nodeFixedSizeSetMeasuredDimensions(node, availableWidth, availableHeight, widthMeasureMode, heightMeasureMode, parentWidth, parentHeight) {
		return
	}

	// Reset layout flags, as they could have changed.
	node.Layout.HadOverflow = false

	// STEP 1: CALCULATE VALUES FOR REMAINDER OF ALGORITHM
	mainAxis := resolveFlexDirection(node.Style.FlexDirection, direction)
	crossAxis := flexDirectionCross(mainAxis, direction)
	isMainAxisRow := flexDirectionIsRow(mainAxis)
	justifyContent := node.Style.JustifyContent
	isNodeFlexWrap := node.Style.FlexWrap != WrapNoWrap

	mainAxisParentSize := parentHeight
	crossAxisParentSize := parentWidth
	if isMainAxisRow {
		mainAxisParentSize = parentWidth
		crossAxisParentSize = parentHeight
	}

	var firstAbsoluteChild *Node
	var currentAbsoluteChild *Node

	leadingPaddingAndBorderMain := nodeLeadingPaddingAndBorder(node, mainAxis, parentWidth)
	trailingPaddingAndBorderMain := nodeTrailingPaddingAndBorder(node, mainAxis, parentWidth)
	leadingPaddingAndBorderCross := nodeLeadingPaddingAndBorder(node, crossAxis, parentWidth)
	paddingAndBorderAxisMain := nodePaddingAndBorderForAxis(node, mainAxis, parentWidth)
	paddingAndBorderAxisCross := nodePaddingAndBorderForAxis(node, crossAxis, parentWidth)

	measureModeMainDim := heightMeasureMode
	measureModeCrossDim := widthMeasureMode

	if isMainAxisRow {
		measureModeMainDim = widthMeasureMode
		measureModeCrossDim = heightMeasureMode
	}

	paddingAndBorderAxisRow := paddingAndBorderAxisCross
	paddingAndBorderAxisColumn := paddingAndBorderAxisMain
	if isMainAxisRow {
		paddingAndBorderAxisRow = paddingAndBorderAxisMain
		paddingAndBorderAxisColumn = paddingAndBorderAxisCross
	}

	marginAxisRow := nodeMarginForAxis(node, FlexDirectionRow, parentWidth)
	marginAxisColumn := nodeMarginForAxis(node, FlexDirectionColumn, parentWidth)

	// STEP 2: DETERMINE AVAILABLE SIZE IN MAIN AND CROSS DIRECTIONS
	minInnerWidth := resolveValue(&node.Style.MinDimensions[DimensionWidth], parentWidth) - marginAxisRow -
		paddingAndBorderAxisRow
	maxInnerWidth := resolveValue(&node.Style.MaxDimensions[DimensionWidth], parentWidth) - marginAxisRow -
		paddingAndBorderAxisRow
	minInnerHeight := resolveValue(&node.Style.MinDimensions[DimensionHeight], parentHeight) -
		marginAxisColumn - paddingAndBorderAxisColumn
	maxInnerHeight := resolveValue(&node.Style.MaxDimensions[DimensionHeight], parentHeight) -
		marginAxisColumn - paddingAndBorderAxisColumn

	minInnerMainDim := minInnerHeight
	maxInnerMainDim := maxInnerHeight
	if isMainAxisRow {
		minInnerMainDim = minInnerWidth
		maxInnerMainDim = maxInnerWidth
	}

	// Max dimension overrides predefined dimension value; Min dimension in turn overrides both of the
	// above
	availableInnerWidth := availableWidth - marginAxisRow - paddingAndBorderAxisRow
	if !FloatIsUndefined(availableInnerWidth) {
		// We want to make sure our available width does not violate min and max raints
		availableInnerWidth = fmaxf(fminf(availableInnerWidth, maxInnerWidth), minInnerWidth)
	}

	availableInnerHeight := availableHeight - marginAxisColumn - paddingAndBorderAxisColumn
	if !FloatIsUndefined(availableInnerHeight) {
		// We want to make sure our available height does not violate min and max raints
		availableInnerHeight = fmaxf(fminf(availableInnerHeight, maxInnerHeight), minInnerHeight)
	}

	availableInnerMainDim := availableInnerHeight
	availableInnerCrossDim := availableInnerWidth
	if isMainAxisRow {
		availableInnerMainDim = availableInnerWidth
		availableInnerCrossDim = availableInnerHeight
	}

	// If there is only one child with flexGrow + flexShrink it means we can set the
	// computedFlexBasis to 0 instead of measuring and shrinking / flexing the child to exactly
	// match the remaining space
	var singleFlexChild *Node
	if measureModeMainDim == MeasureModeExactly {
		for i := 0; i < childCount; i++ {
			child := node.GetChild(i)
			if singleFlexChild != nil {
				if nodeIsFlex(child) {
					// There is already a flexible child, abort.
					singleFlexChild = nil
					break
				}
			} else if resolveFlexGrow(child) > 0 && nodeResolveFlexShrink(child) > 0 {
				singleFlexChild = child
			}
		}
	}

	var totalOuterFlexBasis float32

	// STEP 3: DETERMINE FLEX BASIS FOR EACH ITEM
	for i := 0; i < childCount; i++ {
		child := node.Children[i]
		if child.Style.Display == DisplayNone {
			zeroOutLayoutRecursivly(child)
			child.hasNewLayout = true
			child.IsDirty = false
			continue
		}
		resolveDimensions(child)
		if performLayout {
			// Set the initial position (relative to the parent).
			childDirection := nodeResolveDirection(child, direction)
			nodeSetPosition(child,
				childDirection,
				availableInnerMainDim,
				availableInnerCrossDim,
				availableInnerWidth)
		}

		// Absolute-positioned children don't participate in flex layout. Add them
		// to a list that we can process later.
		if child.Style.PositionType == PositionTypeAbsolute {
			// Store a private linked list of absolutely positioned children
			// so that we can efficiently traverse them later.
			if firstAbsoluteChild == nil {
				firstAbsoluteChild = child
			}
			if currentAbsoluteChild != nil {
				currentAbsoluteChild.NextChild = child
			}
			currentAbsoluteChild = child
			child.NextChild = nil
		} else {
			if child == singleFlexChild {
				child.Layout.computedFlexBasisGeneration = currentGenerationCount
				child.Layout.computedFlexBasis = 0
			} else {
				nodeComputeFlexBasisForChild(node,
					child,
					availableInnerWidth,
					widthMeasureMode,
					availableInnerHeight,
					availableInnerWidth,
					availableInnerHeight,
					heightMeasureMode,
					direction,
					config)
			}
		}

		totalOuterFlexBasis +=
			child.Layout.computedFlexBasis + nodeMarginForAxis(child, mainAxis, availableInnerWidth)

	}

	flexBasisOverflows := totalOuterFlexBasis > availableInnerMainDim
	if measureModeMainDim == MeasureModeUndefined {
		flexBasisOverflows = false
	}
	if isNodeFlexWrap && flexBasisOverflows && measureModeMainDim == MeasureModeAtMost {
		measureModeMainDim = MeasureModeExactly
	}

	// STEP 4: COLLECT FLEX ITEMS INTO FLEX LINES

	// Indexes of children that represent the first and last items in the line.
	startOfLineIndex := 0
	endOfLineIndex := 0

	// Number of lines.
	lineCount := 0

	// Accumulated cross dimensions of all lines so far.
	var totalLineCrossDim float32

	// Max main dimension of all the lines.
	var maxLineMainDim float32

	for endOfLineIndex < childCount {
		// Number of items on the currently line. May be different than the
		// difference
		// between start and end indicates because we skip over absolute-positioned
		// items.
		itemsOnLine := 0

		// sizeConsumedOnCurrentLine is accumulation of the dimensions and margin
		// of all the children on the current line. This will be used in order to
		// either set the dimensions of the node if none already exist or to compute
		// the remaining space left for the flexible children.
		var sizeConsumedOnCurrentLine float32
		var sizeConsumedOnCurrentLineIncludingMinConstraint float32

		var totalFlexGrowFactors float32
		var totalFlexShrinkScaledFactors float32

		// Maintain a linked list of the child nodes that can shrink and/or grow.
		var firstRelativeChild *Node
		var currentRelativeChild *Node

		// Add items to the current line until it's full or we run out of items.
		for i := startOfLineIndex; i < childCount; i++ {
			child := node.Children[i]
			if child.Style.Display == DisplayNone {
				endOfLineIndex++
				continue
			}
			child.lineIndex = lineCount

			if child.Style.PositionType != PositionTypeAbsolute {
				childMarginMainAxis := nodeMarginForAxis(child, mainAxis, availableInnerWidth)
				flexBasisWithMaxConstraints := fminf(resolveValue(&child.Style.MaxDimensions[dim[mainAxis]], mainAxisParentSize), child.Layout.computedFlexBasis)
				flexBasisWithMinAndMaxConstraints := fmaxf(resolveValue(&child.Style.MinDimensions[dim[mainAxis]], mainAxisParentSize), flexBasisWithMaxConstraints)

				// If this is a multi-line flow and this item pushes us over the
				// available size, we've
				// hit the end of the current line. Break out of the loop and lay out
				// the current line.
				if sizeConsumedOnCurrentLineIncludingMinConstraint+flexBasisWithMinAndMaxConstraints+
					childMarginMainAxis >
					availableInnerMainDim &&
					isNodeFlexWrap && itemsOnLine > 0 {
					break
				}

				sizeConsumedOnCurrentLineIncludingMinConstraint +=
					flexBasisWithMinAndMaxConstraints + childMarginMainAxis
				sizeConsumedOnCurrentLine += flexBasisWithMinAndMaxConstraints + childMarginMainAxis
				itemsOnLine++

				if nodeIsFlex(child) {
					totalFlexGrowFactors += resolveFlexGrow(child)

					// Unlike the grow factor, the shrink factor is scaled relative to the child dimension.
					totalFlexShrinkScaledFactors +=
						-nodeResolveFlexShrink(child) * child.Layout.computedFlexBasis
				}

				// Store a private linked list of children that need to be layed out.
				if firstRelativeChild == nil {
					firstRelativeChild = child
				}
				if currentRelativeChild != nil {
					currentRelativeChild.NextChild = child
				}
				currentRelativeChild = child
				child.NextChild = nil
			}
			endOfLineIndex++
		}

		// The total flex factor needs to be floored to 1.
		if totalFlexGrowFactors > 0 && totalFlexGrowFactors < 1 {
			totalFlexGrowFactors = 1
		}

		// The total flex shrink factor needs to be floored to 1.
		if totalFlexShrinkScaledFactors > 0 && totalFlexShrinkScaledFactors < 1 {
			totalFlexShrinkScaledFactors = 1
		}

		// If we don't need to measure the cross axis, we can skip the entire flex
		// step.
		canSkipFlex := !performLayout && measureModeCrossDim == MeasureModeExactly

		// In order to position the elements in the main axis, we have two
		// controls. The space between the beginning and the first element
		// and the space between each two elements.
		var leadingMainDim float32
		var betweenMainDim float32

		// STEP 5: RESOLVING FLEXIBLE LENGTHS ON MAIN AXIS
		// Calculate the remaining available space that needs to be allocated.
		// If the main dimension size isn't known, it is computed based on
		// the line length, so there's no more space left to distribute.

		// If we don't measure with exact main dimension we want to ensure we don't violate min and max
		if measureModeMainDim != MeasureModeExactly {
			if !FloatIsUndefined(minInnerMainDim) && sizeConsumedOnCurrentLine < minInnerMainDim {
				availableInnerMainDim = minInnerMainDim
			} else if !FloatIsUndefined(maxInnerMainDim) &&
				sizeConsumedOnCurrentLine > maxInnerMainDim {
				availableInnerMainDim = maxInnerMainDim
			} else {
				if !node.Config.UseLegacyStretchBehaviour &&
					(totalFlexGrowFactors == 0 || resolveFlexGrow(node) == 0) {
					// If we don't have any children to flex or we can't flex the node itself,
					// space we've used is all space we need. Root node also should be shrunk to minimum
					availableInnerMainDim = sizeConsumedOnCurrentLine
				}
			}
		}

		var remainingFreeSpace float32
		if !FloatIsUndefined(availableInnerMainDim) {
			remainingFreeSpace = availableInnerMainDim - sizeConsumedOnCurrentLine
		} else if sizeConsumedOnCurrentLine < 0 {
			// availableInnerMainDim is indefinite which means the node is being sized based on its
			// content.
			// sizeConsumedOnCurrentLine is negative which means the node will allocate 0 points for
			// its content. Consequently, remainingFreeSpace is 0 - sizeConsumedOnCurrentLine.
			remainingFreeSpace = -sizeConsumedOnCurrentLine
		}

		originalRemainingFreeSpace := remainingFreeSpace
		var deltaFreeSpace float32

		if !canSkipFlex {
			var childFlexBasis float32
			var flexShrinkScaledFactor float32
			var flexGrowFactor float32
			var baseMainSize float32
			var boundMainSize float32

			// Do two passes over the flex items to figure out how to distribute the
			// remaining space.
			// The first pass finds the items whose min/max raints trigger,
			// freezes them at those
			// sizes, and excludes those sizes from the remaining space. The second
			// pass sets the size
			// of each flexible item. It distributes the remaining space amongst the
			// items whose min/max
			// raints didn't trigger in pass 1. For the other items, it sets
			// their sizes by forcing
			// their min/max raints to trigger again.
			//
			// This two pass approach for resolving min/max raints deviates from
			// the spec. The
			// spec (https://www.w3.org/TR/YG-flexbox-1/#resolve-flexible-lengths)
			// describes a process
			// that needs to be repeated a variable number of times. The algorithm
			// implemented here
			// won't handle all cases but it was simpler to implement and it mitigates
			// performance
			// concerns because we know exactly how many passes it'll do.

			// First pass: detect the flex items whose min/max raints trigger
			var deltaFlexShrinkScaledFactors float32
			var deltaFlexGrowFactors float32
			currentRelativeChild = firstRelativeChild
			for currentRelativeChild != nil {
				childFlexBasis =
					fminf(resolveValue(&currentRelativeChild.Style.MaxDimensions[dim[mainAxis]],
						mainAxisParentSize),
						fmaxf(resolveValue(&currentRelativeChild.Style.MinDimensions[dim[mainAxis]],
							mainAxisParentSize),
							currentRelativeChild.Layout.computedFlexBasis))

				if remainingFreeSpace < 0 {
					flexShrinkScaledFactor = -nodeResolveFlexShrink(currentRelativeChild) * childFlexBasis

					// Is this child able to shrink?
					if flexShrinkScaledFactor != 0 {
						baseMainSize =
							childFlexBasis +
								remainingFreeSpace/totalFlexShrinkScaledFactors*flexShrinkScaledFactor
						boundMainSize = nodeBoundAxis(currentRelativeChild,
							mainAxis,
							baseMainSize,
							availableInnerMainDim,
							availableInnerWidth)
						if baseMainSize != boundMainSize {
							// By excluding this item's size and flex factor from remaining,
							// this item's
							// min/max raints should also trigger in the second pass
							// resulting in the
							// item's size calculation being identical in the first and second
							// passes.
							deltaFreeSpace -= boundMainSize - childFlexBasis
							deltaFlexShrinkScaledFactors -= flexShrinkScaledFactor
						}
					}
				} else if remainingFreeSpace > 0 {
					flexGrowFactor = resolveFlexGrow(currentRelativeChild)

					// Is this child able to grow?
					if flexGrowFactor != 0 {
						baseMainSize =
							childFlexBasis + remainingFreeSpace/totalFlexGrowFactors*flexGrowFactor
						boundMainSize = nodeBoundAxis(currentRelativeChild,
							mainAxis,
							baseMainSize,
							availableInnerMainDim,
							availableInnerWidth)

						if baseMainSize != boundMainSize {
							// By excluding this item's size and flex factor from remaining,
							// this item's
							// min/max raints should also trigger in the second pass
							// resulting in the
							// item's size calculation being identical in the first and second
							// passes.
							deltaFreeSpace -= boundMainSize - childFlexBasis
							deltaFlexGrowFactors -= flexGrowFactor
						}
					}
				}

				currentRelativeChild = currentRelativeChild.NextChild
			}

			totalFlexShrinkScaledFactors += deltaFlexShrinkScaledFactors
			totalFlexGrowFactors += deltaFlexGrowFactors
			remainingFreeSpace += deltaFreeSpace

			// Second pass: resolve the sizes of the flexible items
			deltaFreeSpace = 0
			currentRelativeChild = firstRelativeChild
			for currentRelativeChild != nil {
				childFlexBasis =
					fminf(resolveValue(&currentRelativeChild.Style.MaxDimensions[dim[mainAxis]],
						mainAxisParentSize),
						fmaxf(resolveValue(&currentRelativeChild.Style.MinDimensions[dim[mainAxis]],
							mainAxisParentSize),
							currentRelativeChild.Layout.computedFlexBasis))
				updatedMainSize := childFlexBasis

				if remainingFreeSpace < 0 {
					flexShrinkScaledFactor = -nodeResolveFlexShrink(currentRelativeChild) * childFlexBasis
					// Is this child able to shrink?
					if flexShrinkScaledFactor != 0 {
						var childSize float32

						if totalFlexShrinkScaledFactors == 0 {
							childSize = childFlexBasis + flexShrinkScaledFactor
						} else {
							childSize =
								childFlexBasis +
									(remainingFreeSpace/totalFlexShrinkScaledFactors)*flexShrinkScaledFactor
						}

						updatedMainSize = nodeBoundAxis(currentRelativeChild,
							mainAxis,
							childSize,
							availableInnerMainDim,
							availableInnerWidth)
					}
				} else if remainingFreeSpace > 0 {
					flexGrowFactor = resolveFlexGrow(currentRelativeChild)

					// Is this child able to grow?
					if flexGrowFactor != 0 {
						updatedMainSize =
							nodeBoundAxis(currentRelativeChild,
								mainAxis,
								childFlexBasis+
									remainingFreeSpace/totalFlexGrowFactors*flexGrowFactor,
								availableInnerMainDim,
								availableInnerWidth)
					}
				}

				deltaFreeSpace -= updatedMainSize - childFlexBasis

				marginMain := nodeMarginForAxis(currentRelativeChild, mainAxis, availableInnerWidth)
				marginCross := nodeMarginForAxis(currentRelativeChild, crossAxis, availableInnerWidth)

				var childCrossSize float32
				childMainSize := updatedMainSize + marginMain
				var childCrossMeasureMode MeasureMode
				childMainMeasureMode := MeasureModeExactly

				if !FloatIsUndefined(availableInnerCrossDim) &&
					!nodeIsStyleDimDefined(currentRelativeChild, crossAxis, availableInnerCrossDim) &&
					measureModeCrossDim == MeasureModeExactly &&
					!(isNodeFlexWrap && flexBasisOverflows) &&
					nodeAlignItem(node, currentRelativeChild) == AlignStretch {
					childCrossSize = availableInnerCrossDim
					childCrossMeasureMode = MeasureModeExactly
				} else if !nodeIsStyleDimDefined(currentRelativeChild,
					crossAxis,
					availableInnerCrossDim) {
					childCrossSize = availableInnerCrossDim
					childCrossMeasureMode = MeasureModeAtMost
					if FloatIsUndefined(childCrossSize) {
						childCrossMeasureMode = MeasureModeUndefined
					}
				} else {
					childCrossSize = resolveValue(currentRelativeChild.resolvedDimensions[dim[crossAxis]],
						availableInnerCrossDim) +
						marginCross
					isLoosePercentageMeasurement := currentRelativeChild.resolvedDimensions[dim[crossAxis]].Unit == UnitPercent &&
						measureModeCrossDim != MeasureModeExactly
					childCrossMeasureMode = MeasureModeExactly
					if FloatIsUndefined(childCrossSize) || isLoosePercentageMeasurement {
						childCrossMeasureMode = MeasureModeUndefined
					}
				}

				if !FloatIsUndefined(currentRelativeChild.Style.AspectRatio) {
					v := (childMainSize - marginMain) * currentRelativeChild.Style.AspectRatio
					if isMainAxisRow {
						v = (childMainSize - marginMain) / currentRelativeChild.Style.AspectRatio
					}
					childCrossSize = fmaxf(v, nodePaddingAndBorderForAxis(currentRelativeChild, crossAxis, availableInnerWidth))
					childCrossMeasureMode = MeasureModeExactly

					// Parent size raint should have higher priority than flex
					if nodeIsFlex(currentRelativeChild) {
						childCrossSize = fminf(childCrossSize-marginCross, availableInnerCrossDim)
						childMainSize = marginMain
						if isMainAxisRow {
							childMainSize += childCrossSize * currentRelativeChild.Style.AspectRatio
						} else {
							childMainSize += childCrossSize / currentRelativeChild.Style.AspectRatio
						}
					}

					childCrossSize += marginCross
				}

				constrainMaxSizeForMode(currentRelativeChild,
					mainAxis,
					availableInnerMainDim,
					availableInnerWidth,
					&childMainMeasureMode,
					&childMainSize)
				constrainMaxSizeForMode(currentRelativeChild,
					crossAxis,
					availableInnerCrossDim,
					availableInnerWidth,
					&childCrossMeasureMode,
					&childCrossSize)

				requiresStretchLayout := !nodeIsStyleDimDefined(currentRelativeChild, crossAxis, availableInnerCrossDim) &&
					nodeAlignItem(node, currentRelativeChild) == AlignStretch

				childWidth := childCrossSize
				if isMainAxisRow {
					childWidth = childMainSize
				}
				childHeight := childCrossSize
				if !isMainAxisRow {
					childHeight = childMainSize
				}

				childWidthMeasureMode := childCrossMeasureMode
				if isMainAxisRow {
					childWidthMeasureMode = childMainMeasureMode
				}
				childHeightMeasureMode := childCrossMeasureMode
				if !isMainAxisRow {
					childHeightMeasureMode = childMainMeasureMode
				}

				// Recursively call the layout algorithm for this child with the updated
				// main size.
				layoutNodeInternal(currentRelativeChild,
					childWidth,
					childHeight,
					direction,
					childWidthMeasureMode,
					childHeightMeasureMode,
					availableInnerWidth,
					availableInnerHeight,
					performLayout && !requiresStretchLayout,
					"flex",
					config)
				if currentRelativeChild.Layout.HadOverflow {
					node.Layout.HadOverflow = true
				}

				currentRelativeChild = currentRelativeChild.NextChild
			}
		}

		remainingFreeSpace = originalRemainingFreeSpace + deltaFreeSpace
		if remainingFreeSpace < 0 {
			node.Layout.HadOverflow = true
		}

		// STEP 6: MAIN-AXIS JUSTIFICATION & CROSS-AXIS SIZE DETERMINATION

		// At this point, all the children have their dimensions set in the main
		// axis.
		// Their dimensions are also set in the cross axis with the exception of
		// items
		// that are aligned "stretch". We need to compute these stretch values and
		// set the final positions.

		// If we are using "at most" rules in the main axis. Calculate the remaining space when
		// raint by the min size defined for the main axis.

		if measureModeMainDim == MeasureModeAtMost && remainingFreeSpace > 0 {
			if node.Style.MinDimensions[dim[mainAxis]].Unit != UnitUndefined &&
				resolveValue(&node.Style.MinDimensions[dim[mainAxis]], mainAxisParentSize) >= 0 {
				remainingFreeSpace =
					fmaxf(0,
						resolveValue(&node.Style.MinDimensions[dim[mainAxis]], mainAxisParentSize)-
							(availableInnerMainDim-remainingFreeSpace))
			} else {
				remainingFreeSpace = 0
			}
		}

		numberOfAutoMarginsOnCurrentLine := 0
		for i := startOfLineIndex; i < endOfLineIndex; i++ {
			child := node.Children[i]
			if child.Style.PositionType == PositionTypeRelative {
				if marginLeadingValue(child, mainAxis).Unit == UnitAuto {
					numberOfAutoMarginsOnCurrentLine++
				}
				if marginTrailingValue(child, mainAxis).Unit == UnitAuto {
					numberOfAutoMarginsOnCurrentLine++
				}
			}
		}

		if numberOfAutoMarginsOnCurrentLine == 0 {
			switch justifyContent {
			case JustifyCenter:
				leadingMainDim = remainingFreeSpace / 2
			case JustifyFlexEnd:
				leadingMainDim = remainingFreeSpace
			case JustifySpaceBetween:
				if itemsOnLine > 1 {
					betweenMainDim = fmaxf(remainingFreeSpace, 0) / float32(itemsOnLine-1)
				} else {
					betweenMainDim = 0
				}
			case JustifySpaceAround:
				// Space on the edges is half of the space between elements
				betweenMainDim = remainingFreeSpace / float32(itemsOnLine)
				leadingMainDim = betweenMainDim / 2
			case JustifyFlexStart:
			}
		}

		mainDim := leadingPaddingAndBorderMain + leadingMainDim
		var crossDim float32

		for i := startOfLineIndex; i < endOfLineIndex; i++ {
			child := node.Children[i]
			if child.Style.Display == DisplayNone {
				continue
			}
			if child.Style.PositionType == PositionTypeAbsolute &&
				nodeIsLeadingPosDefined(child, mainAxis) {
				if performLayout {
					// In case the child is position absolute and has left/top being
					// defined, we override the position to whatever the user said
					// (and margin/border).
					child.Layout.Position[pos[mainAxis]] =
						nodeLeadingPosition(child, mainAxis, availableInnerMainDim) +
							nodeLeadingBorder(node, mainAxis) +
							nodeLeadingMargin(child, mainAxis, availableInnerWidth)
				}
			} else {
				// Now that we placed the element, we need to update the variables.
				// We need to do that only for relative elements. Absolute elements
				// do not take part in that phase.
				if child.Style.PositionType == PositionTypeRelative {
					if marginLeadingValue(child, mainAxis).Unit == UnitAuto {
						mainDim += remainingFreeSpace / float32(numberOfAutoMarginsOnCurrentLine)
					}

					if performLayout {
						child.Layout.Position[pos[mainAxis]] += mainDim
					}

					if marginTrailingValue(child, mainAxis).Unit == UnitAuto {
						mainDim += remainingFreeSpace / float32(numberOfAutoMarginsOnCurrentLine)
					}

					if canSkipFlex {
						// If we skipped the flex step, then we can't rely on the
						// measuredDims because
						// they weren't computed. This means we can't call YGNodeDimWithMargin.
						mainDim += betweenMainDim + nodeMarginForAxis(child, mainAxis, availableInnerWidth) +
							child.Layout.computedFlexBasis
						crossDim = availableInnerCrossDim
					} else {
						// The main dimension is the sum of all the elements dimension plus the spacing.
						mainDim += betweenMainDim + nodeDimWithMargin(child, mainAxis, availableInnerWidth)

						// The cross dimension is the max of the elements dimension since
						// there can only be one element in that cross dimension.
						crossDim = fmaxf(crossDim, nodeDimWithMargin(child, crossAxis, availableInnerWidth))
					}
				} else if performLayout {
					child.Layout.Position[pos[mainAxis]] +=
						nodeLeadingBorder(node, mainAxis) + leadingMainDim
				}
			}
		}

		mainDim += trailingPaddingAndBorderMain

		containerCrossAxis := availableInnerCrossDim
		if measureModeCrossDim == MeasureModeUndefined ||
			measureModeCrossDim == MeasureModeAtMost {
			// Compute the cross axis from the max cross dimension of the children.
			containerCrossAxis = nodeBoundAxis(node,
				crossAxis,
				crossDim+paddingAndBorderAxisCross,
				crossAxisParentSize,
				parentWidth) -
				paddingAndBorderAxisCross
		}

		// If there's no flex wrap, the cross dimension is defined by the container.
		if !isNodeFlexWrap && measureModeCrossDim == MeasureModeExactly {
			crossDim = availableInnerCrossDim
		}

		// Clamp to the min/max size specified on the container.
		crossDim = nodeBoundAxis(node,
			crossAxis,
			crossDim+paddingAndBorderAxisCross,
			crossAxisParentSize,
			parentWidth) -
			paddingAndBorderAxisCross

		// STEP 7: CROSS-AXIS ALIGNMENT
		// We can skip child alignment if we're just measuring the container.
		if performLayout {
			for i := startOfLineIndex; i < endOfLineIndex; i++ {
				child := node.Children[i]
				if child.Style.Display == DisplayNone {
					continue
				}
				if child.Style.PositionType == PositionTypeAbsolute {
					// If the child is absolutely positioned and has a
					// top/left/bottom/right
					// set, override all the previously computed positions to set it
					// correctly.
					if nodeIsLeadingPosDefined(child, crossAxis) {
						child.Layout.Position[pos[crossAxis]] =
							nodeLeadingPosition(child, crossAxis, availableInnerCrossDim) +
								nodeLeadingBorder(node, crossAxis) +
								nodeLeadingMargin(child, crossAxis, availableInnerWidth)
					} else {
						child.Layout.Position[pos[crossAxis]] =
							nodeLeadingBorder(node, crossAxis) +
								nodeLeadingMargin(child, crossAxis, availableInnerWidth)
					}
				} else {
					leadingCrossDim := leadingPaddingAndBorderCross

					// For a relative children, we're either using alignItems (parent) or
					// alignSelf (child) in order to determine the position in the cross
					// axis
					alignItem := nodeAlignItem(node, child)

					// If the child uses align stretch, we need to lay it out one more
					// time, this time
					// forcing the cross-axis size to be the computed cross size for the
					// current line.
					if alignItem == AlignStretch &&
						marginLeadingValue(child, crossAxis).Unit != UnitAuto &&
						marginTrailingValue(child, crossAxis).Unit != UnitAuto {
						// If the child defines a definite size for its cross axis, there's
						// no need to stretch.
						if !nodeIsStyleDimDefined(child, crossAxis, availableInnerCrossDim) {
							childMainSize := child.Layout.measuredDimensions[dim[mainAxis]]
							childCrossSize := crossDim
							if !FloatIsUndefined(child.Style.AspectRatio) {
								childCrossSize = nodeMarginForAxis(child, crossAxis, availableInnerWidth)
								if isMainAxisRow {
									childCrossSize += childMainSize / child.Style.AspectRatio
								} else {
									childCrossSize += childMainSize * child.Style.AspectRatio
								}
							}

							childMainSize += nodeMarginForAxis(child, mainAxis, availableInnerWidth)

							childMainMeasureMode := MeasureModeExactly
							childCrossMeasureMode := MeasureModeExactly
							constrainMaxSizeForMode(child,
								mainAxis,
								availableInnerMainDim,
								availableInnerWidth,
								&childMainMeasureMode,
								&childMainSize)
							constrainMaxSizeForMode(child,
								crossAxis,
								availableInnerCrossDim,
								availableInnerWidth,
								&childCrossMeasureMode,
								&childCrossSize)

							childWidth := childCrossSize
							if isMainAxisRow {
								childWidth = childMainSize
							}
							childHeight := childCrossSize
							if !isMainAxisRow {
								childHeight = childMainSize
							}

							childWidthMeasureMode := MeasureModeExactly
							if FloatIsUndefined(childWidth) {
								childWidthMeasureMode = MeasureModeUndefined
							}

							childHeightMeasureMode := MeasureModeExactly
							if FloatIsUndefined(childHeight) {
								childHeightMeasureMode = MeasureModeUndefined
							}

							layoutNodeInternal(child,
								childWidth,
								childHeight,
								direction,
								childWidthMeasureMode,
								childHeightMeasureMode,
								availableInnerWidth,
								availableInnerHeight,
								true,
								"stretch",
								config)
						}
					} else {
						remainingCrossDim := containerCrossAxis - nodeDimWithMargin(child, crossAxis, availableInnerWidth)

						if marginLeadingValue(child, crossAxis).Unit == UnitAuto &&
							marginTrailingValue(child, crossAxis).Unit == UnitAuto {
							leadingCrossDim += fmaxf(0, remainingCrossDim/2)
						} else if marginTrailingValue(child, crossAxis).Unit == UnitAuto {
							// No-Op
						} else if marginLeadingValue(child, crossAxis).Unit == UnitAuto {
							leadingCrossDim += fmaxf(0, remainingCrossDim)
						} else if alignItem == AlignFlexStart {
							// No-Op
						} else if alignItem == AlignCenter {
							leadingCrossDim += remainingCrossDim / 2
						} else {
							leadingCrossDim += remainingCrossDim
						}
					}
					// And we apply the position
					child.Layout.Position[pos[crossAxis]] += totalLineCrossDim + leadingCrossDim
				}
			}
		}

		totalLineCrossDim += crossDim
		maxLineMainDim = fmaxf(maxLineMainDim, mainDim)

		lineCount++
		startOfLineIndex = endOfLineIndex

	}

	// STEP 8: MULTI-LINE CONTENT ALIGNMENT
	if performLayout && (lineCount > 1 || isBaselineLayout(node)) &&
		!FloatIsUndefined(availableInnerCrossDim) {
		remainingAlignContentDim := availableInnerCrossDim - totalLineCrossDim

		var crossDimLead float32
		currentLead := leadingPaddingAndBorderCross

		switch node.Style.AlignContent {
		case AlignFlexEnd:
			currentLead += remainingAlignContentDim
		case AlignCenter:
			currentLead += remainingAlignContentDim / 2
		case AlignStretch:
			if availableInnerCrossDim > totalLineCrossDim {
				crossDimLead = remainingAlignContentDim / float32(lineCount)
			}
		case AlignSpaceAround:
			if availableInnerCrossDim > totalLineCrossDim {
				currentLead += remainingAlignContentDim / float32(2*lineCount)
				if lineCount > 1 {
					crossDimLead = remainingAlignContentDim / float32(lineCount)
				}
			} else {
				currentLead += remainingAlignContentDim / 2
			}
		case AlignSpaceBetween:
			if availableInnerCrossDim > totalLineCrossDim && lineCount > 1 {
				crossDimLead = remainingAlignContentDim / float32(lineCount-1)
			}
		case AlignAuto:
		case AlignFlexStart:
		case AlignBaseline:
		}

		endIndex := 0
		for i := 0; i < lineCount; i++ {
			startIndex := endIndex
			var ii int

			// compute the line's height and find the endIndex
			var lineHeight float32
			var maxAscentForCurrentLine float32
			var maxDescentForCurrentLine float32
			for ii = startIndex; ii < childCount; ii++ {
				child := node.Children[ii]
				if child.Style.Display == DisplayNone {
					continue
				}
				if child.Style.PositionType == PositionTypeRelative {
					if child.lineIndex != i {
						break
					}
					if nodeIsLayoutDimDefined(child, crossAxis) {
						lineHeight = fmaxf(lineHeight,
							child.Layout.measuredDimensions[dim[crossAxis]]+
								nodeMarginForAxis(child, crossAxis, availableInnerWidth))
					}
					if nodeAlignItem(node, child) == AlignBaseline {
						ascent := Baseline(child) + nodeLeadingMargin(child, FlexDirectionColumn, availableInnerWidth)
						descent := child.Layout.measuredDimensions[DimensionHeight] + nodeMarginForAxis(child, FlexDirectionColumn, availableInnerWidth) - ascent
						maxAscentForCurrentLine = fmaxf(maxAscentForCurrentLine, ascent)
						maxDescentForCurrentLine = fmaxf(maxDescentForCurrentLine, descent)
						lineHeight = fmaxf(lineHeight, maxAscentForCurrentLine+maxDescentForCurrentLine)
					}
				}
			}
			endIndex = ii
			lineHeight += crossDimLead

			if performLayout {
				for ii = startIndex; ii < endIndex; ii++ {
					child := node.Children[ii]
					if child.Style.Display == DisplayNone {
						continue
					}
					if child.Style.PositionType == PositionTypeRelative {
						switch nodeAlignItem(node, child) {
						case AlignFlexStart:
							{
								child.Layout.Position[pos[crossAxis]] =
									currentLead + nodeLeadingMargin(child, crossAxis, availableInnerWidth)
							}
						case AlignFlexEnd:
							{
								child.Layout.Position[pos[crossAxis]] =
									currentLead + lineHeight -
										nodeTrailingMargin(child, crossAxis, availableInnerWidth) -
										child.Layout.measuredDimensions[dim[crossAxis]]
							}
						case AlignCenter:
							{
								childHeight := child.Layout.measuredDimensions[dim[crossAxis]]
								child.Layout.Position[pos[crossAxis]] = currentLead + (lineHeight-childHeight)/2
							}
						case AlignStretch:
							{
								child.Layout.Position[pos[crossAxis]] =
									currentLead + nodeLeadingMargin(child, crossAxis, availableInnerWidth)

								// Remeasure child with the line height as it as been only measured with the
								// parents height yet.
								if !nodeIsStyleDimDefined(child, crossAxis, availableInnerCrossDim) {
									childWidth := lineHeight
									if isMainAxisRow {
										childWidth = child.Layout.measuredDimensions[DimensionWidth] +
											nodeMarginForAxis(child, mainAxis, availableInnerWidth)
									}

									childHeight := lineHeight
									if !isMainAxisRow {
										childHeight = child.Layout.measuredDimensions[DimensionHeight] +
											nodeMarginForAxis(child, crossAxis, availableInnerWidth)
									}

									if !(FloatsEqual(childWidth,
										child.Layout.measuredDimensions[DimensionWidth]) &&
										FloatsEqual(childHeight,
											child.Layout.measuredDimensions[DimensionHeight])) {
										layoutNodeInternal(child,
											childWidth,
											childHeight,
											direction,
											MeasureModeExactly,
											MeasureModeExactly,
											availableInnerWidth,
											availableInnerHeight,
											true,
											"multiline-stretch",
											config)
									}
								}
							}
						case AlignBaseline:
							{
								child.Layout.Position[EdgeTop] =
									currentLead + maxAscentForCurrentLine - Baseline(child) +
										nodeLeadingPosition(child, FlexDirectionColumn, availableInnerCrossDim)
							}
						case AlignAuto:
						case AlignSpaceBetween:
						case AlignSpaceAround:
						}
					}
				}
			}

			currentLead += lineHeight
		}
	}

	// STEP 9: COMPUTING FINAL DIMENSIONS
	node.Layout.measuredDimensions[DimensionWidth] = nodeBoundAxis(
		node, FlexDirectionRow, availableWidth-marginAxisRow, parentWidth, parentWidth)
	node.Layout.measuredDimensions[DimensionHeight] = nodeBoundAxis(
		node, FlexDirectionColumn, availableHeight-marginAxisColumn, parentHeight, parentWidth)

	// If the user didn't specify a width or height for the node, set the
	// dimensions based on the children.
	if measureModeMainDim == MeasureModeUndefined ||
		(node.Style.Overflow != OverflowScroll && measureModeMainDim == MeasureModeAtMost) {
		// Clamp the size to the min/max size, if specified, and make sure it
		// doesn't go below the padding and border amount.
		node.Layout.measuredDimensions[dim[mainAxis]] =
			nodeBoundAxis(node, mainAxis, maxLineMainDim, mainAxisParentSize, parentWidth)
	} else if measureModeMainDim == MeasureModeAtMost &&
		node.Style.Overflow == OverflowScroll {
		node.Layout.measuredDimensions[dim[mainAxis]] = fmaxf(
			fminf(availableInnerMainDim+paddingAndBorderAxisMain,
				nodeBoundAxisWithinMinAndMax(node, mainAxis, maxLineMainDim, mainAxisParentSize)),
			paddingAndBorderAxisMain)
	}

	if measureModeCrossDim == MeasureModeUndefined ||
		(node.Style.Overflow != OverflowScroll && measureModeCrossDim == MeasureModeAtMost) {
		// Clamp the size to the min/max size, if specified, and make sure it
		// doesn't go below the padding and border amount.
		node.Layout.measuredDimensions[dim[crossAxis]] =
			nodeBoundAxis(node,
				crossAxis,
				totalLineCrossDim+paddingAndBorderAxisCross,
				crossAxisParentSize,
				parentWidth)
	} else if measureModeCrossDim == MeasureModeAtMost &&
		node.Style.Overflow == OverflowScroll {
		node.Layout.measuredDimensions[dim[crossAxis]] =
			fmaxf(fminf(availableInnerCrossDim+paddingAndBorderAxisCross,
				nodeBoundAxisWithinMinAndMax(node,
					crossAxis,
					totalLineCrossDim+paddingAndBorderAxisCross,
					crossAxisParentSize)),
				paddingAndBorderAxisCross)
	}

	// As we only wrapped in normal direction yet, we need to reverse the positions on wrap-reverse.
	if performLayout && node.Style.FlexWrap == WrapWrapReverse {
		for i := 0; i < childCount; i++ {
			child := node.GetChild(i)
			if child.Style.PositionType == PositionTypeRelative {
				child.Layout.Position[pos[crossAxis]] = node.Layout.measuredDimensions[dim[crossAxis]] -
					child.Layout.Position[pos[crossAxis]] -
					child.Layout.measuredDimensions[dim[crossAxis]]
			}
		}
	}

	if performLayout {
		// STEP 10: SIZING AND POSITIONING ABSOLUTE CHILDREN
		for currentAbsoluteChild = firstAbsoluteChild; currentAbsoluteChild != nil; currentAbsoluteChild = currentAbsoluteChild.NextChild {
			mode := measureModeCrossDim
			if isMainAxisRow {
				mode = measureModeMainDim
			}

			nodeAbsoluteLayoutChild(node,
				currentAbsoluteChild,
				availableInnerWidth,
				mode,
				availableInnerHeight,
				direction,
				config)
		}

		// STEP 11: SETTING TRAILING POSITIONS FOR CHILDREN
		needsMainTrailingPos := mainAxis == FlexDirectionRowReverse || mainAxis == FlexDirectionColumnReverse
		needsCrossTrailingPos := crossAxis == FlexDirectionRowReverse || crossAxis == FlexDirectionColumnReverse

		// Set trailing position if necessary.
		if needsMainTrailingPos || needsCrossTrailingPos {
			for i := 0; i < childCount; i++ {
				child := node.Children[i]
				if child.Style.Display == DisplayNone {
					continue
				}
				if needsMainTrailingPos {
					nodeSetChildTrailingPosition(node, child, mainAxis)
				}

				if needsCrossTrailingPos {
					nodeSetChildTrailingPosition(node, child, crossAxis)
				}
			}
		}
	}
}

var (
	gDepth        = 0
	gPrintTree    = false
	gPrintChanges = false
	gPrintSkips   = false
)

const (
	spacerStr = "                                                            "
)

// spacer returns spacer string
func spacer(level int) string {
	n := len(spacerStr)
	if level > n {
		level = n
	}
	return spacerStr[:level]
}

var (
	measureModeNames = [measureModeCount]string{"UNDEFINED", "EXACTLY", "AT_MOST"}
	layoutModeNames  = [measureModeCount]string{"LAY_UNDEFINED", "LAY_EXACTLY", "LAY_AT_MOST"}
)

// measureModeName returns name of measure mode
func measureModeName(mode MeasureMode, performLayout bool) string {

	if mode >= measureModeCount {
		return ""
	}

	if performLayout {
		return layoutModeNames[mode]
	}
	return measureModeNames[mode]
}

func measureModeSizeIsExactAndMatchesOldMeasuredSize(sizeMode MeasureMode, size float32, lastComputedSize float32) bool {
	return sizeMode == MeasureModeExactly && FloatsEqual(size, lastComputedSize)
}

func measureModeOldSizeIsUnspecifiedAndStillFits(sizeMode MeasureMode, size float32, lastSizeMode MeasureMode, lastComputedSize float32) bool {
	return sizeMode == MeasureModeAtMost && lastSizeMode == MeasureModeUndefined &&
		(size >= lastComputedSize || FloatsEqual(size, lastComputedSize))
}

func measureModeNewMeasureSizeIsStricterAndStillValid(sizeMode MeasureMode, size float32, lastSizeMode MeasureMode, lastSize float32, lastComputedSize float32) bool {
	return lastSizeMode == MeasureModeAtMost && sizeMode == MeasureModeAtMost &&
		lastSize > size && (lastComputedSize <= size || FloatsEqual(size, lastComputedSize))
}

// roundValueToPixelGrid rounds value to pixel grid
func roundValueToPixelGrid(value float32, pointScaleFactor float32, forceCeil bool, forceFloor bool) float32 {
	scaledValue := value * pointScaleFactor
	fractial := fmodf(scaledValue, 1.0)
	if FloatsEqual(fractial, 0) {
		// First we check if the value is already rounded
		scaledValue = scaledValue - fractial
	} else if FloatsEqual(fractial, 1.0) {
		scaledValue = scaledValue - fractial + 1.0
	} else if forceCeil {
		// Next we check if we need to use forced rounding
		scaledValue = scaledValue - fractial + 1.0
	} else if forceFloor {
		scaledValue = scaledValue - fractial
	} else {
		// Finally we just round the value
		var f float32
		if fractial >= 0.5 {
			f = 1.0
		}
		scaledValue = scaledValue - fractial + f
	}
	return scaledValue / pointScaleFactor
}

// nodeCanUseCachedMeasurement returns true if can use cached measurement
func nodeCanUseCachedMeasurement(widthMode MeasureMode, width float32, heightMode MeasureMode, height float32, lastWidthMode MeasureMode, lastWidth float32, lastHeightMode MeasureMode, lastHeight float32, lastComputedWidth float32, lastComputedHeight float32, marginRow float32, marginColumn float32, config *Config) bool {
	if lastComputedHeight < 0 || lastComputedWidth < 0 {
		return false
	}
	useRoundedComparison := config != nil && config.PointScaleFactor != 0
	effectiveWidth := width
	effectiveHeight := height
	effectiveLastWidth := lastWidth
	effectiveLastHeight := lastHeight

	if useRoundedComparison {
		effectiveWidth = roundValueToPixelGrid(width, config.PointScaleFactor, false, false)
		effectiveHeight = roundValueToPixelGrid(height, config.PointScaleFactor, false, false)
		effectiveLastWidth = roundValueToPixelGrid(lastWidth, config.PointScaleFactor, false, false)
		effectiveLastHeight = roundValueToPixelGrid(lastHeight, config.PointScaleFactor, false, false)
	}

	hasSameWidthSpec := lastWidthMode == widthMode && FloatsEqual(effectiveLastWidth, effectiveWidth)
	hasSameHeightSpec := lastHeightMode == heightMode && FloatsEqual(effectiveLastHeight, effectiveHeight)

	widthIsCompatible :=
		hasSameWidthSpec || measureModeSizeIsExactAndMatchesOldMeasuredSize(widthMode,
			width-marginRow,
			lastComputedWidth) ||
			measureModeOldSizeIsUnspecifiedAndStillFits(widthMode,
				width-marginRow,
				lastWidthMode,
				lastComputedWidth) ||
			measureModeNewMeasureSizeIsStricterAndStillValid(
				widthMode, width-marginRow, lastWidthMode, lastWidth, lastComputedWidth)

	heightIsCompatible :=
		hasSameHeightSpec || measureModeSizeIsExactAndMatchesOldMeasuredSize(heightMode,
			height-marginColumn,
			lastComputedHeight) ||
			measureModeOldSizeIsUnspecifiedAndStillFits(heightMode,
				height-marginColumn,
				lastHeightMode,
				lastComputedHeight) ||
			measureModeNewMeasureSizeIsStricterAndStillValid(
				heightMode, height-marginColumn, lastHeightMode, lastHeight, lastComputedHeight)

	return widthIsCompatible && heightIsCompatible
}

// layoutNodeInternal is a wrapper around the YGNodelayoutImpl function. It determines
// whether the layout request is redundant and can be skipped.
//
// Parameters:
//  Input parameters are the same as YGNodelayoutImpl (see above)
//  Return parameter is true if layout was performed, false if skipped
func layoutNodeInternal(node *Node, availableWidth float32, availableHeight float32,
	parentDirection Direction, widthMeasureMode MeasureMode,
	heightMeasureMode MeasureMode, parentWidth float32, parentHeight float32,
	performLayout bool, reason string, config *Config) bool {
	layout := &node.Layout

	gDepth++

	needToVisitNode :=
		(node.IsDirty && layout.generationCount != currentGenerationCount) ||
			layout.lastParentDirection != parentDirection

	if needToVisitNode {
		// Invalidate the cached results.
		layout.nextCachedMeasurementsIndex = 0
		layout.cachedLayout.widthMeasureMode = MeasureMode(-1)
		layout.cachedLayout.heightMeasureMode = MeasureMode(-1)
		layout.cachedLayout.computedWidth = -1
		layout.cachedLayout.computedHeight = -1
	}

	var cachedResults *CachedMeasurement

	// Determine whether the results are already cached. We maintain a separate
	// cache for layouts and measurements. A layout operation modifies the
	// positions
	// and dimensions for nodes in the subtree. The algorithm assumes that each
	// node
	// gets layed out a maximum of one time per tree layout, but multiple
	// measurements
	// may be required to resolve all of the flex dimensions.
	// We handle nodes with measure functions specially here because they are the
	// most
	// expensive to measure, so it's worth avoiding redundant measurements if at
	// all possible.
	if node.Measure != nil {
		marginAxisRow := nodeMarginForAxis(node, FlexDirectionRow, parentWidth)
		marginAxisColumn := nodeMarginForAxis(node, FlexDirectionColumn, parentWidth)

		// First, try to use the layout cache.
		if nodeCanUseCachedMeasurement(widthMeasureMode,
			availableWidth,
			heightMeasureMode,
			availableHeight,
			layout.cachedLayout.widthMeasureMode,
			layout.cachedLayout.availableWidth,
			layout.cachedLayout.heightMeasureMode,
			layout.cachedLayout.availableHeight,
			layout.cachedLayout.computedWidth,
			layout.cachedLayout.computedHeight,
			marginAxisRow,
			marginAxisColumn,
			config) {
			cachedResults = &layout.cachedLayout
		} else {
			// Try to use the measurement cache.
			for i := 0; i < layout.nextCachedMeasurementsIndex; i++ {
				if nodeCanUseCachedMeasurement(widthMeasureMode,
					availableWidth,
					heightMeasureMode,
					availableHeight,
					layout.cachedMeasurements[i].widthMeasureMode,
					layout.cachedMeasurements[i].availableWidth,
					layout.cachedMeasurements[i].heightMeasureMode,
					layout.cachedMeasurements[i].availableHeight,
					layout.cachedMeasurements[i].computedWidth,
					layout.cachedMeasurements[i].computedHeight,
					marginAxisRow,
					marginAxisColumn,
					config) {
					cachedResults = &layout.cachedMeasurements[i]
					break
				}
			}
		}
	} else if performLayout {
		if FloatsEqual(layout.cachedLayout.availableWidth, availableWidth) &&
			FloatsEqual(layout.cachedLayout.availableHeight, availableHeight) &&
			layout.cachedLayout.widthMeasureMode == widthMeasureMode &&
			layout.cachedLayout.heightMeasureMode == heightMeasureMode {
			cachedResults = &layout.cachedLayout
		}
	} else {
		for i := 0; i < layout.nextCachedMeasurementsIndex; i++ {
			if FloatsEqual(layout.cachedMeasurements[i].availableWidth, availableWidth) &&
				FloatsEqual(layout.cachedMeasurements[i].availableHeight, availableHeight) &&
				layout.cachedMeasurements[i].widthMeasureMode == widthMeasureMode &&
				layout.cachedMeasurements[i].heightMeasureMode == heightMeasureMode {
				cachedResults = &layout.cachedMeasurements[i]
				break
			}
		}
	}

	if !needToVisitNode && cachedResults != nil {
		layout.measuredDimensions[DimensionWidth] = cachedResults.computedWidth
		layout.measuredDimensions[DimensionHeight] = cachedResults.computedHeight

		if gPrintChanges && gPrintSkips {
			fmt.Printf("%s%d.{[skipped] ", spacer(gDepth), gDepth)
			if node.Print != nil {
				node.Print(node)
			}
			fmt.Printf("wm: %s, hm: %s, aw: %f ah: %f => d: (%f, %f) %s\n",
				measureModeName(widthMeasureMode, performLayout),
				measureModeName(heightMeasureMode, performLayout),
				availableWidth,
				availableHeight,
				cachedResults.computedWidth,
				cachedResults.computedHeight,
				reason)
		}
	} else {
		if gPrintChanges {
			s := ""
			if needToVisitNode {
				s = "*"
			}
			fmt.Printf("%s%d.{%s", spacer(gDepth), gDepth, s)
			if node.Print != nil {
				node.Print(node)
			}
			fmt.Printf("wm: %s, hm: %s, aw: %f ah: %f %s\n",
				measureModeName(widthMeasureMode, performLayout),
				measureModeName(heightMeasureMode, performLayout),
				availableWidth,
				availableHeight,
				reason)
		}

		nodelayoutImpl(node,
			availableWidth,
			availableHeight,
			parentDirection,
			widthMeasureMode,
			heightMeasureMode,
			parentWidth,
			parentHeight,
			performLayout,
			config)

		if gPrintChanges {
			s := ""
			if needToVisitNode {
				s = "*"
			}
			fmt.Printf("%s%d.}%s", spacer(gDepth), gDepth, s)
			if node.Print != nil {
				node.Print(node)
			}
			fmt.Printf("wm: %s, hm: %s, d: (%f, %f) %s\n",
				measureModeName(widthMeasureMode, performLayout),
				measureModeName(heightMeasureMode, performLayout),
				layout.measuredDimensions[DimensionWidth],
				layout.measuredDimensions[DimensionHeight],
				reason)
		}

		layout.lastParentDirection = parentDirection

		if cachedResults == nil {
			if layout.nextCachedMeasurementsIndex == maxCachedResultCount {
				if gPrintChanges {
					fmt.Printf("Out of cache entries!\n")
				}
				layout.nextCachedMeasurementsIndex = 0
			}

			var newCacheEntry *CachedMeasurement
			if performLayout {
				// Use the single layout cache entry.
				newCacheEntry = &layout.cachedLayout
			} else {
				// Allocate a new measurement cache entry.
				newCacheEntry = &layout.cachedMeasurements[layout.nextCachedMeasurementsIndex]
				layout.nextCachedMeasurementsIndex++
			}

			newCacheEntry.availableWidth = availableWidth
			newCacheEntry.availableHeight = availableHeight
			newCacheEntry.widthMeasureMode = widthMeasureMode
			newCacheEntry.heightMeasureMode = heightMeasureMode
			newCacheEntry.computedWidth = layout.measuredDimensions[DimensionWidth]
			newCacheEntry.computedHeight = layout.measuredDimensions[DimensionHeight]
		}
	}

	if performLayout {
		node.Layout.Dimensions[DimensionWidth] = node.Layout.measuredDimensions[DimensionWidth]
		node.Layout.Dimensions[DimensionHeight] = node.Layout.measuredDimensions[DimensionHeight]
		node.hasNewLayout = true
		node.IsDirty = false
	}

	gDepth--
	layout.generationCount = currentGenerationCount
	return needToVisitNode || cachedResults == nil
}

// SetPointScaleFactor sets scale factor
func (config *Config) SetPointScaleFactor(pixelsInPoint float32) {
	assertWithConfig(config, pixelsInPoint >= 0, "Scale factor should not be less than zero")

	// We store points for Pixel as we will use it for rounding
	if pixelsInPoint == 0 {
		// Zero is used to skip rounding
		config.PointScaleFactor = 0
	} else {
		config.PointScaleFactor = pixelsInPoint
	}
}

func roundToPixelGrid(node *Node, pointScaleFactor float32, absoluteLeft float32, absoluteTop float32) {
	if pointScaleFactor == 0.0 {
		return
	}

	nodeLeft := node.Layout.Position[EdgeLeft]
	nodeTop := node.Layout.Position[EdgeTop]

	nodeWidth := node.Layout.Dimensions[DimensionWidth]
	nodeHeight := node.Layout.Dimensions[DimensionHeight]

	absoluteNodeLeft := absoluteLeft + nodeLeft
	absoluteNodeTop := absoluteTop + nodeTop

	absoluteNodeRight := absoluteNodeLeft + nodeWidth
	absoluteNodeBottom := absoluteNodeTop + nodeHeight

	// If a node has a custom measure function we never want to round down its size as this could
	// lead to unwanted text truncation.
	textRounding := node.NodeType == NodeTypeText

	node.Layout.Position[EdgeLeft] = roundValueToPixelGrid(nodeLeft, pointScaleFactor, false, textRounding)
	node.Layout.Position[EdgeTop] = roundValueToPixelGrid(nodeTop, pointScaleFactor, false, textRounding)

	// We multiply dimension by scale factor and if the result is close to the whole number, we don't have any fraction
	// To verify if the result is close to whole number we want to check both floor and ceil numbers
	hasFractionalWidth := !FloatsEqual(fmodf(nodeWidth*pointScaleFactor, 1), 0) &&
		!FloatsEqual(fmodf(nodeWidth*pointScaleFactor, 1), 1)
	hasFractionalHeight := !FloatsEqual(fmodf(nodeHeight*pointScaleFactor, 1), 0) &&
		!FloatsEqual(fmodf(nodeHeight*pointScaleFactor, 1), 1)

	node.Layout.Dimensions[DimensionWidth] =
		roundValueToPixelGrid(
			absoluteNodeRight,
			pointScaleFactor,
			(textRounding && hasFractionalWidth),
			(textRounding && !hasFractionalWidth)) -
			roundValueToPixelGrid(absoluteNodeLeft, pointScaleFactor, false, textRounding)
	node.Layout.Dimensions[DimensionHeight] =
		roundValueToPixelGrid(
			absoluteNodeBottom,
			pointScaleFactor,
			(textRounding && hasFractionalHeight),
			(textRounding && !hasFractionalHeight)) -
			roundValueToPixelGrid(absoluteNodeTop, pointScaleFactor, false, textRounding)

	for _, child := range node.Children {
		roundToPixelGrid(child, pointScaleFactor, absoluteNodeLeft, absoluteNodeTop)
	}
}

func calcStartWidth(node *Node, parentWidth float32) (float32, MeasureMode) {
	if nodeIsStyleDimDefined(node, FlexDirectionRow, parentWidth) {
		width := resolveValue(node.resolvedDimensions[dim[FlexDirectionRow]], parentWidth)
		margin := nodeMarginForAxis(node, FlexDirectionRow, parentWidth)
		return width + margin, MeasureModeExactly
	}
	if resolveValue(&node.Style.MaxDimensions[DimensionWidth], parentWidth) >= 0.0 {
		width := resolveValue(&node.Style.MaxDimensions[DimensionWidth], parentWidth)
		return width, MeasureModeAtMost
	}

	width := parentWidth
	widthMeasureMode := MeasureModeExactly
	if FloatIsUndefined(width) {
		widthMeasureMode = MeasureModeUndefined
	}
	return width, widthMeasureMode
}
func calcStartHeight(node *Node, parentWidth, parentHeight float32) (float32, MeasureMode) {
	if nodeIsStyleDimDefined(node, FlexDirectionColumn, parentHeight) {
		height := resolveValue(node.resolvedDimensions[dim[FlexDirectionColumn]], parentHeight)
		margin := nodeMarginForAxis(node, FlexDirectionColumn, parentWidth)
		return height + margin, MeasureModeExactly
	}
	if resolveValue(&node.Style.MaxDimensions[DimensionHeight], parentHeight) >= 0 {
		height := resolveValue(&node.Style.MaxDimensions[DimensionHeight], parentHeight)
		return height, MeasureModeAtMost
	}
	height := parentHeight
	heightMeasureMode := MeasureModeExactly
	if FloatIsUndefined(height) {
		heightMeasureMode = MeasureModeUndefined
	}
	return height, heightMeasureMode
}

// CalculateLayout calculates layout
func CalculateLayout(node *Node, parentWidth float32, parentHeight float32, parentDirection Direction) {
	// Increment the generation count. This will force the recursive routine to
	// visit
	// all dirty nodes at least once. Subsequent visits will be skipped if the
	// input
	// parameters don't change.
	currentGenerationCount++

	resolveDimensions(node)

	width, widthMeasureMode := calcStartWidth(node, parentWidth)
	height, heightMeasureMode := calcStartHeight(node, parentWidth, parentHeight)

	if layoutNodeInternal(node, width, height, parentDirection,
		widthMeasureMode, heightMeasureMode, parentWidth, parentHeight,
		true, "initial", node.Config) {
		nodeSetPosition(node, node.Layout.Direction, parentWidth, parentHeight, parentWidth)
		roundToPixelGrid(node, node.Config.PointScaleFactor, 0, 0)

		if gPrintTree {
			NodePrint(node, PrintOptionsLayout|PrintOptionsChildren|PrintOptionsStyle)
		}
	}
}

// SetExperimentalFeatureEnabled enables experimental feature
func (config *Config) SetExperimentalFeatureEnabled(feature ExperimentalFeature, enabled bool) {
	config.experimentalFeatures[feature] = enabled
}

// IsExperimentalFeatureEnabled returns if experimental feature is enabled
func (config *Config) IsExperimentalFeatureEnabled(feature ExperimentalFeature) bool {
	return config.experimentalFeatures[feature]
}

func log(node *Node, level LogLevel, format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

func assertCond(cond bool, format string, args ...interface{}) {
	if !cond {
		panic(format)
	}
}

func assertWithNode(node *Node, cond bool, format string, args ...interface{}) {
	assertCond(cond, format, args...)
}

func assertWithConfig(config *Config, condition bool, message string) {
	if !condition {
		panic(message)
	}
}
