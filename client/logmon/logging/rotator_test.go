// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package logging

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

var (
	baseFileName = "redis.stdout"
)

func TestFileRotator_IncorrectPath(t *testing.T) {
	defer goleak.VerifyNone(t)

	_, err := NewFileRotator("/foo", baseFileName, 10, 10, testlog.HCLogger(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "no such file or directory")
}

func TestFileRotator_CreateNewFile(t *testing.T) {
	defer goleak.VerifyNone(t)

	path := t.TempDir()

	fr, err := NewFileRotator(path, baseFileName, 10, 10, testlog.HCLogger(t))
	require.NoError(t, err)
	defer fr.Close()

	_, err = os.Stat(filepath.Join(path, "redis.stdout.0"))
	require.NoError(t, err)
}

func TestFileRotator_OpenLastFile(t *testing.T) {
	defer goleak.VerifyNone(t)

	path := t.TempDir()

	fname1 := filepath.Join(path, "redis.stdout.0")
	fname2 := filepath.Join(path, "redis.stdout.2")

	f1, err := os.Create(fname1)
	require.NoError(t, err)
	f1.Close()

	f2, err := os.Create(fname2)
	require.NoError(t, err)
	f2.Close()

	fr, err := NewFileRotator(path, baseFileName, 10, 10, testlog.HCLogger(t))
	require.NoError(t, err)
	defer fr.Close()

	require.Equal(t, fname2, fr.currentFile.Name())
}

func TestFileRotator_WriteToCurrentFile(t *testing.T) {
	defer goleak.VerifyNone(t)

	path := t.TempDir()

	fname1 := filepath.Join(path, "redis.stdout.0")
	f1, err := os.Create(fname1)
	require.NoError(t, err)
	f1.Close()

	fr, err := NewFileRotator(path, baseFileName, 10, 5, testlog.HCLogger(t))
	require.NoError(t, err)
	defer fr.Close()

	fr.Write([]byte("abcde"))

	testutil.WaitForResult(func() (bool, error) {
		fi, err := os.Stat(fname1)
		if err != nil {
			return false, fmt.Errorf("failed to stat file %v: %w", fname1, err)
		}
		actual := fi.Size()
		if actual != 5 {
			return false, fmt.Errorf("expected size %d but found %d", 5, actual)
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})
}

func TestFileRotator_RotateFiles(t *testing.T) {
	defer goleak.VerifyNone(t)

	path := t.TempDir()

	fr, err := NewFileRotator(path, baseFileName, 10, 5, testlog.HCLogger(t))
	require.NoError(t, err)
	defer fr.Close()

	str := "abcdefgh"
	nw, err := fr.Write([]byte(str))
	require.NoError(t, err)
	require.Equal(t, len(str), nw)

	testutil.WaitForResult(func() (bool, error) {
		fname1 := filepath.Join(path, "redis.stdout.0")
		fi, err := os.Stat(fname1)
		if err != nil {
			return false, fmt.Errorf("failed to stat file %v: %w", fname1, err)
		}
		if fi.Size() != 5 {
			return false, fmt.Errorf("expected size: %v, actual: %v", 5, fi.Size())
		}

		fname2 := filepath.Join(path, "redis.stdout.1")
		if _, err := os.Stat(fname2); err != nil {
			return false, fmt.Errorf("expected file %v to exist", fname2)
		}

		if fi2, err := os.Stat(fname2); err == nil {
			if fi2.Size() != 3 {
				return false, fmt.Errorf("expected size: %v, actual: %v", 3, fi2.Size())
			}
		} else {
			return false, fmt.Errorf("error getting the file info: %v", err)
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})
}

func TestFileRotator_RotateFiles_Boundary(t *testing.T) {
	defer goleak.VerifyNone(t)

	path := t.TempDir()

	fr, err := NewFileRotator(path, baseFileName, 10, 5, testlog.HCLogger(t))
	require.NoError(t, err)
	defer fr.Close()

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
		require.NoError(t, err)
		require.Equal(t, len(str), nw)
	}

	testutil.WaitForResult(func() (bool, error) {

		for i, exp := range expectations {
			fname := filepath.Join(path, fmt.Sprintf("redis.stdout.%d", i))
			fi, err := os.Stat(fname)
			if err != nil {
				return false, fmt.Errorf("failed to stat file %v: %w", fname, err)
			}
			if int(fi.Size()) != len(exp) {
				return false, fmt.Errorf("expected size: %v, actual: %v", len(exp), fi.Size())
			}
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})
}

func TestFileRotator_WriteRemaining(t *testing.T) {
	defer goleak.VerifyNone(t)

	path := t.TempDir()

	fname1 := filepath.Join(path, "redis.stdout.0")
	err := os.WriteFile(fname1, []byte("abcd"), 0600)
	require.NoError(t, err)

	fr, err := NewFileRotator(path, baseFileName, 10, 5, testlog.HCLogger(t))
	require.NoError(t, err)
	defer fr.Close()

	str := "efghijkl"
	nw, err := fr.Write([]byte(str))
	require.NoError(t, err)
	require.Equal(t, len(str), nw)

	testutil.WaitForResult(func() (bool, error) {
		fi, err := os.Stat(fname1)
		if err != nil {
			return false, fmt.Errorf("error getting the file info: %v", err)
		}
		if fi.Size() != 5 {
			return false, fmt.Errorf("expected size: %v, actual: %v", 5, fi.Size())
		}

		fname2 := filepath.Join(path, "redis.stdout.1")
		if _, err := os.Stat(fname2); err != nil {
			return false, fmt.Errorf("expected file %v to exist", fname2)
		}

		if fi2, err := os.Stat(fname2); err == nil {
			if fi2.Size() != 5 {
				return false, fmt.Errorf("expected size: %v, actual: %v", 5, fi2.Size())
			}
		} else {
			return false, fmt.Errorf("error getting the file info: %v", err)
		}

		fname3 := filepath.Join(path, "redis.stdout.2")
		if _, err := os.Stat(fname3); err != nil {
			return false, fmt.Errorf("expected file %v to exist", fname3)
		}

		if fi3, err := os.Stat(fname3); err == nil {
			if fi3.Size() != 2 {
				return false, fmt.Errorf("expected size: %v, actual: %v", 2, fi3.Size())
			}
		} else {
			return false, fmt.Errorf("error getting the file info: %v", err)
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

}

func TestFileRotator_PurgeOldFiles(t *testing.T) {
	defer goleak.VerifyNone(t)

	path := t.TempDir()

	fr, err := NewFileRotator(path, baseFileName, 2, 2, testlog.HCLogger(t))
	require.NoError(t, err)
	defer fr.Close()

	str := "abcdeghijklmn"
	nw, err := fr.Write([]byte(str))
	require.NoError(t, err)
	require.Equal(t, len(str), nw)

	testutil.WaitForResult(func() (bool, error) {
		f, err := os.ReadDir(path)
		if err != nil {
			return false, fmt.Errorf("failed to read dir %v: %w", path, err)
		}

		if len(f) != 2 {
			return false, fmt.Errorf("expected number of files: %v, got: %v %v", 2, len(f), f)
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
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
	path := b.TempDir()

	fr, err := NewFileRotator(path, baseFileName, 5, 1024*1024, testlog.HCLogger(b))
	require.NoError(b, err)
	defer fr.Close()

	b.ResetTimer()

	// run the Fib function b.N times
	for n := 0; n < b.N; n++ {
		// Generate some input
		data := make([]byte, size)
		_, err := rand.Read(data)
		require.NoError(b, err)

		// Insert random new lines
		for i := 0; i < 100; i++ {
			index := rand.Intn(size)
			data[index] = '\n'
		}

		// Write the data
		_, err = fr.Write(data)
		require.NoError(b, err)
	}
}
