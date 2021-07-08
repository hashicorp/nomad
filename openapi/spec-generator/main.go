package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/hashicorp/go-hclog"
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

func logWrapper(logger hclog.Logger) loggerFunc {
	return func(args ...interface{}) {
		logger.Log(hclog.Info, "", args)
	}
}

func run(g *Generator) error {
	var err error
	var analyzer *Analyzer

	logger := hclog.Default().(hclog.Logger)
	analyzer, err = NewAnalyzer(nomadPackages, logWrapper(logger), defaultDebugOptions)
	if err != nil {
		return err
	}

	spec, err := NewNomadSpecBuilder(analyzer).Build()
	if err != nil {
		return fmt.Errorf("Generator.run.NomadSpecBuilder.Build: ")
	}

	g.spec = spec

	if err = g.RenderTemplate(); err != nil {
		return fmt.Errorf("Generator.run.RenderTemplate: %v", err)
	}

	return nil
}
