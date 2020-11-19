package state

import "fmt"

var NamedColors = map[int]string{
	0:  "black",
	1:  "red",
	2:  "green",
	3:  "yellow",
	4:  "blue",
	5:  "magenta",
	6:  "cyan",
	7:  "white",
	8:  "bright black",
	9:  "bright red",
	10: "bright green",
	11: "bright yellow",
	12: "bright blue",
	13: "bright magenta",
	14: "bright cyan",
	15: "bright white",
}

func init() {
	for i := 232; i < 256; i++ {
		NamedColors[i] = fmt.Sprintf("gray%d", 256-i)
	}
}
