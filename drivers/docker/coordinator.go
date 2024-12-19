// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/errdefs"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	// imageNotFoundMatcher is a regex expression that matches the image not
	// found error Docker returns.
	imageNotFoundMatcher = regexp.MustCompile(`Error: image .+ not found`)
)

// pullFuture is a sharable future for retrieving a pulled images ID and user,
// and any error that may have occurred during the pull.
type pullFuture struct {
	waitCh chan struct{}

	err       error
	imageID   string
	imageUser string
}

// newPullFuture returns a new pull future
func newPullFuture() *pullFuture {
	return &pullFuture{
		waitCh: make(chan struct{}),
	}
}

// wait waits till the future has a result
func (p *pullFuture) wait() *pullFuture {
	<-p.waitCh
	return p
}

// result returns the results of the future and should only ever be called after
// wait returns.
func (p *pullFuture) result() (imageID, imageUser string, err error) {
	return p.imageID, p.imageUser, p.err
}

// set is used to set the results and unblock any waiter. This may only be
// called once.
func (p *pullFuture) set(imageID, imageUser string, err error) {
	p.imageID = imageID
	p.imageUser = imageUser
	p.err = err
	close(p.waitCh)
}

// DockerImageClient provides the methods required to do CRUD operations on the
// Docker images
type DockerImageClient interface {
	ImagePull(ctx context.Context, refStr string, opts image.PullOptions) (io.ReadCloser, error)
	ImageInspectWithRaw(ctx context.Context, id string) (types.ImageInspect, []byte, error)
	ImageRemove(ctx context.Context, id string, opts image.RemoveOptions) ([]image.DeleteResponse, error)
}

// LogEventFn is a callback which allows Drivers to emit task events.
type LogEventFn func(message string, annotations map[string]string)

// noopLogEventFn satisfies the LogEventFn type but noops when called
func noopLogEventFn(string, map[string]string) {}

// dockerCoordinatorConfig is used to configure the Docker coordinator.
type dockerCoordinatorConfig struct {
	ctx context.Context

	// logger is the logger the coordinator should use
	logger hclog.Logger

	// cleanup marks whether images should be deleted when the reference count
	// is zero
	cleanup bool

	// client is the Docker client to use for communicating with Docker
	client DockerImageClient

	// removeDelay is the delay between an image's reference count going to
	// zero and the image actually being deleted.
	removeDelay time.Duration
}

// dockerCoordinator is used to coordinate actions against images to prevent
// racy deletions. It can be thought of as a reference counter on images.
type dockerCoordinator struct {
	*dockerCoordinatorConfig

	// imageLock is used to lock access to all images
	imageLock sync.Mutex

	// pullFutures is used to allow multiple callers to pull the same image but
	// only have one request be sent to Docker
	pullFutures map[string]*pullFuture

	// pullLoggers is used to track the LogEventFn for each alloc pulling an image.
	// If multiple alloc's are attempting to pull the same image, each will need
	// to register its own LogEventFn with the coordinator.
	pullLoggers map[string][]LogEventFn

	// pullLoggerLock is used to sync access to the pullLoggers map
	pullLoggerLock sync.RWMutex

	// imageRefCount is the reference count of image IDs
	imageRefCount map[string]map[string]struct{}

	// deleteFuture is indexed by image ID and has a cancelable delete future
	deleteFuture map[string]context.CancelFunc
}

// newDockerCoordinator returns a new Docker coordinator
func newDockerCoordinator(config *dockerCoordinatorConfig) *dockerCoordinator {
	if config.client == nil {
		return nil
	}

	return &dockerCoordinator{
		dockerCoordinatorConfig: config,
		pullFutures:             make(map[string]*pullFuture),
		pullLoggers:             make(map[string][]LogEventFn),
		imageRefCount:           make(map[string]map[string]struct{}),
		deleteFuture:            make(map[string]context.CancelFunc),
	}
}

// PullImage is used to pull an image. It returns the pulled imaged ID or an
// error that occurred during the pull
func (d *dockerCoordinator) PullImage(image string, authOptions *registry.AuthConfig, callerID string,
	emitFn LogEventFn, pullTimeout, pullActivityTimeout time.Duration) (imageID, imageUser string, err error) {
	// Get the future
	d.imageLock.Lock()
	future, ok := d.pullFutures[image]
	d.registerPullLogger(image, emitFn)
	if !ok {
		// Make the future
		future = newPullFuture()
		d.pullFutures[image] = future
		go d.pullImageImpl(image, authOptions, pullTimeout, pullActivityTimeout, future)
	}
	d.imageLock.Unlock()

	// We unlock while we wait since this can take a while
	id, user, err := future.wait().result()

	d.imageLock.Lock()
	defer d.imageLock.Unlock()

	// Delete the future since we don't need it and we don't want to cache an
	// image being there if it has possibly been manually deleted (outside of
	// Nomad).
	delete(d.pullFutures, image)

	// If we are cleaning up, we increment the reference count on the image
	if err == nil && d.cleanup {
		d.incrementImageReferenceImpl(id, image, callerID)
	}

	return id, user, err
}

// pullImageImpl is the implementation of pulling an image. The results are
// returned via the passed future
func (d *dockerCoordinator) pullImageImpl(imageID string, authOptions *registry.AuthConfig,
	pullTimeout, pullActivityTimeout time.Duration, future *pullFuture) {

	defer d.clearPullLogger(imageID)
	// Parse the repo and tag
	repo, tag := parseDockerImage(imageID)
	ctx, cancel := context.WithTimeout(context.Background(), pullTimeout)
	defer cancel()

	pm := newImageProgressManager(imageID, cancel, pullActivityTimeout, d.handlePullInactivity,
		d.handlePullProgressReport, d.handleSlowPullProgressReport)
	defer pm.stop()

	// Attempt to pull the image
	var auth registry.AuthConfig
	if authOptions != nil {
		auth = *authOptions
	}

	pullOptions := image.PullOptions{RegistryAuth: auth.Auth}
	reader, err := d.client.ImagePull(d.ctx, dockerImageRef(repo, tag), pullOptions)

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		d.logger.Error("timeout pulling container", "image_ref", dockerImageRef(repo, tag))
		future.set("", "", recoverablePullError(ctx.Err(), imageID))
		return
	}

	if err != nil {
		d.logger.Error("failed pulling container", "image_ref", dockerImageRef(repo, tag),
			"error", err)
		future.set("", "", recoverablePullError(err, imageID))
		return
	}

	if reader != nil {
		defer reader.Close()
		_, err = io.Copy(pm, reader)
		if err != nil && !errors.Is(err, io.EOF) {
			d.logger.Error("error reading image pull progress", "error", err)
			return
		}
	}

	d.logger.Debug("docker pull succeeded", "image_ref", dockerImageRef(repo, tag))

	dockerImage, _, err := d.client.ImageInspectWithRaw(d.ctx, imageID)
	if err != nil {
		d.logger.Error("failed getting image id", "image_name", imageID, "error", err)
		future.set("", "", recoverableErrTimeouts(err))
		return
	}

	var imageUser string
	if dockerImage.Config != nil {
		imageUser = dockerImage.Config.User
	}

	future.set(dockerImage.ID, imageUser, err)
}

// IncrementImageReference is used to increment an image reference count
func (d *dockerCoordinator) IncrementImageReference(imageID, imageName, callerID string) {
	d.imageLock.Lock()
	defer d.imageLock.Unlock()
	if d.cleanup {
		d.incrementImageReferenceImpl(imageID, imageName, callerID)
	}
}

// incrementImageReferenceImpl assumes the lock is held
func (d *dockerCoordinator) incrementImageReferenceImpl(imageID, imageName, callerID string) {
	// Cancel any pending delete
	if cancel, ok := d.deleteFuture[imageID]; ok {
		d.logger.Debug("cancelling removal of container image", "image_name", imageName)
		cancel()
		delete(d.deleteFuture, imageID)
	}

	// Increment the reference
	references, ok := d.imageRefCount[imageID]
	if !ok {
		references = make(map[string]struct{})
		d.imageRefCount[imageID] = references
	}

	if _, ok := references[callerID]; !ok {
		references[callerID] = struct{}{}
		d.logger.Debug("image reference count incremented", "image_name", imageName, "image_id", imageID, "references", len(references))
	}
}

// RemoveImage removes the given image. If there are any errors removing the
// image, the remove is retried internally.
func (d *dockerCoordinator) RemoveImage(imageID, callerID string) {
	d.imageLock.Lock()
	defer d.imageLock.Unlock()

	if !d.cleanup {
		return
	}

	references, ok := d.imageRefCount[imageID]
	if !ok {
		d.logger.Warn("RemoveImage on non-referenced counted image id", "image_id", imageID)
		return
	}

	// Decrement the reference count
	delete(references, callerID)
	count := len(references)
	d.logger.Debug("image id reference count decremented", "image_id", imageID, "references", count)

	// Nothing to do
	if count != 0 {
		return
	}

	// This should never be the case but we safety guard so we don't leak a
	// cancel.
	if cancel, ok := d.deleteFuture[imageID]; ok {
		d.logger.Error("image id has lingering delete future", "image_id", imageID)
		cancel()
	}

	// Setup a future to delete the image
	ctx, cancel := context.WithCancel(d.ctx)
	d.deleteFuture[imageID] = cancel
	go d.removeImageImpl(imageID, ctx)

	// Delete the key from the reference count
	delete(d.imageRefCount, imageID)
}

// removeImageImpl is used to remove an image. It wil wait the specified remove
// delay to remove the image. If the context is cancelled before that the image
// removal will be cancelled.
func (d *dockerCoordinator) removeImageImpl(id string, ctx context.Context) {
	// Wait for the delay or a cancellation event
	select {
	case <-ctx.Done():
		// We have been cancelled
		return
	case <-time.After(d.removeDelay):
	}

	// Ensure we are suppose to delete. Do a short check while holding the lock
	// so there can't be interleaving. There is still the smallest chance that
	// the delete occurs after the image has been pulled but before it has been
	// incremented. For handling that we just treat it as a recoverable error in
	// the docker driver.
	d.imageLock.Lock()
	select {
	case <-ctx.Done():
		d.imageLock.Unlock()
		return
	default:
	}
	d.imageLock.Unlock()

	for i := 0; i < 3; i++ {
		_, err := d.client.ImageRemove(d.ctx, id, image.RemoveOptions{
			Force: true, // necessary to GC images referenced by multiple tags
		})
		if err == nil {
			break
		}

		if errdefs.IsNotFound(err) {
			d.logger.Debug("unable to cleanup image, does not exist", "image_id", id)
			return
		}
		if errdefs.IsConflict(err) {
			d.logger.Debug("unable to cleanup image, still in use", "image_id", id)
			return
		}

		// Retry on unknown errors
		d.logger.Debug("failed to remove image", "image_id", id, "attempt", i+1, "error", err)

		select {
		case <-ctx.Done():
			// We have been cancelled
			return
		case <-time.After(3 * time.Second):
		}
	}

	d.logger.Debug("cleanup removed downloaded image", "image_id", id)

	// Cleanup the future from the map and free the context by cancelling it
	d.imageLock.Lock()
	if cancel, ok := d.deleteFuture[id]; ok {
		delete(d.deleteFuture, id)
		cancel()
	}
	d.imageLock.Unlock()
}

func (d *dockerCoordinator) registerPullLogger(image string, logger LogEventFn) {
	d.pullLoggerLock.Lock()
	defer d.pullLoggerLock.Unlock()
	if _, ok := d.pullLoggers[image]; !ok {
		d.pullLoggers[image] = []LogEventFn{}
	}
	d.pullLoggers[image] = append(d.pullLoggers[image], logger)
}

func (d *dockerCoordinator) clearPullLogger(image string) {
	d.pullLoggerLock.Lock()
	defer d.pullLoggerLock.Unlock()
	delete(d.pullLoggers, image)
}

func (d *dockerCoordinator) emitEvent(image, message string, annotations map[string]string) {
	d.pullLoggerLock.RLock()
	defer d.pullLoggerLock.RUnlock()
	for i := range d.pullLoggers[image] {
		go d.pullLoggers[image][i](message, annotations)
	}
}

func (d *dockerCoordinator) handlePullInactivity(image, msg string, timestamp time.Time) {
	d.logger.Error("image pull aborted due to inactivity", "image_name", image,
		"last_event_timestamp", timestamp.String(), "last_event", msg)
}

func (d *dockerCoordinator) handlePullProgressReport(image, msg string, _ time.Time) {
	d.logger.Debug("image pull progress", "image_name", image, "message", msg)
}

func (d *dockerCoordinator) handleSlowPullProgressReport(image, msg string, _ time.Time) {
	d.emitEvent(image, fmt.Sprintf("Docker image pull progress: %s", msg), map[string]string{
		"image": image,
	})
}

// recoverablePullError wraps the error gotten when trying to pull and image if
// the error is recoverable.
func recoverablePullError(err error, image string) error {
	recoverable := true
	if imageNotFoundMatcher.MatchString(err.Error()) {
		recoverable = false
	}
	return structs.NewRecoverableError(fmt.Errorf("Failed to pull `%s`: %s", image, err), recoverable)
}
