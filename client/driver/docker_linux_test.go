package driver

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDockerDriver_authFromHelper(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-docker-driver_authfromhelper")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	helperPayload := "{\"Username\":\"hashi\",\"Secret\":\"nomad\"}"
	helperContent := []byte(fmt.Sprintf("#!/bin/sh\ncat > %s/helper-$1.out;echo '%s'", dir, helperPayload))

	helperFile := filepath.Join(dir, "docker-credential-testnomad")
	err = ioutil.WriteFile(helperFile, helperContent, 0777)
	require.NoError(t, err)

	path := os.Getenv("PATH")
	os.Setenv("PATH", fmt.Sprintf("%s:%s", path, dir))
	defer os.Setenv("PATH", path)

	helper := authFromHelper("testnomad")
	creds, err := helper("registry.local:5000/repo/image")
	require.NoError(t, err)
	require.NotNil(t, creds)
	require.Equal(t, "hashi", creds.Username)
	require.Equal(t, "nomad", creds.Password)

	if _, err := os.Stat(filepath.Join(dir, "helper-get.out")); os.IsNotExist(err) {
		t.Fatalf("Expected helper-get.out to exist")
	}
	content, err := ioutil.ReadFile(filepath.Join(dir, "helper-get.out"))
	require.NoError(t, err)
	require.Equal(t, []byte("https://registry.local:5000"), content)
}
