package crypto

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

// RsaDecryptFunc is a function that decrypts an RSA-encrypted ciphertext.
var RsaDecryptFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name: "ciphertext",
			Type: cty.String,
		},
		{
			Name: "privatekey",
			Type: cty.String,
		},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (ret cty.Value, err error) {
		s := args[0].AsString()
		key := args[1].AsString()

		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return cty.UnknownVal(cty.String), fmt.Errorf("failed to decode input %q: cipher text must be base64-encoded", s)
		}

		block, _ := pem.Decode([]byte(key))
		if block == nil {
			return cty.UnknownVal(cty.String), fmt.Errorf("failed to parse key: no key found")
		}
		if block.Headers["Proc-Type"] == "4,ENCRYPTED" {
			return cty.UnknownVal(cty.String), fmt.Errorf(
				"failed to parse key: password protected keys are not supported. Please decrypt the key prior to use",
			)
		}

		x509Key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return cty.UnknownVal(cty.String), err
		}

		out, err := rsa.DecryptPKCS1v15(nil, x509Key, b)
		if err != nil {
			return cty.UnknownVal(cty.String), err
		}

		return cty.StringVal(string(out)), nil
	},
})

// RsaDecrypt decrypts an RSA-encrypted ciphertext, returning the corresponding
// cleartext.
func RsaDecrypt(ciphertext, privatekey cty.Value) (cty.Value, error) {
	return RsaDecryptFunc.Call([]cty.Value{ciphertext, privatekey})
}
