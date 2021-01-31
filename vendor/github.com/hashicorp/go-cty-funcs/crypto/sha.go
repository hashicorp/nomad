package crypto

import (
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"

	"github.com/zclconf/go-cty/cty"
)

// Sha1Func is a function that computes the SHA1 hash of a given string and
// encodes it with hexadecimal digits.
var Sha1Func = makeStringHashFunction(sha1.New, hex.EncodeToString)

// Sha256Func is a function that computes the SHA256 hash of a given string and
// encodes it with hexadecimal digits.
var Sha256Func = makeStringHashFunction(sha256.New, hex.EncodeToString)

// Sha512Func is a function that computes the SHA512 hash of a given string and
// encodes it with hexadecimal digits.
var Sha512Func = makeStringHashFunction(sha512.New, hex.EncodeToString)

// Sha1 computes the SHA1 hash of a given string and encodes it with
// hexadecimal digits.
func Sha1(str cty.Value) (cty.Value, error) {
	return Sha1Func.Call([]cty.Value{str})
}

// Sha256 computes the SHA256 hash of a given string and encodes it with
// hexadecimal digits.
func Sha256(str cty.Value) (cty.Value, error) {
	return Sha256Func.Call([]cty.Value{str})
}

// Sha512 computes the SHA512 hash of a given string and encodes it with
// hexadecimal digits.
func Sha512(str cty.Value) (cty.Value, error) {
	return Sha512Func.Call([]cty.Value{str})
}
