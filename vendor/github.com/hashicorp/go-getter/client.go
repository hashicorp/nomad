package getter

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	urlhelper "github.com/hashicorp/go-getter/helper/url"
)

// Client is a client for downloading things.
//
// Top-level functions such as Get are shortcuts for interacting with a client.
// Using a client directly allows more fine-grained control over how downloading
// is done, as well as customizing the protocols supported.
type Client struct {
	// Src is the source URL to get.
	//
	// Dst is the path to save the downloaded thing as. If Dir is set to
	// true, then this should be a directory. If the directory doesn't exist,
	// it will be created for you.
	//
	// Pwd is the working directory for detection. If this isn't set, some
	// detection may fail. Client will not default pwd to the current
	// working directory for security reasons.
	Src string
	Dst string
	Pwd string

	// Dir, if true, tells the Client it is downloading a directory (versus
	// a single file). This distinction is necessary since filenames and
	// directory names follow the same format so disambiguating is impossible
	// without knowing ahead of time.
	Dir bool

	// Detectors is the list of detectors that are tried on the source.
	// If this is nil, then the default Detectors will be used.
	Detectors []Detector

	// Getters is the map of protocols supported by this client. If this
	// is nil, then the default Getters variable will be used.
	Getters map[string]Getter
}

// Get downloads the configured source to the destination.
func (c *Client) Get() error {
	// Detect the URL. This is safe if it is already detected.
	detectors := c.Detectors
	if detectors == nil {
		detectors = Detectors
	}
	src, err := Detect(c.Src, c.Pwd, detectors)
	if err != nil {
		return err
	}

	// Determine if we have a forced protocol, i.e. "git::http://..."
	force, src := getForcedGetter(src)

	// If there is a subdir component, then we download the root separately
	// and then copy over the proper subdir.
	var realDst string
	dst := c.Dst
	src, subDir := SourceDirSubdir(src)
	if subDir != "" {
		tmpDir, err := ioutil.TempDir("", "tf")
		if err != nil {
			return err
		}
		if err := os.RemoveAll(tmpDir); err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)

		realDst = dst
		dst = tmpDir
	}

	u, err := urlhelper.Parse(src)
	if err != nil {
		return err
	}
	if force == "" {
		force = u.Scheme
	}

	getters := c.Getters
	if getters == nil {
		getters = Getters
	}

	g, ok := getters[force]
	if !ok {
		return fmt.Errorf(
			"download not supported for scheme '%s'", force)
	}

	// Determine if we have a checksum
	var checksumHash hash.Hash
	var checksumValue []byte
	q := u.Query()
	if v := q.Get("checksum"); v != "" {
		// Delete the query parameter if we have it.
		q.Del("checksum")
		u.RawQuery = q.Encode()

		// If we're getting a directory, then this is an error. You cannot
		// checksum a directory. TODO: test
		if c.Dir {
			return fmt.Errorf(
				"checksum cannot be specified for directory download")
		}

		// Determine the checksum hash type
		checksumType := ""
		idx := strings.Index(v, ":")
		if idx > -1 {
			checksumType = v[:idx]
		}
		switch checksumType {
		case "md5":
			checksumHash = md5.New()
		case "sha1":
			checksumHash = sha1.New()
		case "sha256":
			checksumHash = sha256.New()
		case "sha512":
			checksumHash = sha512.New()
		default:
			return fmt.Errorf(
				"unsupported checksum type: %s", checksumType)
		}

		// Get the remainder of the value and parse it into bytes
		b, err := hex.DecodeString(v[idx+1:])
		if err != nil {
			return fmt.Errorf("invalid checksum: %s", err)
		}

		// Set our value
		checksumValue = b
	}

	// If we're not downloading a directory, then just download the file
	// and return.
	if !c.Dir {
		err := g.GetFile(dst, u)
		if err != nil {
			return err
		}

		if checksumHash != nil {
			return checksum(dst, checksumHash, checksumValue)
		}

		return nil
	}

	// We're downloading a directory, which might require a bit more work
	// if we're specifying a subdir.
	err = g.Get(dst, u)
	if err != nil {
		err = fmt.Errorf("error downloading '%s': %s", src, err)
		return err
	}

	// If we have a subdir, copy that over
	if subDir != "" {
		if err := os.RemoveAll(realDst); err != nil {
			return err
		}
		if err := os.MkdirAll(realDst, 0755); err != nil {
			return err
		}

		return copyDir(realDst, filepath.Join(dst, subDir), false)
	}

	return nil
}

// checksum is a simple method to compute the checksum of a source file
// and compare it to the given expected value.
func checksum(source string, h hash.Hash, v []byte) error {
	f, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("Failed to open file for checksum: %s", err)
	}
	defer f.Close()

	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("Failed to hash: %s", err)
	}

	if actual := h.Sum(nil); !bytes.Equal(actual, v) {
		return fmt.Errorf(
			"Checksums did not match.\nExpected: %s\nGot: %s",
			hex.EncodeToString(v),
			hex.EncodeToString(actual))
	}

	return nil
}
