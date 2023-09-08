// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package logging

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
)

const (
	// logBufferSize is the size of the buffer.
	logBufferSize = 64 * 1024

	// bufferFlushDuration is the duration at which we flush the buffer.
	bufferFlushDuration = 100 * time.Millisecond

	// lineScanLimit is the number of bytes we will attempt to scan for new
	// lines when approaching the end of the file to avoid a log line being
	// split between two files. Any single line that is greater than this limit
	// may be split.
	lineScanLimit = 32 * 1024

	// newLineDelimiter is the delimiter used for new lines.
	newLineDelimiter = '\n'
)

// FileRotator writes bytes to a rotated set of files
type FileRotator struct {
	MaxFiles int   // MaxFiles is the maximum number of rotated files allowed in a path
	FileSize int64 // FileSize is the size a rotated file is allowed to grow

	path         string // path is the path on the file system where the rotated set of files are opened
	baseFileName string // baseFileName is the base file name of the rotated files
	logFileIdx   int    // logFileIdx is the current index of the rotated files

	oldestLogFileIdx int // oldestLogFileIdx is the index of the oldest log file in a path
	closed           bool
	fileLock         sync.Mutex

	currentFile *os.File // currentFile is the file that is currently getting written
	currentWr   int64    // currentWr is the number of bytes written to the current file
	bufw        *bufio.Writer
	bufLock     sync.Mutex

	flushTicker *time.Ticker
	logger      hclog.Logger
	purgeCh     chan struct{}
	doneCh      chan struct{}
}

// NewFileRotator returns a new file rotator
func NewFileRotator(path string, baseFile string, maxFiles int,
	fileSize int64, logger hclog.Logger) (*FileRotator, error) {
	logger = logger.Named("rotator")
	rotator := &FileRotator{
		MaxFiles: maxFiles,
		FileSize: fileSize,

		path:         path,
		baseFileName: baseFile,

		flushTicker: time.NewTicker(bufferFlushDuration),
		logger:      logger,
		purgeCh:     make(chan struct{}, 1),
		doneCh:      make(chan struct{}),
	}

	if err := rotator.lastFile(); err != nil {
		return nil, err
	}
	go rotator.purgeOldFiles()
	go rotator.flushPeriodically()
	return rotator, nil
}

// Write writes a byte array to a file and rotates the file if it's size becomes
// equal to the maximum size the user has defined.
func (f *FileRotator) Write(p []byte) (n int, err error) {
	n = 0
	var forceRotate bool

	for n < len(p) {
		// Check if we still have space in the current file, otherwise close and
		// open the next file
		if forceRotate || f.currentWr >= f.FileSize {
			forceRotate = false
			f.flushBuffer()
			f.currentFile.Close()
			if err := f.nextFile(); err != nil {
				f.logger.Error("error creating next file", "error", err)
				return 0, err
			}
		}
		// Calculate the remaining size on this file and how much we have left
		// to write
		remainingSpace := f.FileSize - f.currentWr
		remainingToWrite := int64(len(p[n:]))

		// Check if we are near the end of the file. If we are we attempt to
		// avoid a log line being split between two files.
		var nw int
		if (remainingSpace - lineScanLimit) < remainingToWrite {
			// Scan for new line and if the data up to new line fits in current
			// file, write to buffer
			idx := bytes.IndexByte(p[n:], newLineDelimiter)
			if idx >= 0 && (remainingSpace-int64(idx)-1) >= 0 {
				// We have space so write it to buffer
				nw, err = f.writeToBuffer(p[n : n+idx+1])
			} else if idx >= 0 {
				// We found a new line but don't have space so just force rotate
				forceRotate = true
			} else if remainingToWrite > f.FileSize || f.FileSize-lineScanLimit < 0 {
				// There is no new line remaining but there is no point in
				// rotating since the remaining data will not even fit in the
				// next file either so just fill this one up.
				li := int64(n) + remainingSpace
				if remainingSpace > remainingToWrite {
					li = int64(n) + remainingToWrite
				}
				nw, err = f.writeToBuffer(p[n:li])
			} else {
				// There is no new line in the data remaining for us to write
				// and it will fit in the next file so rotate.
				forceRotate = true
			}
		} else {
			// Write all the bytes in the current file
			nw, err = f.writeToBuffer(p[n:])
		}

		// Increment the number of bytes written so far in this method
		// invocation
		n += nw

		// Increment the total number of bytes in the file
		f.currentWr += int64(n)
		if err != nil {
			f.logger.Error("error writing to file", "error", err)

			// As bufio writer does not automatically recover in case of any
			// io error, we need to recover from it manually resetting the
			// writter.
			f.createOrResetBuffer()

			return
		}
	}
	return
}

// nextFile opens the next file and purges older files if the number of rotated
// files is larger than the maximum files configured by the user
func (f *FileRotator) nextFile() error {
	nextFileIdx := f.logFileIdx
	for {
		nextFileIdx += 1
		logFileName := filepath.Join(f.path, fmt.Sprintf("%s.%d", f.baseFileName, nextFileIdx))
		if fi, err := os.Stat(logFileName); err == nil {
			if fi.IsDir() || fi.Size() >= f.FileSize {
				continue
			}
		}
		f.logFileIdx = nextFileIdx
		if err := f.createFile(); err != nil {
			return err
		}
		break
	}
	// Purge old files if we have more files than MaxFiles
	f.fileLock.Lock()
	defer f.fileLock.Unlock()
	if f.logFileIdx-f.oldestLogFileIdx >= f.MaxFiles && !f.closed {
		select {
		case f.purgeCh <- struct{}{}:
		default:
		}
	}
	return nil
}

// lastFile finds out the rotated file with the largest index in a path.
func (f *FileRotator) lastFile() error {
	finfos, err := os.ReadDir(f.path)
	if err != nil {
		return err
	}

	prefix := fmt.Sprintf("%s.", f.baseFileName)
	for _, fi := range finfos {
		if fi.IsDir() {
			continue
		}
		if strings.HasPrefix(fi.Name(), prefix) {
			fileIdx := strings.TrimPrefix(fi.Name(), prefix)
			n, err := strconv.Atoi(fileIdx)
			if err != nil {
				continue
			}
			if n > f.logFileIdx {
				f.logFileIdx = n
			}
		}
	}
	if err := f.createFile(); err != nil {
		return err
	}
	return nil
}

// createFile opens a new or existing file for writing
func (f *FileRotator) createFile() error {
	logFileName := filepath.Join(f.path, fmt.Sprintf("%s.%d", f.baseFileName, f.logFileIdx))
	cFile, err := os.OpenFile(logFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	f.currentFile = cFile
	fi, err := f.currentFile.Stat()
	if err != nil {
		return err
	}
	f.currentWr = fi.Size()
	f.createOrResetBuffer()
	return nil
}

// flushPeriodically flushes the buffered writer every 100ms to the underlying
// file
func (f *FileRotator) flushPeriodically() {
	for {
		select {
		case <-f.flushTicker.C:
			f.flushBuffer()
		case <-f.doneCh:
			return
		}

	}
}

// Close flushes and closes the rotator. It never returns an error.
func (f *FileRotator) Close() error {
	f.fileLock.Lock()
	defer f.fileLock.Unlock()

	// Stop the ticker and flush for one last time
	f.flushTicker.Stop()
	f.flushBuffer()

	// Stop the go routines
	if !f.closed {
		close(f.doneCh)
		close(f.purgeCh)
		f.closed = true
		f.currentFile.Close()
	}

	return nil
}

// purgeOldFiles removes older files and keeps only the last N files rotated for
// a file
func (f *FileRotator) purgeOldFiles() {
	for {
		select {
		case <-f.purgeCh:
			var fIndexes []int
			files, err := os.ReadDir(f.path)
			if err != nil {
				f.logger.Error("error getting directory listing", "error", err)
				return
			}
			// Inserting all the rotated files in a slice
			for _, fi := range files {
				if strings.HasPrefix(fi.Name(), f.baseFileName) {
					fileIdx := strings.TrimPrefix(fi.Name(), fmt.Sprintf("%s.", f.baseFileName))
					n, err := strconv.Atoi(fileIdx)
					if err != nil {
						f.logger.Error("error extracting file index", "error", err)
						continue
					}
					fIndexes = append(fIndexes, n)
				}
			}

			// Not continuing to delete files if the number of files is not more
			// than MaxFiles
			if len(fIndexes) <= f.MaxFiles {
				continue
			}

			// Sorting the file indexes so that we can purge the older files and keep
			// only the number of files as configured by the user
			sort.Ints(fIndexes)
			toDelete := fIndexes[0 : len(fIndexes)-f.MaxFiles]
			for _, fIndex := range toDelete {
				fname := filepath.Join(f.path, fmt.Sprintf("%s.%d", f.baseFileName, fIndex))
				err := os.RemoveAll(fname)
				if err != nil {
					f.logger.Error("error removing file", "filename", fname, "error", err)
				}
			}

			f.fileLock.Lock()
			f.oldestLogFileIdx = fIndexes[0]
			f.fileLock.Unlock()
		case <-f.doneCh:
			return
		}
	}
}

// flushBuffer flushes the buffer
func (f *FileRotator) flushBuffer() error {
	f.bufLock.Lock()
	defer f.bufLock.Unlock()
	if f.bufw != nil {
		return f.bufw.Flush()
	}
	return nil
}

// writeToBuffer writes the byte array to buffer
func (f *FileRotator) writeToBuffer(p []byte) (int, error) {
	f.bufLock.Lock()
	defer f.bufLock.Unlock()
	return f.bufw.Write(p)
}

// createOrResetBuffer creates a new buffer if we don't have one otherwise
// resets the buffer
func (f *FileRotator) createOrResetBuffer() {
	f.bufLock.Lock()
	defer f.bufLock.Unlock()
	if f.bufw == nil {
		f.bufw = bufio.NewWriterSize(f.currentFile, logBufferSize)
	} else {
		f.bufw.Reset(f.currentFile)
	}
}
