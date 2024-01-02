// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

const (
	// bytesToLines is an estimation of how many bytes are in each log line.
	// This is used to set the offset to read from when a user specifies how
	// many lines to tail from.
	bytesToLines int64 = 120

	// defaultTailLines is the number of lines to tail by default if the value
	// is not overridden.
	defaultTailLines int64 = 10
)

type AllocFSCommand struct {
	Meta
}

func (f *AllocFSCommand) Help() string {
	helpText := `
Usage: nomad alloc fs [options] <allocation> <path>
Alias: nomad fs

  fs displays either the contents of an allocation directory for the passed
  allocation, or displays the file at the given path. The path is relative to
  the root of the alloc dir and defaults to root if unspecified.

  When ACLs are enabled, this command requires a token with the 'read-fs',
  'read-job', and 'list-jobs' capabilities for the allocation's namespace.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

FS Specific Options:

  -H
    Machine friendly output.

  -verbose
    Show full information.

  -job <job-id>
    Use a random allocation from the specified job ID or prefix.

  -stat
    Show file stat information instead of displaying the file, or listing the directory.

  -f
    Causes the output to not stop when the end of the file is reached, but rather to
    wait for additional output.

  -tail
    Show the files contents with offsets relative to the end of the file. If no
    offset is given, -n is defaulted to 10.

  -n
    Sets the tail location in best-efforted number of lines relative to the end
    of the file.

  -c
    Sets the tail location in number of bytes relative to the end of the file.
`
	return strings.TrimSpace(helpText)
}

func (f *AllocFSCommand) Synopsis() string {
	return "Inspect the contents of an allocation directory"
}

func (f *AllocFSCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(f.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-H":       complete.PredictNothing,
			"-verbose": complete.PredictNothing,
			"-job":     complete.PredictAnything,
			"-stat":    complete.PredictNothing,
			"-f":       complete.PredictNothing,
			"-tail":    complete.PredictNothing,
			"-n":       complete.PredictAnything,
			"-c":       complete.PredictAnything,
		})
}

func (f *AllocFSCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := f.Meta.Client()
		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.Allocs, nil)
		if err != nil {
			return []string{}
		}
		return resp.Matches[contexts.Allocs]
	})
}

func (f *AllocFSCommand) Name() string { return "alloc fs" }

func (f *AllocFSCommand) Run(args []string) int {
	var verbose, machine, job, stat, tail, follow bool
	var numLines, numBytes int64

	flags := f.Meta.FlagSet(f.Name(), FlagSetClient)
	flags.Usage = func() { f.Ui.Output(f.Help()) }
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.BoolVar(&machine, "H", false, "")
	flags.BoolVar(&job, "job", false, "")
	flags.BoolVar(&stat, "stat", false, "")
	flags.BoolVar(&follow, "f", false, "")
	flags.BoolVar(&tail, "tail", false, "")
	flags.Int64Var(&numLines, "n", -1, "")
	flags.Int64Var(&numBytes, "c", -1, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}
	args = flags.Args()

	if len(args) < 1 {
		if job {
			f.Ui.Error("A job ID is required")
		} else {
			f.Ui.Error("An allocation ID is required")
		}
		f.Ui.Error(commandErrorText(f))
		return 1
	}

	if len(args) > 2 {
		f.Ui.Error("This command takes one or two arguments: <allocation> [<path>]")
		f.Ui.Error(commandErrorText(f))
		return 1
	}

	path := "/"
	if len(args) == 2 {
		path = args[1]
	}

	client, err := f.Meta.Client()
	if err != nil {
		f.Ui.Error(fmt.Sprintf("Error initializing client: %v", err))
		return 1
	}

	// If -job is specified, use random allocation, otherwise use provided allocation
	allocID := args[0]
	if job {
		jobID, ns, err := f.JobIDByPrefix(client, args[0], nil)
		if err != nil {
			f.Ui.Error(err.Error())
			return 1
		}

		allocID, err = getRandomJobAllocID(client, jobID, ns)
		if err != nil {
			f.Ui.Error(fmt.Sprintf("Error fetching allocations: %v", err))
			return 1
		}
	}

	// Truncate the id unless full length is requested
	length := shortId
	if verbose {
		length = fullId
	}
	// Query the allocation info
	if len(allocID) == 1 {
		f.Ui.Error("Alloc ID must contain at least two characters.")
		return 1
	}

	allocID = sanitizeUUIDPrefix(allocID)
	allocs, _, err := client.Allocations().PrefixList(allocID)
	if err != nil {
		f.Ui.Error(fmt.Sprintf("Error querying allocation: %v", err))
		return 1
	}
	if len(allocs) == 0 {
		f.Ui.Error(fmt.Sprintf("No allocation(s) with prefix or id %q found", allocID))
		return 1
	}
	if len(allocs) > 1 {
		// Format the allocs
		out := formatAllocListStubs(allocs, verbose, length)
		f.Ui.Error(fmt.Sprintf("Prefix matched multiple allocations\n\n%s", out))
		return 1
	}
	// Prefix lookup matched a single allocation
	q := &api.QueryOptions{Namespace: allocs[0].Namespace}
	alloc, _, err := client.Allocations().Info(allocs[0].ID, q)
	if err != nil {
		f.Ui.Error(fmt.Sprintf("Error querying allocation: %s", err))
		return 1
	}

	// Get file stat info
	file, _, err := client.AllocFS().Stat(alloc, path, nil)
	if err != nil {
		f.Ui.Error(err.Error())
		return 1
	}

	// If we want file stats, print those and exit.
	if stat {
		// Display the file information
		out := make([]string, 2)
		out[0] = "Mode|Size|Modified Time|Content Type|Name"
		if file != nil {
			fn := file.Name
			if file.IsDir {
				fn = fmt.Sprintf("%s/", fn)
			}
			var size string
			if machine {
				size = fmt.Sprintf("%d", file.Size)
			} else {
				size = humanize.IBytes(uint64(file.Size))
			}
			out[1] = fmt.Sprintf("%s|%s|%s|%s|%s", file.FileMode, size,
				formatTime(file.ModTime), file.ContentType, fn)
		}
		f.Ui.Output(formatList(out))
		return 0
	}

	// Determine if the path is a file or a directory.
	if file.IsDir {
		// We have a directory, list it.
		files, _, err := client.AllocFS().List(alloc, path, nil)
		if err != nil {
			f.Ui.Error(fmt.Sprintf("Error listing alloc dir: %s", err))
			return 1
		}
		// Display the file information in a tabular format
		out := make([]string, len(files)+1)
		out[0] = "Mode|Size|Modified Time|Name"
		for i, file := range files {
			fn := file.Name
			if file.IsDir {
				fn = fmt.Sprintf("%s/", fn)
			}
			var size string
			if machine {
				size = fmt.Sprintf("%d", file.Size)
			} else {
				size = humanize.IBytes(uint64(file.Size))
			}
			out[i+1] = fmt.Sprintf("%s|%s|%s|%s",
				file.FileMode,
				size,
				formatTime(file.ModTime),
				fn,
			)
		}
		f.Ui.Output(formatList(out))
		return 0
	}

	// We have a file, output it.
	var r io.ReadCloser
	var readErr error
	if !tail {
		if follow {
			r, readErr = f.followFile(client, alloc, path, api.OriginStart, 0, -1)
		} else {
			r, readErr = client.AllocFS().Cat(alloc, path, nil)
		}

		if readErr != nil {
			readErr = fmt.Errorf("Error reading file: %v", readErr)
		}
	} else {
		// Parse the offset
		var offset int64 = defaultTailLines * bytesToLines

		if nLines, nBytes := numLines != -1, numBytes != -1; nLines && nBytes {
			f.Ui.Error("Both -n and -c are not allowed")
			return 1
		} else if numLines < -1 || numBytes < -1 {
			f.Ui.Error("Invalid size is specified")
			return 1
		} else if nLines {
			offset = numLines * bytesToLines
		} else if nBytes {
			offset = numBytes
		} else {
			numLines = defaultTailLines
		}

		if offset > file.Size {
			offset = file.Size
		}

		if follow {
			r, readErr = f.followFile(client, alloc, path, api.OriginEnd, offset, numLines)
		} else {
			// This offset needs to be relative from the front versus the follow
			// is relative to the end
			offset = file.Size - offset
			r, readErr = client.AllocFS().ReadAt(alloc, path, offset, -1, nil)

			// If numLines is set, wrap the reader
			if numLines != -1 {
				r = NewLineLimitReader(r, int(numLines), int(numLines*bytesToLines), 1*time.Second)
			}
		}

		if readErr != nil {
			readErr = fmt.Errorf("Error tailing file: %v", readErr)
		}
	}

	if r != nil {
		defer r.Close()
	}
	if readErr != nil {
		f.Ui.Error(readErr.Error())
		return 1
	}

	_, err = io.Copy(os.Stdout, r)
	if err != nil {
		f.Ui.Error(fmt.Sprintf("error tailing file: %s", err))
		return 1
	}

	return 0
}

// followFile outputs the contents of the file to stdout relative to the end of
// the file. If numLines does not equal -1, then tail -n behavior is used.
func (f *AllocFSCommand) followFile(client *api.Client, alloc *api.Allocation,
	path, origin string, offset, numLines int64) (io.ReadCloser, error) {

	cancel := make(chan struct{})
	frames, errCh := client.AllocFS().Stream(alloc, path, origin, offset, cancel, nil)
	select {
	case err := <-errCh:
		return nil, err
	default:
	}
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)

	// Create a reader
	var r io.ReadCloser
	frameReader := api.NewFrameReader(frames, errCh, cancel)
	frameReader.SetUnblockTime(500 * time.Millisecond)
	r = frameReader

	// If numLines is set, wrap the reader
	if numLines != -1 {
		r = NewLineLimitReader(r, int(numLines), int(numLines*bytesToLines), 1*time.Second)
	}

	go func() {
		<-signalCh

		// End the streaming
		r.Close()
	}()

	return r, nil
}

// Get Random Allocation from a known jobID. Prefer to use a running allocation,
// but use a dead allocation if no running allocations are found
func getRandomJobAlloc(client *api.Client, jobID, namespace string) (*api.AllocationListStub, error) {
	var runningAllocs []*api.AllocationListStub
	q := &api.QueryOptions{
		Namespace: namespace,
	}

	allocs, _, err := client.Jobs().Allocations(jobID, false, q)
	if err != nil {
		return nil, fmt.Errorf("error querying job %q: %w", jobID, err)
	}

	// Check that the job actually has allocations
	if len(allocs) == 0 {
		return nil, fmt.Errorf("job %q doesn't exist or it has no allocations", jobID)
	}

	for _, v := range allocs {
		if v.ClientStatus == "running" {
			runningAllocs = append(runningAllocs, v)
		}
	}
	// If we don't have any allocations running, use dead allocations
	if len(runningAllocs) < 1 {
		runningAllocs = allocs
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	alloc := runningAllocs[r.Intn(len(runningAllocs))]

	return alloc, err
}

func getRandomJobAllocID(client *api.Client, jobID, namespace string) (string, error) {
	alloc, err := getRandomJobAlloc(client, jobID, namespace)
	if err != nil {
		return "", err
	}

	return alloc.ID, nil
}
