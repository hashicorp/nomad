package manager

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

// RenderInput is used as input to the render function.
type RenderInput struct {
	Backup    bool
	Contents  []byte
	Dry       bool
	DryStream io.Writer
	Path      string
	Perms     os.FileMode
}

// RenderResult is returned and stored. It contains the status of the render
// operationg.
type RenderResult struct {
	// DidRender indicates if the template rendered to disk. This will be false in
	// the event of an error, but it will also be false in dry mode or when the
	// template on disk matches the new result.
	DidRender bool

	// WouldRender indicates if the template would have rendered to disk. This
	// will return false in the event of an error, but will return true in dry
	// mode or when the template on disk matches the new result.
	WouldRender bool

	// Contents are the actual contents of the resulting template from the render
	// operation.
	Contents []byte
}

// Render atomically renders a file contents to disk, returning a result of
// whether it would have rendered and actually did render.
func Render(i *RenderInput) (*RenderResult, error) {
	existing, err := ioutil.ReadFile(i.Path)
	if err != nil && !os.IsNotExist(err) {
		return nil, errors.Wrap(err, "failed reading file")
	}

	if bytes.Equal(existing, i.Contents) {
		return &RenderResult{
			DidRender:   false,
			WouldRender: true,
			Contents:    existing,
		}, nil
	}

	if i.Dry {
		fmt.Fprintf(i.DryStream, "> %s\n%s", i.Path, i.Contents)
	} else {
		if err := AtomicWrite(i.Path, i.Contents, i.Perms, i.Backup); err != nil {
			return nil, errors.Wrap(err, "failed writing file")
		}
	}

	return &RenderResult{
		DidRender:   true,
		WouldRender: true,
		Contents:    i.Contents,
	}, nil
}

// AtomicWrite accepts a destination path and the template contents. It writes
// the template contents to a TempFile on disk, returning if any errors occur.
//
// If the parent destination directory does not exist, it will be created
// automatically with permissions 0755. To use a different permission, create
// the directory first or use `chmod` in a Command.
//
// If the destination path exists, all attempts will be made to preserve the
// existing file permissions. If those permissions cannot be read, an error is
// returned. If the file does not exist, it will be created automatically with
// permissions 0644. To use a different permission, create the destination file
// first or use `chmod` in a Command.
//
// If no errors occur, the Tempfile is "renamed" (moved) to the destination
// path.
func AtomicWrite(path string, contents []byte, perms os.FileMode, backup bool) error {
	if path == "" {
		return fmt.Errorf("missing destination")
	}

	parent := filepath.Dir(path)
	if _, err := os.Stat(parent); os.IsNotExist(err) {
		if err := os.MkdirAll(parent, 0755); err != nil {
			return err
		}
	}

	f, err := ioutil.TempFile(parent, "")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())

	if _, err := f.Write(contents); err != nil {
		return err
	}

	if err := f.Sync(); err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	if err := os.Chmod(f.Name(), perms); err != nil {
		return err
	}

	// If we got this far, it means we are about to save the file. Copy the
	// current contents of the file onto disk (if it exists) so we have a backup.
	if backup {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			if err := copyFile(path, path+".bak"); err != nil {
				return err
			}
		}
	}

	if err := os.Rename(f.Name(), path); err != nil {
		return err
	}

	return nil
}

// copyFile copies the file at src to the path at dst. Any errors that occur
// are returned.
func copyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()

	stat, err := s.Stat()
	if err != nil {
		return err
	}

	d, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, stat.Mode())
	if err != nil {
		return err
	}
	if _, err := io.Copy(d, s); err != nil {
		d.Close()
		return err
	}
	return d.Close()
}
