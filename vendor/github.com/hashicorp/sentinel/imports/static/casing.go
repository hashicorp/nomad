package static

import (
	"unicode"
	"unicode/utf8"
)

type casingBuffer struct {
	r         []byte
	runeBytes [utf8.UTFMax]byte
}

func (b *casingBuffer) write(r rune) {
	if r < utf8.RuneSelf {
		b.r = append(b.r, byte(r))
		return
	}
	n := utf8.EncodeRune(b.runeBytes[0:], r)
	b.r = append(b.r, b.runeBytes[0:n]...)
}

func (b *casingBuffer) indent() {
	if len(b.r) > 0 {
		b.r = append(b.r, '_')
	}
}

func underscore(s string) string {
	b := casingBuffer{r: make([]byte, 0, len(s))}
	var m rune
	var w bool
	for _, ch := range s {
		if unicode.IsUpper(ch) {
			if m != 0 {
				if !w {
					b.indent()
					w = true
				}
				b.write(m)
			}
			m = unicode.ToLower(ch)
		} else {
			if m != 0 {
				b.indent()
				b.write(m)
				m = 0
				w = false
			}
			b.write(ch)
		}
	}

	if m != 0 {
		if !w {
			b.indent()
		}
		b.write(m)
	}

	return string(b.r)
}
