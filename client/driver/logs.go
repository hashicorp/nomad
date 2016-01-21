package driver

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	bufSize = 32 * 1024 // Max number of bytes read from a buffer
)

// LogRotator ingests data and writes out to a rotated set of files
type LogRotator struct {
	maxFiles int    // maximum number of rotated files retained by the log rotator
	fileSize int64  // maximum file size of a rotated file
	path     string // path where the rotated files are created
	fileName string // base file name of the rotated files

	logFileIdx int // index to the current file

	logger *log.Logger
}

// NewLogRotator configures and returns a new LogRotator
func NewLogRotator(path string, fileName string, maxFiles int, fileSize int64, logger *log.Logger) (*LogRotator, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	// finding out the log file with the largest index
	logFileIdx := 0
	for _, f := range files {
		if strings.HasPrefix(f.Name(), fileName) {
			fileIdx := strings.TrimPrefix(f.Name(), fmt.Sprintf("%s.", fileName))
			n, err := strconv.Atoi(fileIdx)
			if err != nil {
				continue
			}
			if n > logFileIdx {
				logFileIdx = n
			}
		}
	}

	return &LogRotator{
		maxFiles:   maxFiles,
		fileSize:   fileSize,
		path:       path,
		fileName:   fileName,
		logFileIdx: logFileIdx,
		logger:     logger,
	}, nil
}

// Start reads from a Reader and writes them to files and rotates them when the
// size of the file becomes equal to the max size configured
func (l *LogRotator) Start(r io.Reader) error {
	buf := make([]byte, bufSize)
	for {
		logFileName := filepath.Join(l.path, fmt.Sprintf("%s.%d", l.fileName, l.logFileIdx))
		remainingSize := l.fileSize
		if f, err := os.Stat(logFileName); err == nil {
			// skipping the current file if it happens to be a directory
			if f.IsDir() {
				l.logFileIdx += 1
				continue
			}
			// calculating the remaining capacity of the log file
			remainingSize = l.fileSize - f.Size()
		}
		f, err := os.OpenFile(logFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return err
		}
		l.logger.Printf("[DEBUG] client.logrotator: opened a new file: %s", logFileName)

		// closing the current log file if it doesn't have any more capacity
		if remainingSize <= 0 {
			l.logFileIdx = l.logFileIdx + 1
			f.Close()
			continue
		}

		// reading from the reader and writing into the current log file as long
		// as it has capacity or the reader closes
		for {
			var nr int
			var err error
			if remainingSize < bufSize {
				nr, err = r.Read(buf[0:remainingSize])
			} else {
				nr, err = r.Read(buf)
			}
			if err != nil {
				f.Close()
				return err
			}
			nw, err := f.Write(buf[:nr])
			if err != nil {
				f.Close()
				return err
			}
			if nr != nw {
				f.Close()
				return fmt.Errorf("failed to write data read from the reader into file, R: %d W: %d", nr, nw)
			}
			remainingSize -= int64(nr)
			if remainingSize < 1 {
				f.Close()
				break
			}
		}
		l.logFileIdx = l.logFileIdx + 1
	}
	return nil
}

// PurgeOldFiles removes older files and keeps only the last N files rotated for
// a file
func (l *LogRotator) PurgeOldFiles() {
	var fIndexes []int
	files, err := ioutil.ReadDir(l.path)
	if err != nil {
		return
	}
	// Inserting all the rotated files in a slice
	for _, f := range files {
		if strings.HasPrefix(f.Name(), l.fileName) {
			fileIdx := strings.TrimPrefix(f.Name(), fmt.Sprintf("%s.", l.fileName))
			n, err := strconv.Atoi(fileIdx)
			if err != nil {
				continue
			}
			fIndexes = append(fIndexes, n)
		}
	}

	// sorting the file indexes so that we can purge the older files and keep
	// only the number of files as configured by the user
	sort.Sort(sort.IntSlice(fIndexes))
	toDelete := fIndexes[l.maxFiles-1 : len(fIndexes)-1]
	for _, fIndex := range toDelete {
		fname := filepath.Join(l.path, fmt.Sprintf("%s.%d", l.fileName, fIndex))
		os.RemoveAll(fname)
	}
}
