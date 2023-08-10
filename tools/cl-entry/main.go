// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	pr = `Must have a Pull Request already open.
  Enter PR # => `

	kind = `Choose type, one of
    1. bug
    2. improvement
    3. security
    4. breaking-change
    5. note
    6. deprecation
  Enter Kind => `

	note = `Write a note, for example
	build: Added make target for creating changelog entries
  Enter Note => `
)

var noteRe = regexp.MustCompile(`^[a-z0-9/\s]+: .+`)

func main() {
	pr, err := ask(pr)
	check(err)

	n, err := ask(kind)
	check(err)

	label, err := label(n)
	check(err)

	body, err := askStr(note)
	check(err)

	msg, err := cleanup(body)
	check(err)

	file, err := write(pr, label, msg)
	check(err)

	fmt.Println("Created", file)
}

func write(pr int, label, msg string) (string, error) {
	filename := filepath.Join(".changelog", fmt.Sprintf("%d.txt", pr))
	sb := new(strings.Builder)
	sb.WriteString("```release-note:")
	sb.WriteString(label)
	sb.WriteString("\n")
	sb.WriteString(msg)
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

func label(n int) (string, error) {
	var label string
	switch n {
	case 1:
		label = "bug"
	case 2:
		label = "improvement"
	case 3:
		label = "security"
	case 4:
		label = "breaking-change"
	case 5:
		label = "note"
	case 6:
		label = "deprecation"
	default:
		return "", errors.New("not a valid type, must be 1-6")
	}
	return label, nil
}

func ask(q string) (int, error) {
	r := bufio.NewReader(os.Stdin)
	fmt.Print(q)
	line, err := r.ReadString('\n')
	if err != nil {
		return 0, err
	}
	i, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		return 0, err
	}
	return i, nil
}

func askStr(q string) (string, error) {
	r := bufio.NewReader(os.Stdin)
	fmt.Print(q)
	line, err := r.ReadString('\n')
	return strings.TrimSpace(line), err
}

func check(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "failure: %v\n", err)
		os.Exit(1)
	}
}
