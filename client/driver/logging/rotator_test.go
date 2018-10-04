package logging

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/testutil"
)

var (
	pathPrefix   = "logrotator"
	baseFileName = "redis.stdout"
)

func TestFileRotator_IncorrectPath(t *testing.T) {
	t.Parallel()
	if _, err := NewFileRotator("/foo", baseFileName, 10, 10, testlog.HCLogger(t)); err == nil {
		t.Fatalf("expected error")
	}
}

func TestFileRotator_CreateNewFile(t *testing.T) {
	t.Parallel()
	var path string
	var err error
	if path, err = ioutil.TempDir("", pathPrefix); err != nil {
		t.Fatalf("test setup err: %v", err)
	}
	defer os.RemoveAll(path)

	_, err = NewFileRotator(path, baseFileName, 10, 10, testlog.HCLogger(t))
	if err != nil {
		t.Fatalf("test setup err: %v", err)
	}

	if _, err := os.Stat(filepath.Join(path, "redis.stdout.0")); err != nil {
		t.Fatalf("expected file")
	}
}

func TestFileRotator_OpenLastFile(t *testing.T) {
	t.Parallel()
	var path string
	var err error
	if path, err = ioutil.TempDir("", pathPrefix); err != nil {
		t.Fatalf("test setup err: %v", err)
	}
	defer os.RemoveAll(path)

	fname1 := filepath.Join(path, "redis.stdout.0")
	fname2 := filepath.Join(path, "redis.stdout.2")
	if _, err := os.Create(fname1); err != nil {
		t.Fatalf("test setup failure: %v", err)
	}
	if _, err := os.Create(fname2); err != nil {
		t.Fatalf("test setup failure: %v", err)
	}

	fr, err := NewFileRotator(path, baseFileName, 10, 10, testlog.HCLogger(t))
	if err != nil {
		t.Fatalf("test setup err: %v", err)
	}

	if fr.currentFile.Name() != fname2 {
		t.Fatalf("expected current file: %v, got: %v", fname2, fr.currentFile.Name())
	}
}

func TestFileRotator_WriteToCurrentFile(t *testing.T) {
	t.Parallel()
	var path string
	var err error
	if path, err = ioutil.TempDir("", pathPrefix); err != nil {
		t.Fatalf("test setup err: %v", err)
	}
	defer os.RemoveAll(path)

	fname1 := filepath.Join(path, "redis.stdout.0")
	if _, err := os.Create(fname1); err != nil {
		t.Fatalf("test setup failure: %v", err)
	}

	fr, err := NewFileRotator(path, baseFileName, 10, 5, testlog.HCLogger(t))
	if err != nil {
		t.Fatalf("test setup err: %v", err)
	}

	fr.Write([]byte("abcde"))

	var actual int64
	testutil.WaitForResult(func() (bool, error) {
		fi, err := os.Stat(fname1)
		if err != nil {
			return false, err
		}
		actual = fi.Size()
		if actual != 5 {
			return false, nil
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("expected size: %v, actual: %v", 5, actual)
	})
}

func TestFileRotator_RotateFiles(t *testing.T) {
	t.Parallel()
	var path string
	var err error
	if path, err = ioutil.TempDir("", pathPrefix); err != nil {
		t.Fatalf("test setup err: %v", err)
	}
	defer os.RemoveAll(path)

	fr, err := NewFileRotator(path, baseFileName, 10, 5, testlog.HCLogger(t))
	if err != nil {
		t.Fatalf("test setup err: %v", err)
	}

	str := "abcdefgh"
	nw, err := fr.Write([]byte(str))
	if err != nil {
		t.Fatalf("got error while writing: %v", err)
	}

	if nw != len(str) {
		t.Fatalf("expected %v, got %v", len(str), nw)
	}

	var lastErr error
	testutil.WaitForResult(func() (bool, error) {
		fname1 := filepath.Join(path, "redis.stdout.0")
		fi, err := os.Stat(fname1)
		if err != nil {
			lastErr = err
			return false, nil
		}
		if fi.Size() != 5 {
			lastErr = fmt.Errorf("expected size: %v, actual: %v", 5, fi.Size())
			return false, nil
		}

		fname2 := filepath.Join(path, "redis.stdout.1")
		if _, err := os.Stat(fname2); err != nil {
			lastErr = fmt.Errorf("expected file %v to exist", fname2)
			return false, nil
		}

		if fi2, err := os.Stat(fname2); err == nil {
			if fi2.Size() != 3 {
				lastErr = fmt.Errorf("expected size: %v, actual: %v", 3, fi2.Size())
				return false, nil
			}
		} else {
			lastErr = fmt.Errorf("error getting the file info: %v", err)
			return false, nil
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("%v", lastErr)
	})
}

func TestFileRotator_RotateFiles_Boundary(t *testing.T) {
	t.Parallel()
	var path string
	var err error
	if path, err = ioutil.TempDir("", pathPrefix); err != nil {
		t.Fatalf("test setup err: %v", err)
	}
	defer os.RemoveAll(path)

	fr, err := NewFileRotator(path, baseFileName, 10, 5, testlog.HCLogger(t))
	if err != nil {
		t.Fatalf("test setup err: %v", err)
	}

	// We will write three times:
	// 1st: Write with new lines spanning two files
	// 2nd: Write long string with no new lines
	// 3rd: Write a single new line
	expectations := [][]byte{
		[]byte("ab\n"),
		[]byte("cdef\n"),
		[]byte("12345"),
		[]byte("67890"),
		[]byte("\n"),
	}

	for _, str := range []string{"ab\ncdef\n", "1234567890", "\n"} {
		nw, err := fr.Write([]byte(str))
		if err != nil {
			t.Fatalf("got error while writing: %v", err)
		}

		if nw != len(str) {
			t.Fatalf("expected %v, got %v", len(str), nw)
		}
	}

	var lastErr error
	testutil.WaitForResult(func() (bool, error) {

		for i, exp := range expectations {
			fname := filepath.Join(path, fmt.Sprintf("redis.stdout.%d", i))
			fi, err := os.Stat(fname)
			if err != nil {
				lastErr = err
				return false, nil
			}
			if int(fi.Size()) != len(exp) {
				lastErr = fmt.Errorf("expected size: %v, actual: %v", len(exp), fi.Size())
				return false, nil
			}
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("%v", lastErr)
	})
}

func TestFileRotator_WriteRemaining(t *testing.T) {
	t.Parallel()
	var path string
	var err error
	if path, err = ioutil.TempDir("", pathPrefix); err != nil {
		t.Fatalf("test setup err: %v", err)
	}
	defer os.RemoveAll(path)

	fname1 := filepath.Join(path, "redis.stdout.0")
	if f, err := os.Create(fname1); err == nil {
		f.Write([]byte("abcd"))
	} else {
		t.Fatalf("test setup failure: %v", err)
	}

	fr, err := NewFileRotator(path, baseFileName, 10, 5, testlog.HCLogger(t))
	if err != nil {
		t.Fatalf("test setup err: %v", err)
	}

	str := "efghijkl"
	nw, err := fr.Write([]byte(str))
	if err != nil {
		t.Fatalf("got error while writing: %v", err)
	}
	if nw != len(str) {
		t.Fatalf("expected %v, got %v", len(str), nw)
	}
	var lastErr error
	testutil.WaitForResult(func() (bool, error) {
		fi, err := os.Stat(fname1)
		if err != nil {
			lastErr = fmt.Errorf("error getting the file info: %v", err)
			return false, nil
		}
		if fi.Size() != 5 {
			lastErr = fmt.Errorf("expected size: %v, actual: %v", 5, fi.Size())
			return false, nil
		}

		fname2 := filepath.Join(path, "redis.stdout.1")
		if _, err := os.Stat(fname2); err != nil {
			lastErr = fmt.Errorf("expected file %v to exist", fname2)
			return false, nil
		}

		if fi2, err := os.Stat(fname2); err == nil {
			if fi2.Size() != 5 {
				lastErr = fmt.Errorf("expected size: %v, actual: %v", 5, fi2.Size())
				return false, nil
			}
		} else {
			lastErr = fmt.Errorf("error getting the file info: %v", err)
			return false, nil
		}

		fname3 := filepath.Join(path, "redis.stdout.2")
		if _, err := os.Stat(fname3); err != nil {
			lastErr = fmt.Errorf("expected file %v to exist", fname3)
			return false, nil
		}

		if fi3, err := os.Stat(fname3); err == nil {
			if fi3.Size() != 2 {
				lastErr = fmt.Errorf("expected size: %v, actual: %v", 2, fi3.Size())
				return false, nil
			}
		} else {
			lastErr = fmt.Errorf("error getting the file info: %v", err)
			return false, nil
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("%v", lastErr)
	})

}

func TestFileRotator_PurgeOldFiles(t *testing.T) {
	t.Parallel()
	var path string
	var err error
	if path, err = ioutil.TempDir("", pathPrefix); err != nil {
		t.Fatalf("test setup err: %v", err)
	}
	defer os.RemoveAll(path)

	fr, err := NewFileRotator(path, baseFileName, 2, 2, testlog.HCLogger(t))
	if err != nil {
		t.Fatalf("test setup err: %v", err)
	}

	str := "abcdeghijklmn"
	nw, err := fr.Write([]byte(str))
	if err != nil {
		t.Fatalf("got error while writing: %v", err)
	}
	if nw != len(str) {
		t.Fatalf("expected %v, got %v", len(str), nw)
	}

	var lastErr error
	testutil.WaitForResult(func() (bool, error) {
		f, err := ioutil.ReadDir(path)
		if err != nil {
			lastErr = fmt.Errorf("test error: %v", err)
			return false, nil
		}

		if len(f) != 2 {
			lastErr = fmt.Errorf("expected number of files: %v, got: %v", 2, len(f))
			return false, nil
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("%v", lastErr)
	})
}

func BenchmarkRotator(b *testing.B) {
	kb := 1024
	for _, inputSize := range []int{kb, 2 * kb, 4 * kb, 8 * kb, 16 * kb, 32 * kb, 64 * kb, 128 * kb, 256 * kb} {
		b.Run(fmt.Sprintf("%dKB", inputSize/kb), func(b *testing.B) {
			benchmarkRotatorWithInputSize(inputSize, b)
		})
	}
}

func benchmarkRotatorWithInputSize(size int, b *testing.B) {
	var path string
	var err error
	if path, err = ioutil.TempDir("", pathPrefix); err != nil {
		b.Fatalf("test setup err: %v", err)
	}
	defer os.RemoveAll(path)

	fr, err := NewFileRotator(path, baseFileName, 5, 1024*1024, testlog.HCLogger(b))
	if err != nil {
		b.Fatalf("test setup err: %v", err)
	}
	b.ResetTimer()

	// run the Fib function b.N times
	for n := 0; n < b.N; n++ {
		// Generate some input
		data := make([]byte, size)
		_, err := rand.Read(data)
		if err != nil {
			b.Fatalf("Error generating date: %v", err)
		}

		// Insert random new lines
		for i := 0; i < 100; i++ {
			index := rand.Intn(size)
			data[index] = '\n'
		}

		// Write the data
		if _, err := fr.Write(data); err != nil {
			b.Fatalf("Failed to write data: %v", err)
		}
	}
}
