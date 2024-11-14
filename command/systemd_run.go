package command

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/coreos/go-systemd/v22/unit"
	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/posener/complete"
)

type SystemdRunCommand struct {
	Meta
	JobGetter

	baseDir string
}

func (c *SystemdRunCommand) Help() string {
	helpText := `
Usage: nomad systemd run [options] [args]

  This command runs a jobspec with systemd tasks as a systemd job locally,
  damaging your reputation as a serious engineer and possibly your sanity.
`
	return strings.TrimSpace(helpText)
}

func (c *SystemdRunCommand) Synopsis() string {
	return "Run a Nomad jobspec as a set of systemd units"
}

func (c *SystemdRunCommand) Name() string { return "systemd run" }

func (c *SystemdRunCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictOr(
		complete.PredictFiles("*.nomad"),
		complete.PredictFiles("*.hcl"),
		complete.PredictFiles("*.json"),
	)
}

type Unit struct {
	Name     string
	Path     string
	Sections []*unit.UnitSection
}

func (c *SystemdRunCommand) Run(args []string) int {

	var useSystemBus bool
	var usePersistentUnits bool
	var startOnLoad bool
	var unitDirectory string
	var baseDir string

	homeDir, err := os.UserHomeDir()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("could not determine default user unit directory: %s", err))
		return 1
	}
	defaultUnitDirectory := filepath.Join(homeDir, ".local/share/nomad-systemd")

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&useSystemBus, "system", false, "Use the system bus (requires root)")
	flags.BoolVar(&usePersistentUnits, "persistent", false, "Make the units persistent after reboot")
	flags.BoolVar(&startOnLoad, "start", true, "Start the units that are created")

	flags.StringVar(&unitDirectory, "directory",
		defaultUnitDirectory,
		"Use this directory for placing unit files (must be writable)")

	if err := flags.Parse(args); err != nil {
		return 1
	}
	args = flags.Args()

	defaultBaseDir := os.Getenv("XDG_RUNTIME_DIR")
	if defaultBaseDir == "" {
		defaultBaseDir = "/tmp"
	}
	defaultBaseDir = filepath.Join(defaultBaseDir, "nomad-systemd-allocs")

	flags.StringVar(&baseDir, "basedir",
		defaultBaseDir,
		"Use this directory for creating alloc directories (must be writable)")
	c.baseDir = baseDir

	// load the HCL into an api.Job struct
	if err := c.JobGetter.Validate(); err != nil {
		c.Ui.Error(fmt.Sprintf("Invalid job options: %s", err))
		return 1
	}
	_, job, err := c.JobGetter.Get(args[0])
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error getting job struct: %s", err))
		return 1
	}

	fi, err := os.Stat(unitDirectory)
	if err != nil {
		err = os.Mkdir(unitDirectory, 0744)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Could not create unit directory: %v", err))
			return 1
		}
	} else if fi != nil && !fi.IsDir() {
		c.Ui.Error(fmt.Sprintf("Unit directory %q exists but is not a directory", unitDirectory))
	}

	units := []*Unit{}

	for _, tg := range job.TaskGroups {
		tgUnits, err := c.processTaskGroup(*job.ID, tg)
		if err != nil {
			c.Ui.Error(fmt.Sprintf(
				"Could not generate systemd units from task group %q: %v", *tg.Name, err))
			return 1
		}
		units = append(units, tgUnits...)

	}
	for _, u := range units {
		r := unit.SerializeSections(u.Sections)
		buf := &bytes.Buffer{}
		io.Copy(buf, r)
		u.Path = filepath.Join(unitDirectory, u.Name)
		err := os.WriteFile(u.Path, buf.Bytes(), 0644)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Could not write unit file %q: %v",
				u.Path, err))
			return 1
		}
		c.Ui.Info(fmt.Sprintf("Wrote unit file to %s", u.Path))
	}

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	var conn *dbus.Conn
	if useSystemBus {
		conn, err = dbus.NewSystemdConnectionContext(ctx)
	} else {
		conn, err = dbus.NewSystemdConnectionContext(ctx)
	}
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Could not connect to dbus: %v", err))
		return 1
	}

	unitFiles := helper.ConvertSlice(units, func(u *Unit) string { return u.Path })
	_, err = conn.LinkUnitFilesContext(ctx, unitFiles, !usePersistentUnits, true)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Could not link unit files: %v", err))
		return 1
	}

	if !startOnLoad {
		c.Ui.Output("Created units:")
		for _, u := range units {
			c.Ui.Output(fmt.Sprintf("\t%s", u.Name))
		}
		return 0
	}

	chans := []*unitStatusFuture{}

	for _, u := range units {
		startCh := make(chan string, 1)
		_, err := conn.StartUnitContext(ctx, u.Name, "replace", startCh)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Could not start unit %q: %v", u.Name, err))
			return 1
		}
		chans = append(chans, &unitStatusFuture{
			name: u.Name,
			ch:   startCh,
		})
	}

	var wg sync.WaitGroup

	for _, ch := range chans {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case <-ctx.Done():
				return
			case event := <-ch.ch:
				c.Ui.Info(fmt.Sprintf("==> %s: %s", ch.name, event))
				return
			}
		}()
	}

	wg.Wait()
	return 0
}

type unitStatusFuture struct {
	name string
	ch   chan string
}

func (c *SystemdRunCommand) processTaskGroup(jobID string, tg *api.TaskGroup) ([]*Unit, error) {

	if len(tg.Networks) > 1 {
		return nil, fmt.Errorf("'nomad systemd run' can only support a single network per task group")
	}
	var hasBridge bool
	for _, network := range tg.Networks {
		// TODO: turn this into a... what kind of unit?
		if network.Mode != "bridge" {
			return nil, fmt.Errorf("'nomad systemd run' can only support bridge networking (barely)")
		} else {
			hasBridge = true
		}
	}

	units := []*Unit{}

	// for _, volume := range tg.Volumes {
	// 	// TODO: turn this into a mount unit?
	// 	spew.Dump(volume)
	// }

	// for _, service := range tg.Services {
	// 	// TODO: register with Nomad
	// 	spew.Dump(service)
	// }

	for _, task := range tg.Tasks {
		if task.Driver != "systemd" {
			return nil, fmt.Errorf("'nomad systemd run' can only run systemd tasks")
		}

		name := fmt.Sprintf("%s_%s_%s", jobID, *tg.Name, task.Name)
		u := &Unit{
			Name:     name + ".service",
			Sections: []*unit.UnitSection{},
		}

		u.AddEntry("Unit", "Description", name)
		u.AddEntry("Unit", "Requires", "network-online.target")
		u.AddEntry("Unit", "After", "network-online.target")

		// ref https://www.cloudnull.io/2019/04/running-services-in-network-name-spaces-with-systemd/
		if hasBridge {
			u.AddEntry("Unit", "BindsTo", "systemd-netns@"+u.Name)
			u.AddEntry("Unit", "JoinsNamespaceOf", "systemd-netns@"+u.Name)
			u.AddEntry("Unit", "After", "systemd-netns@"+u.Name)
		}

		u.AddEntry("Service", "User", task.User)

		cmdArgs := []string{}
		if raw, ok := task.Config["command"]; ok {
			cmd := raw.(string)
			cmdArgs = append(cmdArgs, cmd)
		}
		if raw, ok := task.Config["args"]; ok {
			rawArgs := raw.([]interface{})
			for _, rawArg := range rawArgs {
				arg := rawArg.(string)
				cmdArgs = append(cmdArgs, arg)
			}
		}
		if len(cmdArgs) == 0 {
			return nil, fmt.Errorf("task %q in group %q is missing command and/or arguments",
				task.Name, *tg.Name)
		}
		u.AddEntry("Service", "ExecStart", strings.Join(cmdArgs, " "))

		ipcMode := "private"
		if raw, ok := task.Config["ipc_mode"]; ok {
			ipcMode = raw.(string)
		}
		switch ipcMode {
		case "private":
			u.AddEntry("Service", "PrivateIPC", "true")
		case "host", "":
		default:
			return nil, fmt.Errorf(
				"invalid ipc_mode=%q for task %q in group %q is missing command and/or arguments",
				ipcMode, task.Name, *tg.Name)

		}

		// TODO: PrivateNetwork= or NetworkNamespacePath=
		// ref https://www.freedesktop.org/software/systemd/man/latest/systemd.exec.html

		// https://www.freedesktop.org/software/systemd/man/latest/systemd.exec.html#WorkingDirectory=
		if raw, ok := task.Config["work_dir"]; ok {
			workDir := raw.(string)
			u.AddEntry("Service", "WorkingDirectory", workDir)
		}

		// https://www.freedesktop.org/software/systemd/man/latest/systemd.exec.html#BindPaths=
		baseDir := filepath.Join(c.baseDir, jobID, *tg.Name)
		allocDir := filepath.Join(baseDir, "alloc")
		taskDir := filepath.Join(baseDir, name)
		logDir := filepath.Join(allocDir, "logs")

		// https://www.freedesktop.org/software/systemd/man/latest/systemd.exec.html#ProtectSystem=
		u.AddEntry("Service", "ProtectSystem", "full")

		// https://www.freedesktop.org/software/systemd/man/latest/systemd.exec.html#ReadWritePaths=
		readOnlyPaths := []string{"/"}
		// readWritePaths := []string{"/TODO"}
		u.AddEntry("Service", "ReadOnlyPaths", strings.Join(readOnlyPaths, " "))
		// u.AddEntry("Service", "ReadWritePaths", strings.Join(readWritePaths, " "))

		// https://www.freedesktop.org/software/systemd/man/latest/systemd.exec.html#PrivateTmp=
		u.AddEntry("Service", "PrivateTmp", "true")

		// https://www.freedesktop.org/software/systemd/man/latest/systemd.exec.html#RuntimeDirectory=
		u.AddEntry("Service", "StateDirectory", strings.Join(
			[]string{allocDir, taskDir}, " "))
		u.AddEntry("Service", "LogsDirectory", logDir)

		// https://www.freedesktop.org/software/systemd/man/latest/systemd.exec.html#CapabilityBoundingSet=
		caps := set.From([]string{
			"CAP_AUDIT_WRITE", "CAP_CHOWN", "CAP_DAC_OVERRIDE", "CAP_FOWNER",
			"CAP_FSETID", "CAP_KILL", "CAP_MKNOD", "CAP_NET_BIND_SERVICE",
			"CAP_SETFCAP", "CAP_SETGID", "CAP_SETPCAP", "CAP_SETUID", "CAP_SYS_CHROOT"})
		if raw, ok := task.Config["cap_add"]; ok {
			capAdd := raw.([]string)
			for _, cap := range capAdd {
				caps.Insert(cap)
			}
		}
		if raw, ok := task.Config["cap_drop"]; ok {
			capDrop := raw.([]string)
			for _, cap := range capDrop {
				caps.Remove(cap)
			}
		}
		u.AddEntry("Service", "CapabilityBoundingSet", strings.Join(caps.Slice(), " "))

		u.AddEntry("Service", "KillMode", "process")
		u.AddEntry("Service", "Restart", "on-failure")
		if task.RestartPolicy != nil && task.RestartPolicy.Interval != nil {
			u.AddEntry("Service", "RestartSec", task.RestartPolicy.Interval.String())
		}

		units = append(units, u)
	}

	return units, nil
}

func (u *Unit) AddEntry(sectionName, key, value string) {

	for _, section := range u.Sections {
		if section.Section == sectionName {
			if len(section.Entries) == 0 {
				section.Entries = []*unit.UnitEntry{{
					Name:  key,
					Value: value,
				}}
			} else {
				section.Entries = append(section.Entries, &unit.UnitEntry{
					Name:  key,
					Value: value,
				})
			}
			return
		}
	}

	// didn't find section
	u.Sections = append(u.Sections, &unit.UnitSection{
		Section: sectionName,
		Entries: []*unit.UnitEntry{{
			Name:  key,
			Value: value,
		}},
	})

}
