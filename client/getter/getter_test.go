package getter

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"testing"
)

func TestGetArtifact_basic(t *testing.T) {

	logger := log.New(os.Stderr, "", log.LstdFlags)

	// TODO: Use http.TestServer to serve these files from fixtures dir
	passing := []struct {
		Source, Checksum string
	}{
		{
			"https://dl.dropboxusercontent.com/u/47675/jar_thing/hi_darwin_amd64",
			"sha256:66aa0f05fc0cfcf1e5ed8cc5307b5df51e33871d6b295a60e0f9f6dd573da977",
		},
		{
			"https://dl.dropboxusercontent.com/u/47675/jar_thing/hi_linux_amd64",
			"sha256:6f99b4c5184726e601ecb062500aeb9537862434dfe1898dbe5c68d9f50c179c",
		},
		{
			"https://dl.dropboxusercontent.com/u/47675/jar_thing/hi_linux_amd64",
			"md5:a9b14903a8942748e4f8474e11f795d3",
		},
		{
			"https://dl.dropboxusercontent.com/u/47675/jar_thing/hi_linux_amd64?checksum=sha256:6f99b4c5184726e601ecb062500aeb9537862434dfe1898dbe5c68d9f50c179c",
			"",
		},
		{
			"https://dl.dropboxusercontent.com/u/47675/jar_thing/hi_linux_amd64",
			"",
		},
	}

	for i, p := range passing {
		destDir, err := ioutil.TempDir("", fmt.Sprintf("nomad-test-%d", i))
		if err != nil {
			t.Fatalf("Error in TestGetArtifact_basic makeing TempDir: %s", err)
		}

		path, err := GetArtifact(destDir, p.Source, p.Checksum, logger)
		if err != nil {
			t.Fatalf("TestGetArtifact_basic unexpected failure here: %s", err)
		}

		if p.Checksum != "" {
			if ok := strings.Contains(path, p.Checksum); ok {
				t.Fatalf("path result should not contain the checksum, got: %s", path)
			}
		}

		// verify artifact exists
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("source path error: %s", err)
		}
	}
}

func TestGetArtifact_fails(t *testing.T) {

	logger := log.New(os.Stderr, "", log.LstdFlags)

	failing := []struct {
		Source, Checksum string
	}{
		{
			"",
			"sha256:66aa0f05fc0cfcf1e5ed8cc5307b5d",
		},
		{
			"/u/47675/jar_thing/hi_darwin_amd64",
			"sha256:66aa0f05fc0cfcf1e5ed8cc5307b5d",
		},
		{
			"https://dl.dropboxusercontent.com/u/47675/jar_thing/hi_darwin_amd64",
			"sha256:66aa0f05fc0cfcf1e5ed8cc5307b5d",
		},
		{
			"https://dl.dropboxusercontent.com/u/47675/jar_thing/hi_linux_amd64",
			"sha257:6f99b4c5184726e601ecb062500aeb9537862434dfe1898dbe5c68d9f50c179c",
		},
		// malformed checksum
		{
			"https://dl.dropboxusercontent.com/u/47675/jar_thing/hi_linux_amd64",
			"6f99b4c5184726e601ecb062500aeb9537862434dfe1898dbe5c68d9f50c179c",
		},
		// 404
		{
			"https://dl.dropboxusercontent.com/u/47675/jar_thing/hi_linux_amd86",
			"",
		},
	}
	for i, p := range failing {
		destDir, err := ioutil.TempDir("", fmt.Sprintf("nomad-test-%d", i))
		if err != nil {
			t.Fatalf("Error in TestGetArtifact_basic makeing TempDir: %s", err)
		}

		_, err = GetArtifact(destDir, p.Source, p.Checksum, logger)
		if err == nil {
			t.Fatalf("TestGetArtifact_basic expected failure, but got none")
		}
	}
}
