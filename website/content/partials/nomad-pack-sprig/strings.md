# String Functions

Sprig has a number of string manipulation functions.

## trim

The `trim` function removes space from either side of a string:

```
trim "   hello    "
```

The above produces `hello`

## trimAll

Remove given characters from the front or back of a string:

```
trimAll "$" "$5.00"
```

The above returns `5.00` (as a string).

## trimSuffix

Trim just the suffix from a string:

```
trimSuffix "-" "hello-"
```

The above returns `hello`

## trimPrefix

Trim just the prefix from a string:

```
trimPrefix "-" "-hello"
```

The above returns `hello`

## upper

Convert the entire string to uppercase:

```
upper "hello"
```

The above returns `HELLO`

## lower

Convert the entire string to lowercase:

```
lower "HELLO"
```

The above returns `hello`

## title

Convert to title case:

```
title "hello world"
```

The above returns `Hello World`

## untitle

Remove title casing. `untitle "Hello World"` produces `hello world`.

## repeat

Repeat a string multiple times:

```
repeat 3 "hello"
```

The above returns `hellohellohello`

## substr

Get a substring from a string. It takes three parameters:

- start (int)
- end (int)
- string (string)

```
substr 0 5 "hello world"
```

The above returns `hello`

## nospace

Remove all whitespace from a string.

```
nospace "hello w o r l d"
```

The above returns `helloworld`

## trunc

Truncate a string (and add no suffix)

```
trunc 5 "hello world"
```

The above produces `hello`.

```
trunc -5 "hello world"
```

The above produces `world`.

## abbrev

Truncate a string with ellipses (`...`)

Parameters:

- max length
- the string

```
abbrev 5 "hello world"
```

The above returns `he...`, since it counts the width of the ellipses against the
maximum length.

## abbrevboth

Abbreviate both sides:

```
abbrevboth 5 10 "1234 5678 9123"
```

the above produces `...5678...`

It takes:

- left offset
- max length
- the string

## initials

Given multiple words, take the first letter of each word and combine.

```
initials "First Try"
```

The above returns `FT`

## randAlphaNum, randAlpha, randNumeric, and randAscii

These four functions generate cryptographically secure (uses `crypto/rand`)
random strings, but with different base character sets:

- `randAlphaNum` uses `0-9a-zA-Z`
- `randAlpha` uses `a-zA-Z`
- `randNumeric` uses `0-9`
- `randAscii` uses all printable ASCII characters

Each of them takes one parameter: the integer length of the string.

```
randNumeric 3
```

The above will produce a random string with three digits.

## wrap

Wrap text at a given column count:

```
wrap 80 $someText
```

The above will wrap the string in `$someText` at 80 columns.

## wrapWith

`wrapWith` works as `wrap`, but lets you specify the string to wrap with.
(`wrap` uses `\n`)

```
wrapWith 5 "\t" "Hello World"
```

The above produces `hello world` (where the whitespace is an ASCII tab
character)

## contains

Test to see if one string is contained inside of another:

```
contains "cat" "catch"
```

The above returns `true` because `catch` contains `cat`.

## hasPrefix and hasSuffix

The `hasPrefix` and `hasSuffix` functions test whether a string has a given
prefix or suffix:

```
hasPrefix "cat" "catch"
```

The above returns `true` because `catch` has the prefix `cat`.

## quote and squote

These functions wrap a string in double quotes (`quote`) or single quotes
(`squote`).

## cat

The `cat` function concatenates multiple strings together into one, separating
them with spaces:

```
cat "hello" "beautiful" "world"
```

The above produces `hello beautiful world`

## indent

The `indent` function indents every line in a given string to the specified
indent width. This is useful when aligning multi-line strings:

```
indent 4 $lots_of_text
```

The above will indent every line of text by 4 space characters.

## nindent

The `nindent` function is the same as the indent function, but prepends a new
line to the beginning of the string.

```
nindent 4 $lots_of_text
```

The above will indent every line of text by 4 space characters and add a new
line to the beginning.

## replace

Perform simple string replacement.

It takes three arguments:

- string to replace
- string to replace with
- source string

```
"I Am Henry VIII" | replace " " "-"
```

The above will produce `I-Am-Henry-VIII`

## plural

Pluralize a string.

```
len $fish | plural "one anchovy" "many anchovies"
```

In the above, if the length of the string is 1, the first argument will be
printed (`one anchovy`). Otherwise, the second argument will be printed
(`many anchovies`).

The arguments are:

- singular string
- plural string
- length integer

NOTE: Sprig does not currently support languages with more complex pluralization
rules. And `0` is considered a plural because the English language treats it
as such (`zero anchovies`). The Sprig developers are working on a solution for
better internationalization.

## snakecase

Convert string from camelCase to snake_case.

```
snakecase "FirstName"
```

This above will produce `first_name`.

## camelcase

Convert string from snake_case to CamelCase

```
camelcase "http_server"
```

This above will produce `HttpServer`.

## kebabcase

Convert string from camelCase to kebab-case.

```
kebabcase "FirstName"
```

This above will produce `first-name`.

## swapcase

Swap the case of a string using a word based algorithm.

Conversion algorithm:

- Upper case character converts to Lower case
- Title case character converts to Lower case
- Lower case character after Whitespace or at start converts to Title case
- Other Lower case character converts to Upper case
- Whitespace is defined by unicode.IsSpace(char)

```
swapcase "This Is A.Test"
```

This above will produce `tHIS iS a.tEST`.

## shuffle

Shuffle a string.

```
shuffle "hello"
```

The above will randomize the letters in `hello`, perhaps producing `oelhl`.

## regexMatch, mustRegexMatch

Returns true if the input string contains any match of the regular expression.

```
regexMatch "^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\\.[A-Za-z]{2,}$" "test@acme.com"
```

The above produces `true`

`regexMatch` panics if there is a problem and `mustRegexMatch` returns an error to the
template engine if there is a problem.

## regexFindAll, mustRegexFindAll

Returns a slice of all matches of the regular expression in the input string.
The last parameter n determines the number of substrings to return, where -1 means return all matches

```
regexFindAll "[2,4,6,8]" "123456789" -1
```

The above produces `[2 4 6 8]`

`regexFindAll` panics if there is a problem and `mustRegexFindAll` returns an error to the
template engine if there is a problem.

## regexFind, mustRegexFind

Return the first (left most) match of the regular expression in the input string

```
regexFind "[a-zA-Z][1-9]" "abcd1234"
```

The above produces `d1`

`regexFind` panics if there is a problem and `mustRegexFind` returns an error to the
template engine if there is a problem.

## regexReplaceAll, mustRegexReplaceAll

Returns a copy of the input string, replacing matches of the Regexp with the replacement string replacement.
Inside string replacement, $ signs are interpreted as in Expand, so for instance $1 represents the text of the first submatch

```
regexReplaceAll "a(x*)b" "-ab-axxb-" "${1}W"
```

The above produces `-W-xxW-`

`regexReplaceAll` panics if there is a problem and `mustRegexReplaceAll` returns an error to the
template engine if there is a problem.

## regexReplaceAllLiteral, mustRegexReplaceAllLiteral

Returns a copy of the input string, replacing matches of the Regexp with the replacement string replacement
The replacement string is substituted directly, without using Expand

```
regexReplaceAllLiteral "a(x*)b" "-ab-axxb-" "${1}"
```

The above produces `-${1}-${1}-`

`regexReplaceAllLiteral` panics if there is a problem and `mustRegexReplaceAllLiteral` returns an error to the
template engine if there is a problem.

## regexSplit, mustRegexSplit

Slices the input string into substrings separated by the expression and returns a slice of the substrings between those expression matches. The last parameter `n` determines the number of substrings to return, where `-1` means return all matches

```
regexSplit "z+" "pizza" -1
```

The above produces `[pi a]`

`regexSplit` panics if there is a problem and `mustRegexSplit` returns an error to the
template engine if there is a problem.

## regexQuoteMeta

Returns a string that escapes all regular expression metacharacters inside the argument text;
the returned string is a regular expression matching the literal text.

```
regexQuoteMeta "1.2.3"
```

The above produces `1\.2\.3`

## See Also...

The [Conversion Functions](conversion.html) contain functions for converting
strings. The [String Slice Functions](string_slice.html) contains functions
for working with an array of strings.
