package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/hashicorp/go-hclog"
)

func main() {
	var packageDir string
	var logLevel string

	flag.StringVar(&packageDir, "package-dir", "./", "Sets the package source directory to target")
	flag.StringVar(&logLevel, "log-level", "info", "Sets the logging level")
	flag.Parse()

	logger := hclog.New(&hclog.LoggerOptions{
		Level: hclog.LevelFromString(logLevel),
	})

	err := run(packageDir, logger)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
}

func run(packageDir string, logger hclog.Logger) error {
	var err error
	var pkgs []*ParsedPackage

	if pkgs, err = loadPackages(logger, packageDir); err != nil {
		return fmt.Errorf("error loading packages from %q: %w", packageDir, err)
	}
	if len(pkgs) == 0 {
		return fmt.Errorf("did not find any packages")
	}

	for _, pkg := range pkgs {
		logger.Debug("analyzing package", "package", pkg.Name)

		results, err := analyze(logger, pkg)
		if err != nil {
			return fmt.Errorf("error analyzing: %w", err)
		}
		if len(results) == 0 {
			return fmt.Errorf("did not analyze any types")
		}

		g := NewGenerator(packageDir, logger)
		if err = g.generate(results); err != nil {
			return fmt.Errorf("error generating: %v", err)
		}
	}
	return nil
}
