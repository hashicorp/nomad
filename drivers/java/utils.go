package java

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

var javaVersionCommand = []string{"java", "-version"}

func javaVersionInfo() (version, runtime, vm string, err error) {
	var out bytes.Buffer

	cmd := exec.Command(javaVersionCommand[0], javaVersionCommand[1:]...)
	cmd.Stdout = &out
	cmd.Stderr = &out
	err = cmd.Run()
	if err != nil {
		return
	}

	version, runtime, vm, err = parseJavaVersionOutput(out.String())
	return
}

func parseJavaVersionOutput(infoString string) (version, runtime, vm string, err error) {
	infoString = strings.TrimSpace(infoString)

	lines := strings.Split(infoString, "\n")
	if len(lines) != 3 {
		return "", "", "", fmt.Errorf("unexpected java version info output, expected 3 lines but got: %v", infoString)
	}

	versionString := strings.TrimSpace(lines[0])

	re := regexp.MustCompile(`version "([^"]*)"`)
	if match := re.FindStringSubmatch(lines[0]); len(match) == 2 {
		versionString = match[1]
	}

	return versionString, strings.TrimSpace(lines[1]), strings.TrimSpace(lines[2]), nil
}
