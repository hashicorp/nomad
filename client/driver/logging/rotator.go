package logging

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

var (
	bufSize = 32
	buf     = make([]byte, bufSize)
)

type FileRotator struct {
	MaxFiles int
	FileSize int64

	path         string
	baseFileName string

	logFileIdx       int
	oldestLogFileIdx int

	currentFile *os.File
	currentWr   int64

	logger  *log.Logger
	purgeCh chan interface{}
}

func NewFileRotator(path string, baseFile string, maxFiles int, fileSize int64, logger *log.Logger) (*FileRotator, error) {
	rotator := &FileRotator{
		MaxFiles: maxFiles,
		FileSize: fileSize,

		path:         path,
		baseFileName: baseFile,

		logger:  logger,
		purgeCh: make(chan interface{}),
	}
	if err := rotator.lastFile(); err != nil {
		return nil, err
	}
	go rotator.purgeOldFiles()
	return rotator, nil
}

func (f *FileRotator) Write(p []byte) (n int, err error) {
	n = 0
	var nw int

	for n < len(p) {
		if f.currentWr >= f.FileSize {
			f.currentFile.Close()
			if err := f.nextFile(); err != nil {
				return 0, err
			}
		}
		remainingSize := f.FileSize - f.currentWr
		if remainingSize < int64(len(p[n:])) {
			li := int64(n) + remainingSize
			nw, err = f.currentFile.Write(p[n:li])
		} else {
			nw, err = f.currentFile.Write(p[n:])
		}
		n += nw
		f.currentWr += int64(n)
		if err != nil {
			return
		}
	}
	return
}

func (f *FileRotator) nextFile() error {
	nextFileIdx := f.logFileIdx
	for {
		nextFileIdx += 1
		logFileName := filepath.Join(f.path, fmt.Sprintf("%s.%d", f.baseFileName, nextFileIdx))
		if fi, err := os.Stat(logFileName); err == nil {
			if fi.IsDir() {
				nextFileIdx += 1
				continue
			}
			if fi.Size() >= f.FileSize {
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
		case f.purgeCh <- new(interface{}):
		default:
		}
	}
	return nil
}

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
	return nil
}

// PurgeOldFiles removes older files and keeps only the last N files rotated for
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
		}
	}
}
