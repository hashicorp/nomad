package logging

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	flushDur = 100 * time.Millisecond
)

// FileRotator writes bytes to a rotated set of files
type FileRotator struct {
	MaxFiles int   // MaxFiles is the maximum number of rotated files allowed in a path
	FileSize int64 // FileSize is the size a rotated file is allowed to grow

	path             string // path is the path on the file system where the rotated set of files are opened
	baseFileName     string // baseFileName is the base file name of the rotated files
	logFileIdx       int    // logFileIdx is the current index of the rotated files
	oldestLogFileIdx int    // oldestLogFileIdx is the index of the oldest log file in a path

	currentFile *os.File // currentFile is the file that is currently getting written
	currentWr   int64    // currentWr is the number of bytes written to the current file
	bufw        *bufio.Writer

	flushTicker *time.Ticker
	logger      *log.Logger
	purgeCh     chan struct{}
	doneCh      chan struct{}
}

// NewFileRotator returns a new file rotator
func NewFileRotator(path string, baseFile string, maxFiles int,
	fileSize int64, logger *log.Logger) (*FileRotator, error) {
	rotator := &FileRotator{
		MaxFiles: maxFiles,
		FileSize: fileSize,

		path:         path,
		baseFileName: baseFile,

		flushTicker: time.NewTicker(flushDur),
		logger:      logger,
		purgeCh:     make(chan struct{}, 1),
		doneCh:      make(chan struct{}, 1),
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
	var nw int

	for n < len(p) {
		// Check if we still have space in the current file, otherwise close and
		// open the next file
		if f.currentWr >= f.FileSize {
			f.bufw.Flush()
			f.currentFile.Close()
			if err := f.nextFile(); err != nil {
				return 0, err
			}
		}
		// Calculate the remaining size on this file
		remainingSize := f.FileSize - f.currentWr

		// Check if the number of bytes that we have to write is less than the
		// remaining size of the file
		if remainingSize < int64(len(p[n:])) {
			// Write the number of bytes that we can write on the current file
			li := int64(n) + remainingSize
			nw, err = f.bufw.Write(p[n:li])
			//nw, err = f.currentFile.Write(p[n:li])
		} else {
			// Write all the bytes in the current file
			nw, err = f.bufw.Write(p[n:])
			//nw, err = f.currentFile.Write(p[n:])
		}

		// Increment the number of bytes written so far in this method
		// invocation
		n += nw

		// Increment the total number of bytes in the file
		f.currentWr += int64(n)
		if err != nil {
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
	if f.logFileIdx-f.oldestLogFileIdx >= f.MaxFiles {
		select {
		case f.purgeCh <- struct{}{}:
		default:
		}
	}
	return nil
}

// lastFile finds out the rotated file with the largest index in a path.
func (f *FileRotator) lastFile() error {
	finfos, err := ioutil.ReadDir(f.path)
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
	cFile, err := os.OpenFile(logFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return err
	}
	f.currentFile = cFile
	fi, err := f.currentFile.Stat()
	if err != nil {
		return err
	}
	f.currentWr = fi.Size()
	if f.bufw == nil {
		f.bufw = bufio.NewWriter(f.currentFile)
	} else {
		f.bufw.Reset(f.currentFile)
	}
	return nil
}

// flushPeriodically flushes the buffered writer every 100ms to the underlying
// file
func (f *FileRotator) flushPeriodically() {
	for _ = range f.flushTicker.C {
		if f.bufw != nil {
			f.bufw.Flush()
		}
	}
}

func (f *FileRotator) Close() {
	// Stop the ticker and flush for one last time
	f.flushTicker.Stop()
	if f.bufw != nil {
		f.bufw.Flush()
	}

	// Stop the purge go routine
	f.doneCh <- struct{}{}
	close(f.purgeCh)
}

// purgeOldFiles removes older files and keeps only the last N files rotated for
// a file
func (f *FileRotator) purgeOldFiles() {
	for {
		select {
		case <-f.purgeCh:
			var fIndexes []int
			files, err := ioutil.ReadDir(f.path)
			if err != nil {
				return
			}
			// Inserting all the rotated files in a slice
			for _, fi := range files {
				if strings.HasPrefix(fi.Name(), f.baseFileName) {
					fileIdx := strings.TrimPrefix(fi.Name(), fmt.Sprintf("%s.", f.baseFileName))
					n, err := strconv.Atoi(fileIdx)
					if err != nil {
						continue
					}
					fIndexes = append(fIndexes, n)
				}
			}

			// Sorting the file indexes so that we can purge the older files and keep
			// only the number of files as configured by the user
			sort.Sort(sort.IntSlice(fIndexes))
			var toDelete []int
			toDelete = fIndexes[0 : len(fIndexes)-f.MaxFiles]
			for _, fIndex := range toDelete {
				fname := filepath.Join(f.path, fmt.Sprintf("%s.%d", f.baseFileName, fIndex))
				os.RemoveAll(fname)
			}
			f.oldestLogFileIdx = fIndexes[0]
		case <-f.doneCh:
			return
		}
	}
}
