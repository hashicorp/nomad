[![GoDoc](https://godoc.org/github.com/hashicorp/go-envparse?status.svg)](https://godoc.org/github.com/hashicorp/go-envparse)
[![Build Status](https://travis-ci.org/hashicorp/go-envparse.svg?branch=master)](https://travis-ci.org/hashicorp/go-envparse)
[![Coverage Status](https://coveralls.io/repos/github/hashicorp/go-envparse/badge.svg?branch=master)](https://coveralls.io/github/hashicorp/go-envparse?branch=master)

# go-envparse

A minimal Go environment variable parser. It's intended to be used to parse
`.env` style files similar to [godotenv](https://github.com/joho/godotenv) or
[rubydotenv](https://github.com/bkeepers/dotenv), but perform minimal
allocations, handle more complex quoting, and be better tested.

Parsing a line does 2 allocations regardless of line length or complexity.

The parser supports JSON strings which allows for cross-language/platform
encoding of arbitrarily complex data.

For example if you are parsing environment variables from a templated file, the
template can JSON encode data that may contain newlines:

```
FOO={{ some_template_function | toJSON }}
```

...would be templated to:

```
FOO="The template value\nmay have included\nsome newlines!\n\ud83d\udd25"
```

...and `envparse.Parse()` would return:

```go
map[string]string{
	"FOO": "The template value\nmay have included\nsome newlines!\n🔥",
}
```

## Minimal

The following common features *are intentionaly missing*:

* Full shell quoting semantics
* Full shell escape sequence support
  * Only JSON escape sequences are supported (see below)
* Variable interpolation
  * Use [Go's os.Expand](https://golang.org/pkg/os/#Expand) on the parsed
    values
* Anything YAML related
  * No

However, comments, unquoted, single quoted, and double quoted text may all be
used within a single value:

```
SOME_KEY = normal unquoted \text 'plus single quoted\' "\"double quoted " # EOL
```

...parses to:

```go
map[string]string{
	"SOME_KEY": `normal unquoted \text plus single quoted\ "double quoted `
}
```

(Note the trailing space inside the double quote is kept, but the space between
the final `"` and `#` is trimmed.)

## Format

* Keys should be of the form: `[A-Za-z_][A-Za-z0-9_]?`
  * Keys may be prefixed with `export ` which will be ignored
  * Whitespace around keys will be trimmed
* Values should be valid ASCII or UTF-8 encoded.
* Newlines are always treated as delimiters, so newlines within values *must*
  be escaped.
* Values may use one of more quoting styles:
  * Unquoted - `FOO=bar baz`
    * No escape sequences
    * Ends at `#`, `"`, `'`, or newline
    * Preceeding and trailing whitespace will be trimmed
  * Double Quotes - `FOO="bar baz"`
    * Supports JSON escape sequences: `\uXXXX`, `\r`, `\n`, `\t`, `\\`, and
      `\"`
    * Ends at unescaped `"`
    * No whitespace trimming
  * Single Quotes - `FOO='bar baz'`
    * No escape sequences
    * Ends at `'`
    * No whitespace trimming

See `envparse_test.go` for examples of valid and invalid data.
