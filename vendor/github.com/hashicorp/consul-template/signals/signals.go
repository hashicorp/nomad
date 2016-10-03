package signals

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

var SIGNIL os.Signal = new(NilSignal)

var ValidSignals []string

func init() {
	valid := make([]string, 0, len(SignalLookup))
	for k, _ := range SignalLookup {
		valid = append(valid, k)
	}
	sort.Strings(valid)
	ValidSignals = valid
}

// Parse parses the given string as a signal. If the signal is not found,
// an error is returned.
func Parse(s string) (os.Signal, error) {
	sig, ok := SignalLookup[strings.ToUpper(s)]
	if !ok {
		return nil, fmt.Errorf("invalid signal %q - valid signals are %q",
			sig, ValidSignals)
	}
	return sig, nil
}
