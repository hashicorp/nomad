package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Object represents a  container data (S3)
type Object struct {
	LastModified string `json:"last_modified"`
	Name         string `json:"name"`
	Size         string `json:"size"`
	Public       bool   `json:"public"`
}

// Container represents a  container (S3)
type Container struct {
	Organization    `json:"organization"`
	Name            string `json:"name"`
	Size            int    `json:"size,string"`
	NumberOfObjects int    `json:"num_objects,string"`
	Public          bool   `json:"public"`
}

// CreateBucketRequest is used to create new buckets
type CreateBucketRequest struct {
	Name         string `json:"name"`
	Organization string `json:"organization"`
}

// ErrRegionNotYetSupported is returned to indicate that the selected region does not support buckets yet
var ErrRegionNotYetSupported = errors.New("Only the AMS1 region is supported at the moment")

// PutObjectRequest uploads an object into an bucket
type PutObjectRequest struct {
	BucketName string
	ObjectName string
}

// ListObjects lists all objects in a bucket
func (s *API) ListObjects(bucket string) ([]*Object, error) {
	vs := url.Values{}
	vs.Set("delimiter", "/")
	vs.Set("prefix", "")
	resp, err := s.GetResponsePaginate(s.objectstoreAPI, fmt.Sprintf("containers/%s", bucket), vs)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusOK}, resp)
	if err != nil {
		return nil, err
	}
	type listObjectsResponse struct {
		Containers []*Object `json:"container"`
	}
	var containers listObjectsResponse

	if err = json.Unmarshal(body, &containers); err != nil {
		return nil, err
	}
	return containers.Containers, nil
}

// GetObject fetches a single object from a bucket
func (s *API) GetObject(bucket, name string) (*Object, error) {
	resp, err := s.GetResponsePaginate(s.objectstoreAPI, fmt.Sprintf("containers/%s/%s", bucket, name), url.Values{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusOK}, resp)
	if err != nil {
		return nil, err
	}
	type getObjectResponse struct {
		Container *Object `json:"object"`
	}
	var containers getObjectResponse

	if err = json.Unmarshal(body, &containers); err != nil {
		return nil, err
	}
	return containers.Container, nil
}

// PutObject uploads a file into a bucket
func (s *API) PutObject(req *PutObjectRequest, r io.Reader) (*Object, error) {
	if s.objectstoreAPI == "" {
		return nil, ErrRegionNotYetSupported
	}
	resp, err := s.Upload(
		s.objectstoreAPI,
		fmt.Sprintf("containers/%s/upload/%s", req.BucketName, req.ObjectName),
		req.ObjectName, r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	_, err = s.handleHTTPError([]int{http.StatusNoContent}, resp)
	if err != nil {
		return nil, err
	}

	c, err := s.GetObject(req.BucketName, req.ObjectName)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// DeleteObject removes an object from a bucket on scaleway SIS
func (s *API) DeleteObject(bucket, name string) error {
	if s.objectstoreAPI == "" {
		return ErrRegionNotYetSupported
	}
	resp, err := s.DeleteResponse(s.objectstoreAPI, fmt.Sprintf("containers/%s/%s", bucket, name))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = s.handleHTTPError([]int{http.StatusNoContent}, resp)
	return err
}

// CreateBucket creates a new scaleway SIS bucket
func (s *API) CreateBucket(req *CreateBucketRequest) (*Container, error) {
	type createBucketResponse struct {
		Container `json:"container"`
	}
	if s.objectstoreAPI == "" {
		return nil, ErrRegionNotYetSupported
	}
	resp, err := s.PostResponse(s.objectstoreAPI, "containers", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusOK}, resp)
	if err != nil {
		return nil, err
	}

	var container createBucketResponse
	if err := json.Unmarshal(body, &container); err != nil {
		return nil, err
	}
	return &container.Container, nil
}

// DeleteBucket removes a bucket on scaleway SIS
func (s *API) DeleteBucket(name string) error {
	if s.objectstoreAPI == "" {
		return ErrRegionNotYetSupported
	}
	resp, err := s.DeleteResponse(s.objectstoreAPI, fmt.Sprintf("containers/%s", name))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = s.handleHTTPError([]int{http.StatusNoContent}, resp)
	return err
}

// GetContainers returns a GetContainers
func (s *API) GetContainers() ([]*Container, error) {
	resp, err := s.GetResponsePaginate(s.computeAPI, "containers", url.Values{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusOK}, resp)
	if err != nil {
		return nil, err
	}
	type getContainers struct {
		Containers []*Container `json:"containers"`
	}

	var containers getContainers

	if err = json.Unmarshal(body, &containers); err != nil {
		return nil, err
	}
	return containers.Containers, nil
}
