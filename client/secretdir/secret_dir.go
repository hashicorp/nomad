package secretdir

import (
	"fmt"
	"os"
	"path/filepath"
)

const ()

type SecretDirectory interface {
	Destroy() error
	CreateFor(allocID, task string) (path string, err error)
	Remove(allocID, task string) error
}

type SecretDir struct {
	// Dir is the path to the secret directory
	Dir string
}

func NewSecretDir(dir string) (*SecretDir, error) {
	s := &SecretDir{
		Dir: dir,
	}

	if err := s.init(); err != nil {
		return nil, err
	}

	return s, nil
}

// init checks the secret directory exists and if it doesn't creates the secret
// directory
func (s *SecretDir) init() error {
	if _, err := os.Stat(s.Dir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat secret dir: %v", err)
	}

	// TODO this shouldn't be hardcoded
	if err := s.create(32); err != nil {
		return fmt.Errorf("failed to create secret dir: %v", err)
	}

	return nil
}

// Destroy is used to destroy the secret dir
func (s *SecretDir) Destroy() error {
	return s.destroy()
}

func (s *SecretDir) getPathFor(allocID, task string) string {
	return filepath.Join(s.Dir, fmt.Sprintf("%s-%s", allocID, task))
}

// CreateFor creates a secret directory for the given allocation and task. If
// the directory couldn't be created an error is returned, otherwise the path
// is.
func (s *SecretDir) CreateFor(allocID, task string) (string, error) {
	path := s.getPathFor(allocID, task)
	if err := os.Mkdir(path, 0777); err != nil {
		return "", err
	}
	return path, nil
}

// Remove deletes the secret directory for the given allocation and task
func (s *SecretDir) Remove(allocID, task string) error {
	path := s.getPathFor(allocID, task)
	return os.RemoveAll(path)
}
