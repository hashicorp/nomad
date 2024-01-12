// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package renderer

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"

	"github.com/hashicorp/consul-template/renderer"
)

const (
	// DefaultFilePerms are the default file permissions for files rendered onto
	// disk when a specific file permission has not already been specified.
	DefaultFilePerms = 0o644

	ExitDidRender           = 0
	ExitError               = 1
	ExitWouldRenderButDidnt = 117 // something unmistakeably belonging to Nomad
)

// This init() must be initialized last in package required by the child plugin
// process. It's recommended to avoid any other `init()` or inline any necessary
// calls here. See eeaa95d commit message for more details.
func init() {
	if len(os.Args) > 1 && os.Args[1] == "template-render" {

		if len(os.Args) <= 3 {
			// note: we don't use logger here as any message we send will get
			// wrapped by CT's own logger, but it's important to keep Stderr and
			// Stdout separate so that "read" has a clean output.
			fmt.Fprintln(os.Stderr, `expected "read" or "write" argument`)
		}

		switch os.Args[2] {
		case "read":
			err := readTemplate()
			if err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
				os.Exit(ExitError)
			}
			os.Exit(0)

		case "write":
			result, err := writeTemplate()
			if err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
				os.Exit(ExitError)
			}

			if result.DidRender {
				os.Exit(ExitDidRender)
			}
			if result.WouldRender {
				os.Exit(ExitWouldRenderButDidnt)
			}
			os.Exit(ExitError)
		default:
			fmt.Fprintln(os.Stderr, `expected "read" or "write" argument`)
			os.Exit(ExitError)
		}
	}
}

func readTemplate() error {
	var (
		sandboxPath, sourcePath string
		err                     error
	)

	flags := flag.NewFlagSet("template-render", flag.ExitOnError)
	flags.StringVar(&sandboxPath, "sandbox-path", "", "")
	flags.StringVar(&sourcePath, "source-path", "", "")
	flags.Parse(os.Args[3:])

	sourcePath, err = sandbox(sandboxPath, sourcePath) // platform-specific sandboxing
	if err != nil {
		return fmt.Errorf("failed to sandbox alloc dir %q: %w", sandboxPath, err)
	}

	f, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source file %q: %w", sourcePath, err)
	}
	defer f.Close()

	_, err = io.Copy(os.Stdout, f)
	return err
}

func writeTemplate() (*renderer.RenderResult, error) {

	var (
		sandboxPath, destPath, perms, user, group string
	)

	flags := flag.NewFlagSet("template-render", flag.ExitOnError)
	flags.StringVar(&sandboxPath, "sandbox-path", "", "")
	flags.StringVar(&destPath, "dest-path", "", "")
	flags.StringVar(&perms, "perms", "", "")
	flags.StringVar(&user, "user", "", "")
	flags.StringVar(&group, "group", "", "")

	flags.Parse(os.Args[3:])

	contents := new(bytes.Buffer)
	_, err := io.Copy(contents, os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("failed reading template contents: %w", err)
	}

	destPath, err = sandbox(sandboxPath, destPath) // platform-specific sandboxing
	if err != nil {
		return nil, fmt.Errorf("failed to sandbox alloc dir %q: %w", sandboxPath, err)
	}

	// perms must parse into a valid file permission
	fileMode := os.FileMode(DefaultFilePerms)
	if perms != "" {
		fileModeInt, err := strconv.ParseUint(perms, 8, 32)
		if err != nil {
			return nil, fmt.Errorf(
				"Invalid file mode %q: Must be a valid octal number: %w", perms, err)

		}
		fileMode = fs.FileMode(fileModeInt)
		if fileMode.Perm() != fileMode {
			return nil, fmt.Errorf(
				"Invalid file mode %q: Must be a valid Unix permission: %w", perms, err)
		}
	}

	input := &renderer.RenderInput{
		Backup:         false,
		Contents:       contents.Bytes(),
		CreateDestDirs: true,
		Dry:            false,
		DryStream:      nil,
		Path:           destPath,
		Perms:          fileMode,
		User:           user,
		Group:          group,
	}

	return renderer.Render(input)
}
