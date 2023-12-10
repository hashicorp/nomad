// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package java

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	rt "runtime"
	"strings"
)

var javaVersionCommand = []string{"java", "-version"}
var macOSJavaTestCommand = "/usr/libexec/java_home"

func checkForMacJVM() (ok bool, err error) {
	// test for java differently because of the shim application
	var out bytes.Buffer
	cmd := exec.Command(macOSJavaTestCommand)
	cmd.Stdout = &out
	cmd.Stderr = &out
	err = cmd.Run()
	if err != nil {
		err = fmt.Errorf("failed check for macOS jvm: %v, out: %v", err, strings.ReplaceAll(strings.ReplaceAll(out.String(), "\n", " "), `"`, `\"`))
		return false, err
	}
	return true, nil
}

func javaVersionInfo() (version, runtime, vm string, err error) {
	var out bytes.Buffer

	if rt.GOOS == "darwin" {
		_, err = checkForMacJVM()
		if err != nil {
			err = fmt.Errorf("failed to check java version: %v", err)
			return
		}
	}

	cmd := exec.Command(javaVersionCommand[0], javaVersionCommand[1:]...)
	cmd.Stdout = &out
	cmd.Stderr = &out
	err = cmd.Run()
	if err != nil {
		err = fmt.Errorf("failed to check java version: %v", err)
		return
	}

	version, runtime, vm = parseJavaVersionOutput(out.String())
	return
}

var (
	javaVersionRe = regexp.MustCompile(`([.\d_]+)`)
)

func parseJavaVersionOutput(infoString string) (version, runtime, vm string) {
	infoString = strings.TrimSpace(infoString)

	lines := strings.Split(infoString, "\n")
	if strings.Contains(lines[0], "Picked up _JAVA_OPTIONS") {
		lines = lines[1:]
	}

	if len(lines) < 3 {
		// unexpected output format, don't attempt to parse output for version
		return "", "", ""
	}

	versionString := strings.TrimSpace(lines[0])

	if match := javaVersionRe.FindStringSubmatch(versionString); len(match) == 2 {
		versionString = match[1]
	}

	return versionString, strings.TrimSpace(lines[1]), strings.TrimSpace(lines[2])
}
