package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	var outputFile string

	flag.StringVar(&outputFile, "outputFile", "./", "The output file to save spec to")
	flag.Parse()

	g := &Generator{
		OutputFile: outputFile,
	}

	err := run(g)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
}

func run(g *Generator) error {
	spec, err := NewNomadSpecBuilder().Build()
	if err != nil {
		return fmt.Errorf("Generator.run.NomadSpecBuilder.Build: ")
	}

	g.spec = spec

	if err = g.RenderTemplate(); err != nil {
		return fmt.Errorf("Generator.run.RenderTemplate: %v", err)
	}

	return nil
}
