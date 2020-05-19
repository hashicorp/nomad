package hclutil

import (
	"unicode"
	"unicode/utf8"

	"github.com/hashicorp/hcl/v2/hclsimple"
)

func Decode(filename string, src string, target interface{}) error {
	srcBytes := []byte(src)
	return hclsimple.Decode(filename+ext(srcBytes), srcBytes, nil, target)
}

func ext(v []byte) string {
	var (
		r      rune
		w      int
		offset int
	)

	for {
		r, w = utf8.DecodeRune(v[offset:])
		offset += w
		if unicode.IsSpace(r) {
			continue
		}
		if r == '{' {
			return ".json"
		}
		break
	}

	return ".hcl"

}
