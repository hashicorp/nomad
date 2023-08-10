// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/pkg/jsonmessage"
	units "github.com/docker/go-units"
)

const (
	// dockerImageProgressReportInterval is the default value set in the
	// imageProgressManager when newImageProgressManager is called
	dockerImageProgressReportInterval = 10 * time.Second

	// dockerImageSlowProgressReportInterval is the default value set in the
	// imageProgressManager when newImageProgressManager is called
	dockerImageSlowProgressReportInterval = 2 * time.Minute
)

// layerProgress tracks the state and downloaded bytes of a single layer within
// a docker image
type layerProgress struct {
	id           string
	status       layerProgressStatus
	currentBytes int64
	totalBytes   int64
}

type layerProgressStatus int

const (
	layerProgressStatusUnknown layerProgressStatus = iota
	layerProgressStatusStarting
	layerProgressStatusWaiting
	layerProgressStatusDownloading
	layerProgressStatusVerifying
	layerProgressStatusDownloaded
	layerProgressStatusExtracting
	layerProgressStatusComplete
	layerProgressStatusExists
)

func lpsFromString(status string) layerProgressStatus {
	switch status {
	case "Pulling fs layer":
		return layerProgressStatusStarting
	case "Waiting":
		return layerProgressStatusWaiting
	case "Downloading":
		return layerProgressStatusDownloading
	case "Verifying Checksum":
		return layerProgressStatusVerifying
	case "Download complete":
		return layerProgressStatusDownloaded
	case "Extracting":
		return layerProgressStatusExtracting
	case "Pull complete":
		return layerProgressStatusComplete
	case "Already exists":
		return layerProgressStatusExists
	default:
		return layerProgressStatusUnknown
	}
}

// imageProgress tracks the status of each child layer as its pulled from a
// docker image repo
type imageProgress struct {
	sync.RWMutex
	lastMessage *jsonmessage.JSONMessage
	timestamp   time.Time
	layers      map[string]*layerProgress
	pullStart   time.Time
}

// get returns a status message and the timestamp of the last status update
func (p *imageProgress) get() (string, time.Time) {
	p.RLock()
	defer p.RUnlock()

	if p.lastMessage == nil {
		return "No progress", p.timestamp
	}

	var pulled, pulling, waiting int
	for _, l := range p.layers {
		switch {
		case l.status == layerProgressStatusStarting ||
			l.status == layerProgressStatusWaiting:
			waiting++
		case l.status == layerProgressStatusDownloading ||
			l.status == layerProgressStatusVerifying:
			pulling++
		case l.status >= layerProgressStatusDownloaded:
			pulled++
		}
	}

	elapsed := time.Since(p.pullStart)
	cur := p.currentBytes()
	total := p.totalBytes()
	var est int64
	if cur != 0 {
		est = (elapsed.Nanoseconds() / cur * total) - elapsed.Nanoseconds()
	}

	var msg strings.Builder
	fmt.Fprintf(&msg, "Pulled %d/%d (%s/%s) layers: %d waiting/%d pulling",
		pulled, len(p.layers), units.BytesSize(float64(cur)), units.BytesSize(float64(total)),
		waiting, pulling)

	if est > 0 {
		fmt.Fprintf(&msg, " - est %.1fs remaining", time.Duration(est).Seconds())
	}
	return msg.String(), p.timestamp
}

// set takes a status message received from the docker engine api during an image
// pull and updates the status of the corresponding layer
func (p *imageProgress) set(msg *jsonmessage.JSONMessage) {
	p.Lock()
	defer p.Unlock()

	p.lastMessage = msg
	p.timestamp = time.Now()

	lps := lpsFromString(msg.Status)
	if lps == layerProgressStatusUnknown {
		return
	}

	layer, ok := p.layers[msg.ID]
	if !ok {
		layer = &layerProgress{id: msg.ID}
		p.layers[msg.ID] = layer
	}
	layer.status = lps
	if msg.Progress != nil && lps == layerProgressStatusDownloading {
		layer.currentBytes = msg.Progress.Current
		layer.totalBytes = msg.Progress.Total
	} else if lps == layerProgressStatusDownloaded {
		layer.currentBytes = layer.totalBytes
	}
}

// currentBytes iterates through all image layers and sums the total of
// current bytes. The caller is responsible for acquiring a read lock on the
// imageProgress struct
func (p *imageProgress) currentBytes() int64 {
	var b int64
	for _, l := range p.layers {
		b += l.currentBytes
	}
	return b
}

// totalBytes iterates through all image layers and sums the total of
// total bytes. The caller is responsible for acquiring a read lock on the
// imageProgress struct
func (p *imageProgress) totalBytes() int64 {
	var b int64
	for _, l := range p.layers {
		b += l.totalBytes
	}
	return b
}

// progressReporterFunc defines the method for handling inactivity and report
// events from the imageProgressManager. The image name, current status message
// and timestamp of last received status update are passed in.
type progressReporterFunc func(image string, msg string, timestamp time.Time)

// imageProgressManager tracks the progress of pulling a docker image from an
// image repository.
// It also implemented the io.Writer interface so as to be passed to the docker
// client pull image method in order to receive status updates from the docker
// engine api.
type imageProgressManager struct {
	imageProgress      *imageProgress
	image              string
	activityDeadline   time.Duration
	inactivityFunc     progressReporterFunc
	reportInterval     time.Duration
	reporter           progressReporterFunc
	slowReportInterval time.Duration
	slowReporter       progressReporterFunc
	lastSlowReport     time.Time
	cancel             context.CancelFunc
	stopCh             chan struct{}
	buf                bytes.Buffer
}

func newImageProgressManager(
	image string, cancel context.CancelFunc,
	pullActivityTimeout time.Duration, inactivityFunc, reporter, slowReporter progressReporterFunc) *imageProgressManager {

	pm := &imageProgressManager{
		image:              image,
		activityDeadline:   pullActivityTimeout,
		inactivityFunc:     inactivityFunc,
		reportInterval:     dockerImageProgressReportInterval,
		reporter:           reporter,
		slowReportInterval: dockerImageSlowProgressReportInterval,
		slowReporter:       slowReporter,
		imageProgress: &imageProgress{
			timestamp: time.Now(),
			layers:    make(map[string]*layerProgress),
		},
		cancel: cancel,
		stopCh: make(chan struct{}),
	}

	pm.start()
	return pm
}

// start intiates the ticker to trigger the inactivity and reporter handlers
func (pm *imageProgressManager) start() {
	now := time.Now()
	pm.imageProgress.pullStart = now
	pm.lastSlowReport = now
	go func() {
		ticker := time.NewTicker(dockerImageProgressReportInterval)
		for {
			select {
			case <-ticker.C:
				msg, lastStatusTime := pm.imageProgress.get()
				t := time.Now()
				if t.Sub(lastStatusTime) > pm.activityDeadline {
					pm.inactivityFunc(pm.image, msg, lastStatusTime)
					pm.cancel()
					return
				}
				if t.Sub(pm.lastSlowReport) > pm.slowReportInterval {
					pm.slowReporter(pm.image, msg, lastStatusTime)
					pm.lastSlowReport = t
				}
				pm.reporter(pm.image, msg, lastStatusTime)
			case <-pm.stopCh:
				return
			}
		}
	}()
}

func (pm *imageProgressManager) stop() {
	close(pm.stopCh)
}

func (pm *imageProgressManager) Write(p []byte) (n int, err error) {
	n, err = pm.buf.Write(p)
	var msg jsonmessage.JSONMessage

	for {
		line, err := pm.buf.ReadBytes('\n')
		if err == io.EOF {
			// Partial write of line; push back onto buffer and break until full line
			pm.buf.Write(line)
			break
		}
		if err != nil {
			return n, err
		}
		err = json.Unmarshal(line, &msg)
		if err != nil {
			return n, err
		}

		if msg.Error != nil {
			// error received from the docker engine api
			return n, msg.Error
		}

		pm.imageProgress.set(&msg)
	}

	return
}
