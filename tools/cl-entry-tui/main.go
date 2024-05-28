// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/huh"
)

var noteRe = regexp.MustCompile(`^[a-z0-9/\s]+: .+`)
var digitsRe = regexp.MustCompile(`^[0-9]+$`)

type PR struct {
	Number string
	Label  string
	Note   string
}

func (p *PR) Write() error {
	cNote, err := cleanup(p.Note)
	if err == nil {
		p.Note = cNote
	}
	return err
}

func main() {
	os.Exit(Main(os.Args[1:]...))
}

func Main(args ...string) int {
	pr := new(PR)
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Enter the PR number corresponding to this changelog entry:").
				Description("Note: This PR must already exist.").
				Value(&pr.Number).
				Validate(func(str string) error {
					str = strings.TrimSpace(str)
					if ok := digitsRe.Match([]byte(str)); !ok {
						return fmt.Errorf("value must only contain digits")
					}
					// TODO: Add PR presence validation here
					return nil
				}),
			// Ask the user for the kind of the PR.
			huh.NewSelect[string]().
				Title("Select the change kind:").
				Options(
					huh.NewOption("Bug", "bug"),
					huh.NewOption("Improvement", "improvement"),
					huh.NewOption("Security", "security"),
					huh.NewOption("Breaking Change", "breaking-change"),
					huh.NewOption("Note", "note"),
					huh.NewOption("Deprecation", "deprecation"),
				).
				Value(&pr.Label), // store the chosen option in the "burger" variable

			huh.NewText().
				Title("Write a note describing the change.").
				Description(`Example: "build: Added make target for creating changelog entries"`).
				CharLimit(400).
				Validate(huh.ValidateNotEmpty()).
				Validate(func(in string) error {
					clean, err := cleanup(in)
					if err == nil {
						pr.Note = clean
					}
					return err
				}).
				Value(&pr.Note),
		),
	)
	// TODO: Add a visual validation step here?
	err := form.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		return 1
	}

	file, err := write(*pr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		return 1
	}

	fmt.Println("Created", file)
	return 0
}

func write(pr PR) (string, error) {
	filename := filepath.Join(".changelog", fmt.Sprintf("%s.txt", pr.Number))
	sb := new(strings.Builder)
	sb.WriteString("```release-note:")
	sb.WriteString(pr.Label)
	sb.WriteString("\n")
	sb.WriteString(pr.Note)
	sb.WriteString("\n")
	sb.WriteString("```\n")
	s := sb.String()
	if err := os.WriteFile(filename, []byte(s), 0o644); err != nil {
		return "", err
	}
	return filename, nil
}

func cleanup(note string) (string, error) {
	note = strings.TrimSpace(note)
	note = strings.TrimSuffix(note, ".")
	if !noteRe.MatchString(note) {
		return "", errors.New("note does not comply with format")
	}
	return note, nil
}
