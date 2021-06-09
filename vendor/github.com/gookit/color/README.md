# CLI Color

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/gookit/color?style=flat-square)
[![Actions Status](https://github.com/gookit/color/workflows/action-tests/badge.svg)](https://github.com/gookit/color/actions)
[![Codacy Badge](https://api.codacy.com/project/badge/Grade/51b28c5f7ffe4cc2b0f12ecf25ed247f)](https://app.codacy.com/app/inhere/color)
[![GoDoc](https://godoc.org/github.com/gookit/color?status.svg)](https://pkg.go.dev/github.com/gookit/color?tab=overview)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/gookit/color)](https://github.com/gookit/color)
[![Build Status](https://travis-ci.org/gookit/color.svg?branch=master)](https://travis-ci.org/gookit/color)
[![Coverage Status](https://coveralls.io/repos/github/gookit/color/badge.svg?branch=master)](https://coveralls.io/github/gookit/color?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/gookit/color)](https://goreportcard.com/report/github.com/gookit/color)

A command-line color library with true color support, universal API methods and Windows support.

> **[中文说明](README.zh-CN.md)**

Basic color preview:

![basic-color](_examples/images/basic-color2.png)

Now, 256 colors and RGB colors have also been supported to work in Windows CMD and PowerShell:

![color-on-cmd-pwsh](_examples/images/color-on-cmd-pwsh.jpg)

## Features

  - Simple to use, zero dependencies
  - Supports rich color output: 16-color, 256-color, true color (24-bit, RGB)
    - 16-color output is the most commonly used and most widely supported, working on any Windows version
    - Since `v1.2.4` **the 256-color, true color (24-bit) support windows CMD and PowerShell**
    - See [this gist](https://gist.github.com/XVilka/8346728) for information on true color support
  - Generic API methods: `Print`, `Printf`, `Println`, `Sprint`, `Sprintf`
  - Supports HTML tag-style color rendering, such as `<green>message</>`. Support working on windows `cmd` `powerShell`
  - Basic colors: `Bold`, `Black`, `White`, `Gray`, `Red`, `Green`, `Yellow`, `Blue`, `Magenta`, `Cyan`
  - Additional styles: `Info`, `Note`, `Light`, `Error`, `Danger`, `Notice`, `Success`, `Comment`, `Primary`, `Warning`, `Question`, `Secondary`

## GoDoc

  - [godoc for gopkg](https://pkg.go.dev/gopkg.in/gookit/color.v1)
  - [godoc for github](https://pkg.go.dev/github.com/gookit/color)

## Install

```bash
go get github.com/gookit/color
```

## Quick start

```go
package main

import (
	"fmt"

	"github.com/gookit/color"
)

func main() {
	// quick use like fmt.Print*
	color.Red.Println("Simple to use color")
	color.Green.Print("Simple to use color\n")
	color.Cyan.Printf("Simple to use %s\n", "color")
	color.Yellow.Printf("Simple to use %s\n", "color")

	// use like func
	red := color.FgRed.Render
	green := color.FgGreen.Render
	fmt.Printf("%s line %s library\n", red("Command"), green("color"))

	// custom color
	color.New(color.FgWhite, color.BgBlack).Println("custom color style")

	// can also:
	color.Style{color.FgCyan, color.OpBold}.Println("custom color style")

	// internal theme/style:
	color.Info.Tips("message")
	color.Info.Prompt("message")
	color.Info.Println("message")
	color.Warn.Println("message")
	color.Error.Println("message")

	// use style tag
	color.Print("<suc>he</><comment>llo</>, <cyan>wel</><red>come</>\n")

	// apply a style tag
	color.Tag("info").Println("info style text")

	// prompt message
	color.Info.Prompt("prompt style message")
	color.Warn.Prompt("prompt style message")

	// tips message
	color.Info.Tips("tips style message")
	color.Warn.Tips("tips style message")
}
```

Run demo: `go run ./_examples/demo.go`

![colored-out](_examples/images/color-demo.jpg)

## Custom Build Color

```go
// Only use foreground color
color.FgCyan.Printf("Simple to use %s\n", "color")
// Only use background color
color.BgRed.Printf("Simple to use %s\n", "color")

// Full custom: foreground, background, option
myStyle := color.New(color.FgWhite, color.BgBlack, color.OpBold)
myStyle.Println("custom color style")

// can also:
color.Style{color.FgCyan, color.OpBold}.Println("custom color style")
```

custom set console settings:

```go
// set console color
color.Set(color.FgCyan)

// print message
fmt.Print("message")

// reset console settings
color.Reset()
```

## Basic Color

Supported on any Windows version.

  - `color.Bold`
  - `color.Black`
  - `color.White`
  - `color.Gray`
  - `color.Red`
  - `color.Green`
  - `color.Yellow`
  - `color.Blue`
  - `color.Magenta`
  - `color.Cyan`

```go
color.Bold.Println("bold message")
color.Yellow.Println("yellow message")
```

Run demo: `go run ./_examples/basiccolor.go`

![basic-color](_examples/images/basic-color.png)

## Additional styles

Supported on any Windows version.

  - `color.Info`
  - `color.Note`
  - `color.Warn`
  - `color.Light`
  - `color.Error`
  - `color.Danger`
  - `color.Debug`
  - `color.Notice`
  - `color.Success`
  - `color.Comment`
  - `color.Primary`
  - `color.Question`
  - `color.Secondary`

### Basic Style

print message use defined style:

```go
color.Info.Println("Info message")
color.Note.Println("Note message")
color.Notice.Println("Notice message")
color.Error.Println("Error message")
color.Danger.Println("Danger message")
color.Warn.Println("Warn message")
color.Debug.Println("Debug message")
color.Primary.Println("Primary message")
color.Question.Println("Question message")
color.Secondary.Println("Secondary message")
```

Run demo: `go run ./_examples/theme_basic.go`

![theme-basic](_examples/images/theme-basic.png)

### Tips Style

```go
color.Info.Tips("Info tips message")
color.Note.Tips("Note tips message")
color.Notice.Tips("Notice tips message")
color.Error.Tips("Error tips message")
color.Danger.Tips("Danger tips message")
color.Warn.Tips("Warn tips message")
color.Debug.Tips("Debug tips message")
color.Primary.Tips("Primary tips message")
color.Question.Tips("Question tips message")
color.Secondary.Tips("Secondary tips message")
```

Run demo: `go run ./_examples/theme_tips.go`

![theme-tips](_examples/images/theme-tips.png)

### Prompt Style

```go
color.Info.Prompt("Info prompt message")
color.Note.Prompt("Note prompt message")
color.Notice.Prompt("Notice prompt message")
color.Error.Prompt("Error prompt message")
color.Danger.Prompt("Danger prompt message")
color.Warn.Prompt("Warn prompt message")
color.Debug.Prompt("Debug prompt message")
color.Primary.Prompt("Primary prompt message")
color.Question.Prompt("Question prompt message")
color.Secondary.Prompt("Secondary prompt message")
```

Run demo: `go run ./_examples/theme_prompt.go`

![theme-prompt](_examples/images/theme-prompt.png)

### Block Style

```go
color.Info.Block("Info block message")
color.Note.Block("Note block message")
color.Notice.Block("Notice block message")
color.Error.Block("Error block message")
color.Danger.Block("Danger block message")
color.Warn.Block("Warn block message")
color.Debug.Block("Debug block message")
color.Primary.Block("Primary block message")
color.Question.Block("Question block message")
color.Secondary.Block("Secondary block message")
```

Run demo: `go run ./_examples/theme_block.go`

![theme-block](_examples/images/theme-block.png)

## HTML-like tag usage

**Supported** on Windows `cmd.exe` `PowerShell` .

```go
// use style tag
color.Print("<suc>he</><comment>llo</>, <cyan>wel</><red>come</>")
color.Println("<suc>hello</>")
color.Println("<error>hello</>")
color.Println("<warning>hello</>")

// custom color attributes
color.Print("<fg=yellow;bg=black;op=underscore;>hello, welcome</>\n")
```

- `color.Tag`

```go
// set a style tag
color.Tag("info").Print("info style text")
color.Tag("info").Printf("%s style text", "info")
color.Tag("info").Println("info style text")
```

Run demo: `go run ./_examples/colortag.go`

![color-tags](_examples/images/color-tags.png)

## 256-color usage

> 256 colors support Windows CMD, PowerShell environment after `v1.2.4`

### Set the foreground or background color

- `color.C256(val uint8, isBg ...bool) Color256`

```go
c := color.C256(132) // fg color
c.Println("message")
c.Printf("format %s", "message")

c := color.C256(132, true) // bg color
c.Println("message")
c.Printf("format %s", "message")
```

### Use a 256-color style

Can be used to set foreground and background colors at the same time.

- `color.S256(fgAndBg ...uint8) *Style256`

```go
s := color.S256(32, 203)
s.Println("message")
s.Printf("format %s", "message")
```

Run demo: `go run ./_examples/color256.go`

![color-tags](_examples/images/color-256.png)

## Use RGB color

> RGB colors support Windows `CMD`, `PowerShell` environment after `v1.2.4`

**Preview:**

> Run demo: `Run demo: go run ./_examples/color_rgb.go`

![color-rgb](_examples/images/color-rgb.png)

example:

```go
color.RGB(30, 144, 255).Println("message. use RGB number")

color.HEX("#1976D2").Println("blue-darken")
color.HEX("#D50000", true).Println("red-accent. use HEX style")

color.RGBStyleFromString("213,0,0").Println("red-accent. use RGB number")
color.HEXStyle("eee", "D50000").Println("deep-purple color")
```

### Set the foreground or background color

  - `color.RGB(r, g, b uint8, isBg ...bool) RGBColor`

```go
c := color.RGB(30,144,255) // fg color
c.Println("message")
c.Printf("format %s", "message")

c := color.RGB(30,144,255, true) // bg color
c.Println("message")
c.Printf("format %s", "message")
```

 Create a style from an hexadecimal color string:

  - `color.HEX(hex string, isBg ...bool) RGBColor`

```go
c := color.HEX("ccc") // can also: "cccccc" "#cccccc"
c.Println("message")
c.Printf("format %s", "message")

c = color.HEX("aabbcc", true) // as bg color
c.Println("message")
c.Printf("format %s", "message")
```

### Use an RGB color style

Can be used to set the foreground and background colors at the same time.

- `color.NewRGBStyle(fg RGBColor, bg ...RGBColor) *RGBStyle`

```go
s := color.NewRGBStyle(RGB(20, 144, 234), RGB(234, 78, 23))
s.Println("message")
s.Printf("format %s", "message")
```

 Create a style from an hexadecimal color string:

- `color.HEXStyle(fg string, bg ...string) *RGBStyle`

```go
s := color.HEXStyle("11aa23", "eee")
s.Println("message")
s.Printf("format %s", "message")
```

## Func refer

there are some useful functions reference

- `Disable()` disable color render
- `SetOutput(io.Writer)` custom set the colored text output writer
- `ForceOpenColor()` force open color render
- `ClearCode(str string) string` Use for clear color codes
- `ClearTag(s string) string` clear all color html-tag for a string
- `IsConsole(w io.Writer)` Determine whether w is one of stderr, stdout, stdin
- `HexToRgb(hex string) (rgb []int)` Convert hex color string to RGB numbers
- `RgbToHex(rgb []int) string` Convert RGB to hex code

## Gookit packages

  - [gookit/ini](https://github.com/gookit/ini) Go config management, use INI files
  - [gookit/rux](https://github.com/gookit/rux) Simple and fast request router for golang HTTP 
  - [gookit/gcli](https://github.com/gookit/gcli) build CLI application, tool library, running CLI commands
  - [gookit/event](https://github.com/gookit/event) Lightweight event manager and dispatcher implements by Go
  - [gookit/cache](https://github.com/gookit/cache) Generic cache use and cache manager for golang. support File, Memory, Redis, Memcached.
  - [gookit/config](https://github.com/gookit/config) Go config management. support JSON, YAML, TOML, INI, HCL, ENV and Flags
  - [gookit/color](https://github.com/gookit/color) A command-line color library with true color support, universal API methods and Windows support
  - [gookit/filter](https://github.com/gookit/filter) Provide filtering, sanitizing, and conversion of golang data
  - [gookit/validate](https://github.com/gookit/validate) Use for data validation and filtering. support Map, Struct, Form data
  - [gookit/goutil](https://github.com/gookit/goutil) Some utils for the Go: string, array/slice, map, format, cli, env, filesystem, test and more
  - More please see https://github.com/gookit

## See also

  - [`issue9/term`](https://github.com/issue9/term)
  - [`beego/bee`](https://github.com/beego/bee)
  - [`inhere/console`](https://github.com/inhere/php-console)
  - [ANSI escape code](https://en.wikipedia.org/wiki/ANSI_escape_code)

## License

[MIT](/LICENSE)
