package driver

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"sync"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	// createCoordinator allows us to only create a single coordinator
	createCoordinator sync.Once

	// globalCoordinator is the shared coordinator and should only be retreived
	// using the GetDockerCoordinator() method.
	globalCoordinator *dockerCoordinator

	// imageNotFoundMatcher is a regex expression that matches the image not
	// found error Docker returns.
	imageNotFoundMatcher = regexp.MustCompile(`Error: image .+ not found`)
)

// pullFuture is a sharable future for retrieving a pulled images ID and any
// error that may have occurred during the pull.
type pullFuture struct {
	waitCh chan struct{}

	err     error
	imageID string
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
func (p *pullFuture) result() (imageID string, err error) {
	return p.imageID, p.err
}

// set is used to set the results and unblock any waiter. This may only be
// called once.
func (p *pullFuture) set(imageID string, err error) {
	p.imageID = imageID
	p.err = err
	close(p.waitCh)
}

// DockerImageClient provides the methods required to do CRUD operations on the
// Docker images
type DockerImageClient interface {
	PullImage(opts docker.PullImageOptions, auth docker.AuthConfiguration) error
	InspectImage(id string) (*docker.Image, error)
	RemoveImage(id string) error
}

// dockerCoordinatorConfig is used to configure the Docker coordinator.
type dockerCoordinatorConfig struct {
	// logger is the logger the coordinator should use
	logger *log.Logger

	// cleanup marks whether images should be deleting when the reference count
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

	// imageRefCount is the reference count of image IDs
	imageRefCount map[string]map[string]struct{}

	// deleteFuture is indexed by image ID and has a cancable delete future
	deleteFuture map[string]context.CancelFunc
}

// NewDockerCoordinator returns a new Docker coordinator
func NewDockerCoordinator(config *dockerCoordinatorConfig) *dockerCoordinator {
	if config.client == nil {
		return nil
	}

	return &dockerCoordinator{
		dockerCoordinatorConfig: config,
		pullFutures:             make(map[string]*pullFuture),
		imageRefCount:           make(map[string]map[string]struct{}),
		deleteFuture:            make(map[string]context.CancelFunc),
	}
}

// GetDockerCoordinator returns the shared dockerCoordinator instance
func GetDockerCoordinator(config *dockerCoordinatorConfig) *dockerCoordinator {
	createCoordinator.Do(func() {
		globalCoordinator = NewDockerCoordinator(config)
	})

	return globalCoordinator
}

// PullImage is used to pull an image. It returns the pulled imaged ID or an
// error that occurred during the pull
func (d *dockerCoordinator) PullImage(image string, authOptions *docker.AuthConfiguration, callerID string) (imageID string, err error) {
	// Get the future
	d.imageLock.Lock()
	future, ok := d.pullFutures[image]
	if !ok {
		// Make the future
		future = newPullFuture()
		d.pullFutures[image] = future
		go d.pullImageImpl(image, authOptions, future)
	}
	d.imageLock.Unlock()

	// We unlock while we wait since this can take a while
	id, err := future.wait().result()

	d.imageLock.Lock()
	defer d.imageLock.Unlock()

	// Delete the future since we don't need it and we don't want to cache an
	// image being there if it has possibly been manually deleted (outside of
	// Nomad).
	if _, ok := d.pullFutures[image]; ok {
		delete(d.pullFutures, image)
	}

	// If we are cleaning up, we increment the reference count on the image
	if err == nil && d.cleanup {
		d.incrementImageReferenceImpl(id, image, callerID)
	}

	return id, err
}

// pullImageImpl is the implementation of pulling an image. The results are
// returned via the passed future
func (d *dockerCoordinator) pullImageImpl(image string, authOptions *docker.AuthConfiguration, future *pullFuture) {
	// Parse the repo and tag
	repo, tag := docker.ParseRepositoryTag(image)
	if tag == "" {
		tag = "latest"
	}
	pullOptions := docker.PullImageOptions{
		Repository: repo,
		Tag:        tag,
	}

	// Attempt to pull the image
	var auth docker.AuthConfiguration
	if authOptions != nil {
		auth = *authOptions
	}
	err := d.client.PullImage(pullOptions, auth)
	if err != nil {
		d.logger.Printf("[ERR] driver.docker: failed pulling container %s:%s: %s", repo, tag, err)
		future.set("", recoverablePullError(err, image))
		return
	}

	d.logger.Printf("[DEBUG] driver.docker: docker pull %s:%s succeeded", repo, tag)

	dockerImage, err := d.client.InspectImage(image)
	if err != nil {
		d.logger.Printf("[ERR] driver.docker: failed getting image id for %q: %v", image, err)
		future.set("", recoverableErrTimeouts(err))
		return
	}

	future.set(dockerImage.ID, nil)
	return
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
		d.logger.Printf("[DEBUG] driver.docker: cancelling removal of image %q", imageName)
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
		d.logger.Printf("[DEBUG] driver.docker: image %q (%v) reference count incremented: %d", imageName, imageID, len(references))
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
		d.logger.Printf("[WARN] driver.docker: RemoveImage on non-referenced counted image id %q", imageID)
		return
	}

	// Decrement the reference count
	delete(references, callerID)
	count := len(references)
	d.logger.Printf("[DEBUG] driver.docker: image id %q reference count decremented: %d", imageID, count)

	// Nothing to do
	if count != 0 {
		return
	}

	// This should never be the case but we safefty guard so we don't leak a
	// cancel.
	if cancel, ok := d.deleteFuture[imageID]; ok {
		d.logger.Printf("[ERR] driver.docker: image id %q has lingering delete future", imageID)
		cancel()
	}

	// Setup a future to delete the image
	ctx, cancel := context.WithCancel(context.Background())
	d.deleteFuture[imageID] = cancel
	go d.removeImageImpl(imageID, ctx)

	// Delete the key from the reference count
	delete(d.imageRefCount, imageID)
}

// removeImageImpl is used to remove an image. It wil wait the specified remove
// delay to remove the image. If the context is cancalled before that the image
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
		err := d.client.RemoveImage(id)
		if err == nil {
			break
		}

		if err == docker.ErrNoSuchImage {
			d.logger.Printf("[DEBUG] driver.docker: unable to cleanup image %q: does not exist", id)
			return
		}
		if derr, ok := err.(*docker.Error); ok && derr.Status == 409 {
			d.logger.Printf("[DEBUG] driver.docker: unable to cleanup image %q: still in use", id)
			return
		}

		// Retry on unknown errors
		d.logger.Printf("[DEBUG] driver.docker: failed to remove image %q (attempt %d): %v", id, i+1, err)

		select {
		case <-ctx.Done():
			// We have been cancelled
			return
		case <-time.After(3 * time.Second):
		}
	}

	d.logger.Printf("[DEBUG] driver.docker: cleanup removed downloaded image: %q", id)

	// Cleanup the future from the map and free the context by cancelling it
	d.imageLock.Lock()
	if cancel, ok := d.deleteFuture[id]; ok {
		delete(d.deleteFuture, id)
		cancel()
	}
	d.imageLock.Unlock()
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
