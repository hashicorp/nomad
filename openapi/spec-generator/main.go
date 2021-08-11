package main

import (
	"fmt"
	"github.com/hashicorp/go-hclog"
	"os"
)

var logger = hclog.Default()

func main() {
	if len(os.Args) != 2 {
		logger.Info("spec-generator accepts only one argument which must be the path to the output file to generate")
		os.Exit(1)
	}

	os.Exit(run(os.Args[1]))
}

func run(outputPath string) int {
	builder := specBuilder{logger: logger}

	s, err := builder.buildSpec()
	if err != nil {
		logger.Error("specBuilder.buildSpec failed", err)
		return 1
	}

	var yaml []byte
	yaml, err = s.ToBytes()

	if err = os.WriteFile(outputPath, yaml, 0644); err != nil {
		logger.Error("SpecGenerator.run.os.WriteFile", err)
		return 1
	}

	logger.Info(fmt.Sprintf("specification generated at %s", outputPath))

	return 0
}
