package secretdir

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

type TestSecretDir struct {
	// Dir is the path to the secret directory
	Dir string

	// MemoryUsed is returned when the MemoryUse function is called
	MemoryUsed int
}

func NewTestSecretDir(t *testing.T) *TestSecretDir {
	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("Failed to make tmp dir: %v", err)
	}

	s := &TestSecretDir{
		Dir: tmp,
	}

	return s
}

func (s *TestSecretDir) Destroy() error {
	return os.RemoveAll(s.Dir)
}

func (s *TestSecretDir) getPathFor(allocID, task string) string {
	return filepath.Join(s.Dir, fmt.Sprintf("%s-%s", allocID, task))
}

func (s *TestSecretDir) CreateFor(allocID, task string) (string, error) {
	path := s.getPathFor(allocID, task)
	if err := os.Mkdir(path, 0777); err != nil {
		return "", err
	}
	return path, nil
}

func (s *TestSecretDir) Remove(allocID, task string) error {
	path := s.getPathFor(allocID, task)
	return os.RemoveAll(path)
}

func (s *TestSecretDir) MemoryUse() int { return s.MemoryUsed }
