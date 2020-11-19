package flex

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// NodePrinter node printer.
type NodePrinter struct {
	writer  io.Writer
	options PrintOptions
}

// NodePrint prints node to standard output.
func NodePrint(node *Node, options PrintOptions) {
	printer := NewNodePrinter(os.Stdout, options)
	printer.Print(node)
}

// NewNodePrinter creates new node printer.
func NewNodePrinter(writer io.Writer, options PrintOptions) *NodePrinter {
	return &NodePrinter{
		writer:  writer,
		options: options,
	}
}

// Print prints node.
func (printer *NodePrinter) Print(node *Node) {
	printer.printNode(node, 0)
}

func (printer *NodePrinter) printNode(node *Node, level int) {
	printer.printIndent(level)
	printer.printf("<div ")

	if node.Print != nil {
		node.Print(node)
	}

	if printer.options&PrintOptionsLayout != 0 {
		printer.printf("layout=\"")
		printer.printf("width: %g; ", node.Layout.Dimensions[DimensionWidth])
		printer.printf("height: %g; ", node.Layout.Dimensions[DimensionHeight])
		printer.printf("top: %g; ", node.Layout.Position[EdgeTop])
		printer.printf("left: %g;", node.Layout.Position[EdgeLeft])
		printer.printf("\" ")
	}

	if printer.options&PrintOptionsStyle != 0 {
		printer.printf("style=\"")
		if node.Style.FlexDirection != nodeDefaults.Style.FlexDirection {
			printer.printf("flex-direction: %s; ",
				FlexDirectionToString(node.Style.FlexDirection))
		}
		if node.Style.JustifyContent != nodeDefaults.Style.JustifyContent {
			printer.printf("justify-content: %s; ",
				JustifyToString(node.Style.JustifyContent))
		}
		if node.Style.AlignItems != nodeDefaults.Style.AlignItems {
			printer.printf("align-items: %s; ", AlignToString(node.Style.AlignItems))
		}
		if node.Style.AlignContent != nodeDefaults.Style.AlignContent {
			printer.printf("align-content: %s; ", AlignToString(node.Style.AlignContent))
		}
		if node.Style.AlignSelf != nodeDefaults.Style.AlignSelf {
			printer.printf("align-self: %s; ", AlignToString(node.Style.AlignSelf))
		}

		printer.printFloatIfNotUndefined(node, "flex-grow", node.Style.FlexGrow)
		printer.printFloatIfNotUndefined(node, "flex-shrink", node.Style.FlexShrink)
		printer.printNumberIfNotAuto(node, "flex-basis", &node.Style.FlexBasis)
		printer.printFloatIfNotUndefined(node, "flex", node.Style.Flex)

		if node.Style.FlexWrap != nodeDefaults.Style.FlexWrap {
			printer.printf("flexWrap: %s; ", WrapToString(node.Style.FlexWrap))
		}

		if node.Style.Overflow != nodeDefaults.Style.Overflow {
			printer.printf("overflow: %s; ", OverflowToString(node.Style.Overflow))
		}

		if node.Style.Display != nodeDefaults.Style.Display {
			printer.printf("display: %s; ", DisplayToString(node.Style.Display))
		}

		printer.printEdges(node, "margin", node.Style.Margin[:])
		printer.printEdges(node, "padding", node.Style.Padding[:])
		printer.printEdges(node, "border", node.Style.Border[:])

		printer.printNumberIfNotAuto(node, "width", &node.Style.Dimensions[DimensionWidth])
		printer.printNumberIfNotAuto(node, "height", &node.Style.Dimensions[DimensionHeight])
		printer.printNumberIfNotAuto(node, "max-width", &node.Style.MaxDimensions[DimensionWidth])
		printer.printNumberIfNotAuto(node, "max-height", &node.Style.MaxDimensions[DimensionHeight])
		printer.printNumberIfNotAuto(node, "min-width", &node.Style.MinDimensions[DimensionWidth])
		printer.printNumberIfNotAuto(node, "min-height", &node.Style.MinDimensions[DimensionHeight])

		if node.Style.PositionType != nodeDefaults.Style.PositionType {
			printer.printf("position: %s; ",
				PositionTypeToString(node.Style.PositionType))
		}

		printer.printEdgeIfNotUndefined(node, "left", node.Style.Position[:], EdgeLeft)
		printer.printEdgeIfNotUndefined(node, "right", node.Style.Position[:], EdgeRight)
		printer.printEdgeIfNotUndefined(node, "top", node.Style.Position[:], EdgeTop)
		printer.printEdgeIfNotUndefined(node, "bottom", node.Style.Position[:], EdgeBottom)
		printer.printf("\"")

		if node.Measure != nil {
			printer.printf(" has-custom-measure=\"true\"")
		}
	}
	printer.printf(">")

	childCount := len(node.Children)
	if printer.options&PrintOptionsChildren != 0 && childCount > 0 {
		for i := 0; i < childCount; i++ {
			printer.printf("\n")
			printer.printNode(node.Children[i], level+1)
		}
		printer.printIndent(level)
		printer.printf("\n")
	}
	if childCount != 0 {
		printer.printIndent(level)
	}
	printer.printf("</div>")
}

func (printer *NodePrinter) printEdges(node *Node, str string, edges []Value) {
	if fourValuesEqual(edges) {
		printer.printNumberIfNotZero(node, str, &edges[EdgeLeft])
		// bugfix for issue #5
		// if we set EdgeAll, the values are
		// [{NaN 0} {NaN 0} {NaN 0} {NaN 0} {NaN 0} {NaN 0} {NaN 0} {NaN 0} {20 1}]
		// so EdgeLeft is not printed and we won't print padding
		// for simplicity, I assume that EdgeAll is exclusive with setting specific edges
		// so we can print both and only one should show up
		// C code has this bug: https://github.com/facebook/yoga/blob/26481a6553a33d9c005f2b8d24a7952fc58df32c/yoga/Yoga.c#L1036
		printer.printNumberIfNotZero(node, str, &edges[EdgeAll])
	} else {
		for edge := EdgeLeft; edge < EdgeCount; edge++ {
			buf := fmt.Sprintf("%s-%s", str, EdgeToString(edge))
			printer.printNumberIfNotZero(node, buf, &edges[edge])
		}
	}
}

func (printer *NodePrinter) printEdgeIfNotUndefined(node *Node, str string, edges []Value, edge Edge) {
	printer.printNumberIfNotUndefined(node, str, computedEdgeValue(edges, edge, &ValueUndefined))
}

func (printer *NodePrinter) printFloatIfNotUndefined(node *Node, str string, number float32) {
	if !FloatIsUndefined(number) {
		printer.printf("%s: %g; ", str, number)
	}
}

func (printer *NodePrinter) printNumberIfNotUndefined(node *Node, str string, number *Value) {
	if number.Unit != UnitUndefined {
		if number.Unit == UnitAuto {
			printer.printf("%s: auto; ", str)
		} else {
			unit := "%"

			if number.Unit == UnitPoint {
				unit = "px"
			}
			printer.printf("%s: %g%s; ", str, number.Value, unit)
		}
	}
}

func (printer *NodePrinter) printNumberIfNotAuto(node *Node, str string, number *Value) {
	if number.Unit != UnitAuto {
		printer.printNumberIfNotUndefined(node, str, number)
	}
}

func (printer *NodePrinter) printNumberIfNotZero(node *Node, str string, number *Value) {
	if !FloatsEqual(number.Value, 0) {
		printer.printNumberIfNotUndefined(node, str, number)
	}
}

func (printer *NodePrinter) printf(format string, args ...interface{}) {
	fmt.Fprintf(printer.writer, format, args...)
}

func (printer *NodePrinter) printIndent(n int) {
	printer.writer.Write([]byte(strings.Repeat("  ", n)))
}

func fourValuesEqual(four []Value) bool {
	return ValueEqual(four[0], four[1]) && ValueEqual(four[0], four[2]) &&
		ValueEqual(four[0], four[3])
}
