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

func lockdown(defaults bool, elements []string) error {
	paths, err := convert(elements)
	if err != nil {
		return err
	}

	if defaults {
		paths = append(paths, landlock.Shared())
		paths = append(paths, landlock.Stdio())
		paths = append(paths, landlock.Tmp())
		paths = append(paths, landlock.DNS())
		paths = append(paths, landlock.Certs())
		paths = append(paths,
			landlock.Dir("/bin", "rx"),
			landlock.Dir("/usr/bin", "rx"),
			landlock.Dir("/usr/local/bin", "rx"),
		)
	}

	return landlock.New(paths...).Lock(landlock.Mandatory)
}

func convert(elements []string) ([]*landlock.Path, error) {
	paths := make([]*landlock.Path, 0, len(elements))

	for _, path := range elements {
		idx := strings.LastIndex(path, ":")
		if idx == -1 {
			return nil, fmt.Errorf("path %q does not contain mode prefix", path)
		}

		mode := path[0:idx]
		filepath := path[idx+1:]

		info, err := os.Stat(filepath)
		if err != nil {
			return nil, fmt.Errorf("failed to stat unveil path: %w", err)
		}

		if info.IsDir() {
			paths = append(paths, landlock.Dir(filepath, mode))
		} else {
			paths = append(paths, landlock.File(filepath, mode))
		}
	}

	return paths, nil
}
