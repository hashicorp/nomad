package driver

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type LogRotator struct {
	maxFiles   int
	fileSize   int64
	path       string
	fileName   string
	logFileIdx int
}

func NewLogRotor(path string, fileName string, maxFiles int, fileSize int64) (*LogRotator, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	logFileIdx := 1
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
	}, nil
}

func (l *LogRotator) Start(r io.Reader) error {
	for {
		logFileName := filepath.Join(l.path, fmt.Sprintf("%s.%d", l.fileName, l.logFileIdx))
		f, err := os.OpenFile(logFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 066)
		if err != nil {
			return err
		}
		remainingSize := l.fileSize
		if finfo, err := os.Stat(logFileName); err == nil {
			remainingSize = l.fileSize - finfo.Size()
		}
		if remainingSize < 1 {
			l.logFileIdx = l.logFileIdx + 1
			continue
		}
		if _, err := io.Copy(f, io.LimitReader(r, remainingSize)); err != nil {
			return err
		}
		l.logFileIdx = l.logFileIdx + 1
	}
	return nil
}
