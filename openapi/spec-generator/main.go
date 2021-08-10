package spec

import (
	"fmt"
	"os"
)

func main() {
	g := &Generator{}

	err := run(g)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
}

func run(g *Generator) error {
	if err := g.run(); err != nil {
		return fmt.Errorf("spec-generator.init: %v", err)
	}

	return nil
}
