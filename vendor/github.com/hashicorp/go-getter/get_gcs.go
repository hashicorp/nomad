package getter

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

// GCSGetter is a Getter implementation that will download a module from
// a GCS bucket.
type GCSGetter struct {
	getter
}

func (g *GCSGetter) ClientMode(u *url.URL) (ClientMode, error) {
	ctx := g.Context()

	// Parse URL
	bucket, object, _, err := g.parseURL(u)
	if err != nil {
		return 0, err
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		return 0, err
	}
	iter := client.Bucket(bucket).Objects(ctx, &storage.Query{Prefix: object})
	for {
		obj, err := iter.Next()
		if err != nil && err != iterator.Done {
			return 0, err
		}

		if err == iterator.Done {
			break
		}
		if strings.HasSuffix(obj.Name, "/") {
			// A directory matched the prefix search, so this must be a directory
			return ClientModeDir, nil
		} else if obj.Name != object {
			// A file matched the prefix search and doesn't have the same name
			// as the query, so this must be a directory
			return ClientModeDir, nil
		}
	}
	// There are no directories or subdirectories, and if a match was returned,
	// it was exactly equal to the prefix search. So return File mode
	return ClientModeFile, nil
}

func (g *GCSGetter) Get(dst string, u *url.URL) error {
	ctx := g.Context()

	// Parse URL
	bucket, object, _, err := g.parseURL(u)
	if err != nil {
		return err
	}

	// Remove destination if it already exists
	_, err = os.Stat(dst)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		// Remove the destination
		if err := os.RemoveAll(dst); err != nil {
			return err
		}
	}

	// Create all the parent directories
	if err := os.MkdirAll(filepath.Dir(dst), g.client.mode(0755)); err != nil {
		return err
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}

	// Iterate through all matching objects.
	iter := client.Bucket(bucket).Objects(ctx, &storage.Query{Prefix: object})
	for {
		obj, err := iter.Next()
		if err != nil && err != iterator.Done {
			return err
		}
		if err == iterator.Done {
			break
		}

		if !strings.HasSuffix(obj.Name, "/") {
			// Get the object destination path
			objDst, err := filepath.Rel(object, obj.Name)
			if err != nil {
				return err
			}
			objDst = filepath.Join(dst, objDst)
			// Download the matching object.
			err = g.getObject(ctx, client, objDst, bucket, obj.Name, "")
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *GCSGetter) GetFile(dst string, u *url.URL) error {
	ctx := g.Context()

	// Parse URL
	bucket, object, fragment, err := g.parseURL(u)
	if err != nil {
		return err
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}
	return g.getObject(ctx, client, dst, bucket, object, fragment)
}

func (g *GCSGetter) getObject(ctx context.Context, client *storage.Client, dst, bucket, object, fragment string) error {
	var rc *storage.Reader
	var err error
	if fragment != "" {
		generation, err := strconv.ParseInt(fragment, 10, 64)
		if err != nil {
			return err
		}
		rc, err = client.Bucket(bucket).Object(object).Generation(generation).NewReader(ctx)
	} else {
		rc, err = client.Bucket(bucket).Object(object).NewReader(ctx)
	}
	if err != nil {
		return err
	}
	defer rc.Close()

	// Create all the parent directories
	if err := os.MkdirAll(filepath.Dir(dst), g.client.mode(0755)); err != nil {
		return err
	}

	return copyReader(dst, rc, 0666, g.client.umask())
}

func (g *GCSGetter) parseURL(u *url.URL) (bucket, path, fragment string, err error) {
	if strings.Contains(u.Host, "googleapis.com") {
		hostParts := strings.Split(u.Host, ".")
		if len(hostParts) != 3 {
			err = fmt.Errorf("URL is not a valid GCS URL")
			return
		}

		pathParts := strings.SplitN(u.Path, "/", 5)
		if len(pathParts) != 5 {
			err = fmt.Errorf("URL is not a valid GCS URL")
			return
		}
		bucket = pathParts[3]
		path = pathParts[4]
		fragment = u.Fragment
	}
	return
}
