package logmon

import (
	"os/exec"
	"os/user"
	"strconv"
	"syscall"

	hclog "github.com/hashicorp/go-hclog"
)

const DefaultLogMonUser = "nobody"

func newPluginCmd(nomadBin string, logger hclog.Logger) *exec.Cmd {

	cmd := exec.Command(nomadBin, "logmon")

	u, err := user.Lookup(DefaultLogMonUser)
	if err != nil {
		logger.Warn("failed to set user for logmon process, running as current user instead", "error", err)
		return cmd
	}

	// Get the groups the user is a part of
	gidStrings, err := u.GroupIds()
	if err != nil {
		logger.Warn("failed to set user for logmon process, running as current user instead", "error", err)
		return cmd
	}

	gids := make([]uint32, len(gidStrings))
	for _, gidString := range gidStrings {
		u, err := strconv.Atoi(gidString)
		if err != nil {
			logger.Warn("failed to set user for logmon process, running as current user instead", "error", err)
			return cmd
		}

		gids = append(gids, uint32(u))
	}

	// Convert the uid and gid
	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		logger.Warn("failed to set user for logmon process, running as current user instead", "error", err)
		return cmd
	}
	gid, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		logger.Warn("failed to set user for logmon process, running as current user instead", "error", err)
		return cmd
	}

	// Set the command to run as that user and group.
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	if cmd.SysProcAttr.Credential == nil {
		cmd.SysProcAttr.Credential = &syscall.Credential{}
	}
	cmd.SysProcAttr.Credential.Uid = uint32(uid)
	cmd.SysProcAttr.Credential.Gid = uint32(gid)
	cmd.SysProcAttr.Credential.Groups = gids

	logger.Debug("setting logmon process user", "user", uid, "group", gid, "additional_groups", gids)

	return cmd
}
