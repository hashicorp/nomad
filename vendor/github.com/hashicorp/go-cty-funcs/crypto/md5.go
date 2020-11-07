package crypto

import (
	"crypto/md5"
	"encoding/hex"

	"github.com/zclconf/go-cty/cty"
)

// Md5Func is a function that computes the MD5 hash of a given string and
// encodes it with hexadecimal digits.
var Md5Func = makeStringHashFunction(md5.New, hex.EncodeToString)

// Md5 computes the MD5 hash of a given string and encodes it with hexadecimal
// digits.
func Md5(str cty.Value) (cty.Value, error) {
	return Md5Func.Call([]cty.Value{str})
}
