package java

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

const oracleJDKOutput = `java version "1.7.0_80"
Java(TM) SE Runtime Environment (build 1.7.0_80-b15)
Java HotSpot(TM) 64-Bit Server VM (build 24.80-b11, mixed mode)
`

func TestDriver_parseJavaVersionOutput(t *testing.T) {
	cases := []struct {
		name    string
		output  string
		version string
		runtime string
		vm      string
	}{
		{
			"OracleJDK",
			oracleJDKOutput,
			"1.7.0_80",
			"Java(TM) SE Runtime Environment (build 1.7.0_80-b15)",
			"Java HotSpot(TM) 64-Bit Server VM (build 24.80-b11, mixed mode)",
		},
		{
			"OpenJDK",
			`openjdk version "11.0.1" 2018-10-16
			OpenJDK Runtime Environment 18.9 (build 11.0.1+13)
			OpenJDK 64-Bit Server VM 18.9 (build 11.0.1+13, mixed mode)`,
			"11.0.1",
			"OpenJDK Runtime Environment 18.9 (build 11.0.1+13)",
			"OpenJDK 64-Bit Server VM 18.9 (build 11.0.1+13, mixed mode)",
		},
		{
			"OpenJDK",
			`Picked up _JAVA_OPTIONS: -Xmx2048m -Xms512m
			openjdk version "11.0.1" 2018-10-16
			OpenJDK Runtime Environment 18.9 (build 11.0.1+13)
			OpenJDK 64-Bit Server VM 18.9 (build 11.0.1+13, mixed mode)`,
			"11.0.1",
			"OpenJDK Runtime Environment 18.9 (build 11.0.1+13)",
			"OpenJDK 64-Bit Server VM 18.9 (build 11.0.1+13, mixed mode)",
		},
		{
			"IcedTea",
			`java version "1.6.0_36"
			 OpenJDK Runtime Environment (IcedTea6 1.13.8) (6b36-1.13.8-0ubuntu1~12.04)
			 OpenJDK 64-Bit Server VM (build 23.25-b01, mixed mode)`,
			"1.6.0_36",
			"OpenJDK Runtime Environment (IcedTea6 1.13.8) (6b36-1.13.8-0ubuntu1~12.04)",
			"OpenJDK 64-Bit Server VM (build 23.25-b01, mixed mode)",
		},
		{
			"Eclipse OpenJ9",
			`openjdk version "1.8.0_192"
			OpenJDK Runtime Environment (build 1.8.0_192-b12_openj9)
			Eclipse OpenJ9 VM (build openj9-0.11.0, JRE 1.8.0 Linux amd64-64-Bit Compressed References
			20181107_95 (JIT enabled, AOT enabled)
			OpenJ9 - 090ff9dcd
			OMR - ea548a66
			JCL - b5a3affe73 based on jdk8u192-b12)`,
			"1.8.0_192",
			"OpenJDK Runtime Environment (build 1.8.0_192-b12_openj9)",
			"Eclipse OpenJ9 VM (build openj9-0.11.0, JRE 1.8.0 Linux amd64-64-Bit Compressed References",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			version, runtime, vm := parseJavaVersionOutput(c.output)

			require.Equal(t, c.version, version)
			require.Equal(t, c.runtime, runtime)
			require.Equal(t, c.vm, vm)
		})
	}
}

func TestDriver_javaVersionInfo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test requires bash to run")
	}

	initCmd := javaVersionCommand
	defer func() {
		javaVersionCommand = initCmd
	}()

	javaVersionCommand = []string{
		"/bin/sh", "-c",
		fmt.Sprintf("printf '%%s\n' '%s' >/dev/stderr", oracleJDKOutput),
	}

	version, runtime, vm, err := javaVersionInfo()
	require.NoError(t, err)
	require.Equal(t, "1.7.0_80", version)
	require.Equal(t, "Java(TM) SE Runtime Environment (build 1.7.0_80-b15)", runtime)
	require.Equal(t, "Java HotSpot(TM) 64-Bit Server VM (build 24.80-b11, mixed mode)", vm)

}

func TestDriver_javaVersionInfo_UnexpectedOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test requires bash to run")
	}

	initCmd := javaVersionCommand
	defer func() {
		javaVersionCommand = initCmd
	}()

	javaVersionCommand = []string{
		"/bin/sh", "-c",
		fmt.Sprintf("printf '%%s\n' '%s' >/dev/stderr", "unexpected java -version output"),
	}

	version, runtime, vm, err := javaVersionInfo()
	require.NoError(t, err)
	require.Equal(t, "", version)
	require.Equal(t, "", runtime)
	require.Equal(t, "", vm)
}

func TestDriver_javaVersionInfo_JavaVersionFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test requires bash to run")
	}

	initCmd := javaVersionCommand
	defer func() {
		javaVersionCommand = initCmd
	}()

	javaVersionCommand = []string{
		"/bin/sh", "-c",
		"exit 127",
	}

	version, runtime, vm, err := javaVersionInfo()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to check java version")

	require.Equal(t, "", version)
	require.Equal(t, "", runtime)
	require.Equal(t, "", vm)
}
