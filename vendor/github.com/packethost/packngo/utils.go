package packngo

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
)

var (
	timestampType = reflect.TypeOf(Timestamp{})
	Facilities    = []string{
		"yyz1", "nrt1", "atl1", "mrs1", "hkg1", "ams1",
		"ewr1", "sin1", "dfw1", "lax1", "syd1", "sjc1",
		"ord1", "iad1", "fra1", "sea1", "dfw2"}
	FacilityFeatures = []string{
		"baremetal", "layer_2", "backend_transfer", "storage", "global_ipv4"}
	UtilizationLevels = []string{"unavailable", "critical", "limited", "normal"}
	DevicePlans       = []string{"c2.medium.x86", "g2.large.x86",
		"m2.xlarge.x86", "x2.xlarge.x86", "baremetal_2a", "baremetal_2a2",
		"baremetal_1", "baremetal_3", "baremetal_2", "baremetal_s",
		"baremetal_0", "baremetal_1e",
	}
)

// Stringify creates a string representation of the provided message
func Stringify(message interface{}) string {
	var buf bytes.Buffer
	v := reflect.ValueOf(message)
	stringifyValue(&buf, v)
	return buf.String()
}

// StreamToString converts a reader to a string
func StreamToString(stream io.Reader) string {
	buf := new(bytes.Buffer)
	buf.ReadFrom(stream)
	return buf.String()
}

// contains tells whether a contains x.
func contains(a []string, x string) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
}

// stringifyValue was graciously cargoculted from the goprotubuf library
func stringifyValue(w io.Writer, val reflect.Value) {
	if val.Kind() == reflect.Ptr && val.IsNil() {
		w.Write([]byte("<nil>"))
		return
	}

	v := reflect.Indirect(val)

	switch v.Kind() {
	case reflect.String:
		fmt.Fprintf(w, `"%s"`, v)
	case reflect.Slice:
		w.Write([]byte{'['})
		for i := 0; i < v.Len(); i++ {
			if i > 0 {
				w.Write([]byte{' '})
			}

			stringifyValue(w, v.Index(i))
		}

		w.Write([]byte{']'})
		return
	case reflect.Struct:
		if v.Type().Name() != "" {
			w.Write([]byte(v.Type().String()))
		}

		// special handling of Timestamp values
		if v.Type() == timestampType {
			fmt.Fprintf(w, "{%s}", v.Interface())
			return
		}

		w.Write([]byte{'{'})

		var sep bool
		for i := 0; i < v.NumField(); i++ {
			fv := v.Field(i)
			if fv.Kind() == reflect.Ptr && fv.IsNil() {
				continue
			}
			if fv.Kind() == reflect.Slice && fv.IsNil() {
				continue
			}

			if sep {
				w.Write([]byte(", "))
			} else {
				sep = true
			}

			w.Write([]byte(v.Type().Field(i).Name))
			w.Write([]byte{':'})
			stringifyValue(w, fv)
		}

		w.Write([]byte{'}'})
	default:
		if v.CanInterface() {
			fmt.Fprint(w, v.Interface())
		}
	}
}
