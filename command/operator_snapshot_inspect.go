// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/nomad/helper/snapshot"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/raft"
	"github.com/posener/complete"
)

type OperatorSnapshotInspectCommand struct {
	Meta
}

type typeStats struct {
	Name  string
	Sum   int
	Count int
}

type SnapshotInspectFormat struct {
	Meta  *raft.SnapshotMeta
	Stats []typeStats
}

// SnapshotInfo is used for passing snapshot stat
// information between functions
type SnapshotInfo struct {
	Stats     map[nomad.SnapshotType]typeStats
	TotalSize int
}

// countingReader helps keep track of the bytes we have read
// when reading snapshots
type countingReader struct {
	wrappedReader io.Reader
	read          int
}

func (r *countingReader) Read(p []byte) (n int, err error) {
	n, err = r.wrappedReader.Read(p)
	if err == nil {
		r.read += n
	}
	return n, err
}

func (c *OperatorSnapshotInspectCommand) Help() string {
	helpText := `
Usage: nomad operator snapshot inspect [options] <file>

  Displays information about a snapshot file on disk.
  The output will include all snapshot types and their
  respective sizes, sorted in descending order.

  To inspect the file "backup.snap":
    $ nomad operator snapshot inspect backup.snap

Snapshot Inspect Options:

  -json
  	Output the snapshot inspect in its JSON format.
`
	return strings.TrimSpace(helpText)
}

func (c *OperatorSnapshotInspectCommand) AutocompleteFlags() complete.Flags {
	return complete.Flags{}
}

func (c *OperatorSnapshotInspectCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *OperatorSnapshotInspectCommand) Synopsis() string {
	return "Displays information about a Nomad snapshot file"
}

func (c *OperatorSnapshotInspectCommand) Name() string { return "operator snapshot inspect" }

func (c *OperatorSnapshotInspectCommand) Run(args []string) int {
	var json bool

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&json, "json", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we either got no filename or exactly one.
	args = flags.Args()
	if len(args) != 1 {
		c.Ui.Error("This command takes one argument: <filename>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	path := args[0]
	f, err := os.Open(path)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error opening snapshot file: %s", err))
		return 1
	}
	defer f.Close()

	meta, info, err := inspect(f)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error inspecting snapshot: %s", err))
		return 1
	}
	stats := generateStats(info)

	// format as JSON if requested
	if json {
		data := SnapshotInspectFormat{
			Meta:  meta,
			Stats: stats,
		}
		out, err := Format(json, "", data)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	// print human-readable output
	c.Ui.Output(formatListWithSpaces([]string{
		fmt.Sprintf("Created|%s", extractTimeFromName(meta.ID)),
		fmt.Sprintf("ID|%s", meta.ID),
		fmt.Sprintf("Size|%s", ByteToHumanString(uint64(meta.Size))),
		fmt.Sprintf("Index|%d", meta.Index),
		fmt.Sprintf("Term|%d", meta.Term),
		fmt.Sprintf("Version|%d", meta.Version),
	}))
	c.Ui.Output("")

	output := []string{
		"Type|Count|Size",
		"----|-----|----",
	}

	for _, stat := range stats {
		output = append(output, fmt.Sprintf("%s|%d|%s", stat.Name, stat.Count, ByteToHumanString(uint64(stat.Sum))))
	}
	output = append(output, "----|-----|----")
	output = append(output, fmt.Sprintf("Total|-|%s", ByteToHumanString(uint64(info.TotalSize))))

	c.Ui.Output(formatList(output))
	return 0
}

func inspect(file io.Reader) (*raft.SnapshotMeta, *SnapshotInfo, error) {
	info := &SnapshotInfo{
		Stats:     make(map[nomad.SnapshotType]typeStats),
		TotalSize: 0,
	}

	// w is closed by CopySnapshot
	r, w := io.Pipe()
	cr := &countingReader{wrappedReader: r}
	errCh := make(chan error)
	metaCh := make(chan *raft.SnapshotMeta)

	go func() {
		meta, err := snapshot.CopySnapshot(file, w)
		if err != nil {
			errCh <- fmt.Errorf("failed to read snapshot: %w", err)
		} else {
			metaCh <- meta
		}
	}()

	handler := func(header *nomad.SnapshotHeader, snapType nomad.SnapshotType, dec *codec.Decoder) error {
		name := snapType.String()
		stat := info.Stats[snapType]

		if stat.Name == "" {
			stat.Name = name
		}

		var val interface{}
		err := dec.Decode(&val)
		if err != nil {
			return fmt.Errorf("failed to decode snapshot %q: %v", snapType, err)
		}

		size := cr.read - info.TotalSize
		stat.Sum += size
		stat.Count++
		info.TotalSize = cr.read
		info.Stats[snapType] = stat

		return nil
	}

	err := nomad.ReadSnapshot(cr, handler)
	if err != nil {
		return nil, nil, err
	}

	select {
	case err := <-errCh:
		return nil, nil, err
	case meta := <-metaCh:
		return meta, info, nil
	}
}

func generateStats(info *SnapshotInfo) []typeStats {
	ss := make([]typeStats, 0, len(info.Stats))
	for _, stat := range info.Stats {
		ss = append(ss, stat)
	}

	// sort by Sum
	sort.Slice(ss, func(i, j int) bool {
		// sort alphabetically if size is equal
		if ss[i].Sum == ss[j].Sum {
			return ss[i].Name < ss[j].Name
		}
		return ss[i].Sum > ss[j].Sum
	})

	return ss
}

// Raft snapshot name is in format of <term>-<index>-<time-milliseconds>
// we will extract the creation time
func extractTimeFromName(snapshotName string) string {
	parts := strings.Split(snapshotName, "-")
	if len(parts) != 3 {
		return ""
	}
	msec, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return ""
	}
	return formatTime(time.UnixMilli(msec))
}
