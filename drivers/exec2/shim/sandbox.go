// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package shim

import (
	"fmt"
	"os"
	"strings"

	"github.com/shoenig/go-landlock"
)

// When the nomad binary is invoked as exec2-shim, the format is
// nomad exec2-shim [path, [...]] -- [commands, [...]]
// so basically we need to find the first instance of '--' and split on that
func split(args []string) ([]string, []string) {
	var (
		paths    []string
		commands []string
	)

	index := 0
	for ; index < len(args); index++ {
		if args[index] == "--" {
			index++
			break
		}
		paths = append(paths, args[index])
	}

	for ; index < len(args); index++ {
		commands = append(commands, args[index])
	}

	return paths, commands
}

func lockdown(paths []string) error {
	conversions, err := convert(paths)
	if err != nil {
		return err
	}
	conversions = append(conversions, landlock.Shared())
	return landlock.New(conversions...).Lock(landlock.Mandatory)
}

func convert(paths []string) ([]*landlock.Path, error) {
	conversions := make([]*landlock.Path, 0, len(paths))

	for _, path := range paths {
		idx := strings.LastIndex(path, ":")
		if idx == -1 {
			return nil, fmt.Errorf("path %q does not contain mode suffix", path)
		}
		filepath := path[0:idx]
		mode := path[idx+1:]

		fmt.Println("PATH", path, "FILEPATH", filepath, "MODE", mode)

		info, err := os.Stat(filepath)
		if err != nil {
			return nil, fmt.Errorf("failed to stat unveil path: %w", err)
		}
		if info.IsDir() {
			conversions = append(conversions, landlock.Dir(filepath, mode))
		} else {
			conversions = append(conversions, landlock.File(filepath, mode))
		}
	}

	fmt.Println("CONVERSIONS", conversions)
	return conversions, nil
}
