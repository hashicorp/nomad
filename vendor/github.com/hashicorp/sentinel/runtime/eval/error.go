package eval

import (
	"bytes"
	"errors"

	"github.com/hashicorp/sentinel/lang/object"
)

func (e *evalState) funcError(args []object.Object) (interface{}, error) {
	// Add it to the print buffer as well
	printArgs := make([]object.Object, len(args)+1)
	printArgs[0] = &object.StringObj{Value: "error: "}
	copy(printArgs[1:], args)
	e.funcPrint(printArgs)

	// Collect into our own buffer
	var errorBuf bytes.Buffer
	e.funcPrintBuf(&errorBuf, args)
	return nil, errors.New(errorBuf.String())
}
