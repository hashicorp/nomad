// Copyright © 2013, 2014, The Go-LXC Authors. All rights reserved.
// Use of this source code is governed by a LGPLv2.1
// license that can be found in the LICENSE file.

// +build linux,cgo

#include <stdbool.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <errno.h>

#include <lxc/lxccontainer.h>
#include <lxc/attach_options.h>
#include <lxc/version.h>

#include "lxc-binding.h"

#define VERSION_AT_LEAST(major, minor, micro)							\
	(!(major > LXC_VERSION_MAJOR ||								\
	major == LXC_VERSION_MAJOR && minor > LXC_VERSION_MINOR ||				\
	major == LXC_VERSION_MAJOR && minor == LXC_VERSION_MINOR && micro > LXC_VERSION_MICRO))

bool go_lxc_defined(struct lxc_container *c) {
	return c->is_defined(c);
}

const char* go_lxc_state(struct lxc_container *c) {
	return c->state(c);
}

bool go_lxc_running(struct lxc_container *c) {
	return c->is_running(c);
}

bool go_lxc_freeze(struct lxc_container *c) {
	return c->freeze(c);
}

bool go_lxc_unfreeze(struct lxc_container *c) {
	return c->unfreeze(c);
}

pid_t go_lxc_init_pid(struct lxc_container *c) {
	return c->init_pid(c);
}

bool go_lxc_want_daemonize(struct lxc_container *c, bool state) {
	return c->want_daemonize(c, state);
}

bool go_lxc_want_close_all_fds(struct lxc_container *c, bool state) {
	return c->want_close_all_fds(c, state);
}

bool go_lxc_create(struct lxc_container *c, const char *t, const char *bdevtype, int flags, char * const argv[]) {
	return c->create(c, t, bdevtype, NULL, !!(flags & LXC_CREATE_QUIET), argv);
}

bool go_lxc_start(struct lxc_container *c, int useinit, char * const argv[]) {
	return c->start(c, useinit, argv);
}

bool go_lxc_stop(struct lxc_container *c) {
	return c->stop(c);
}

bool go_lxc_reboot(struct lxc_container *c) {
	return c->reboot(c);
}

bool go_lxc_shutdown(struct lxc_container *c, int timeout) {
	return c->shutdown(c, timeout);
}

char* go_lxc_config_file_name(struct lxc_container *c) {
	return c->config_file_name(c);
}

bool go_lxc_destroy(struct lxc_container *c) {
	return c->destroy(c);
}

bool go_lxc_destroy_with_snapshots(struct lxc_container *c) {
#if VERSION_AT_LEAST(1, 1, 0)
	return c->destroy_with_snapshots(c);
#else
	return false;
#endif
}

bool go_lxc_wait(struct lxc_container *c, const char *state, int timeout) {
	return c->wait(c, state, timeout);
}

char* go_lxc_get_config_item(struct lxc_container *c, const char *key) {
	int len = c->get_config_item(c, key, NULL, 0);
	if (len <= 0) {
		return NULL;
	}

	char* value = (char*)malloc(sizeof(char)*len + 1);
	if (c->get_config_item(c, key, value, len + 1) != len) {
		return NULL;
	}
	return value;
}

bool go_lxc_set_config_item(struct lxc_container *c, const char *key, const char *value) {
	return c->set_config_item(c, key, value);
}

void go_lxc_clear_config(struct lxc_container *c) {
	c->clear_config(c);
}

bool go_lxc_clear_config_item(struct lxc_container *c, const char *key) {
	return c->clear_config_item(c, key);
}

char* go_lxc_get_running_config_item(struct lxc_container *c, const char *key) {
	return c->get_running_config_item(c, key);
}

char* go_lxc_get_keys(struct lxc_container *c, const char *key) {
	int len = c->get_keys(c, key, NULL, 0);
	if (len <= 0) {
		return NULL;
	}

	char* value = (char*)malloc(sizeof(char)*len + 1);
	if (c->get_keys(c, key, value, len + 1) != len) {
		return NULL;
	}
	return value;
}

char* go_lxc_get_cgroup_item(struct lxc_container *c, const char *key) {
	int len = c->get_cgroup_item(c, key, NULL, 0);
	if (len <= 0) {
		return NULL;
	}

	char* value = (char*)malloc(sizeof(char)*len + 1);
	if (c->get_cgroup_item(c, key, value, len + 1) != len) {
		return NULL;
	}
	return value;
}

bool go_lxc_set_cgroup_item(struct lxc_container *c, const char *key, const char *value) {
	return c->set_cgroup_item(c, key, value);
}

const char* go_lxc_get_config_path(struct lxc_container *c) {
	return c->get_config_path(c);
}

bool go_lxc_set_config_path(struct lxc_container *c, const char *path) {
	return c->set_config_path(c, path);
}

bool go_lxc_load_config(struct lxc_container *c, const char *alt_file) {
	return c->load_config(c, alt_file);
}

bool go_lxc_save_config(struct lxc_container *c, const char *alt_file) {
	return c->save_config(c, alt_file);
}

bool go_lxc_clone(struct lxc_container *c, const char *newname, const char *lxcpath, int flags, const char *bdevtype) {
	return c->clone(c, newname, lxcpath, flags, bdevtype, NULL, 0, NULL) != NULL;
}

int go_lxc_console_getfd(struct lxc_container *c, int ttynum) {
	int masterfd;

	if (c->console_getfd(c, &ttynum, &masterfd) < 0) {
		return -1;
	}
	return masterfd;
}

bool go_lxc_console(struct lxc_container *c, int ttynum, int stdinfd, int stdoutfd, int stderrfd, int escape) {

	if (c->console(c, ttynum, stdinfd, stdoutfd, stderrfd, escape) == 0) {
		return true;
	}
	return false;
}

char** go_lxc_get_interfaces(struct lxc_container *c) {
	return c->get_interfaces(c);
}

char** go_lxc_get_ips(struct lxc_container *c, const char *interface, const char *family, int scope) {
	return c->get_ips(c, interface, family, scope);
}

int wait_for_pid_status(pid_t pid)
{
        int status, ret;

again:
        ret = waitpid(pid, &status, 0);
        if (ret == -1) {
                if (errno == EINTR)
                        goto again;
                return -1;
        }
        if (ret != pid)
                goto again;
        return status;
}

int go_lxc_attach(struct lxc_container *c,
		bool clear_env,
		int namespaces,
		long personality,
		uid_t uid, gid_t gid,
		int stdinfd, int stdoutfd, int stderrfd,
		char *initial_cwd,
		char **extra_env_vars,
		char **extra_keep_env) {
	int ret;
	pid_t pid;

	lxc_attach_options_t attach_options = LXC_ATTACH_OPTIONS_DEFAULT;

	attach_options.env_policy = LXC_ATTACH_KEEP_ENV;
	if (clear_env) {
		attach_options.env_policy = LXC_ATTACH_CLEAR_ENV;
	}

	attach_options.namespaces = namespaces;
	attach_options.personality = personality;

	attach_options.uid = uid;
	attach_options.gid = gid;

	attach_options.stdin_fd = stdinfd;
	attach_options.stdout_fd = stdoutfd;
	attach_options.stderr_fd = stderrfd;

	attach_options.initial_cwd = initial_cwd;
	attach_options.extra_env_vars = extra_env_vars;
	attach_options.extra_keep_env = extra_keep_env;

	/*
	   remount_sys_proc
	   When using -s and the mount namespace is not included, this flag will cause lxc-attach to remount /proc and /sys to reflect the current other namespace contexts.
	   default_options.attach_flags |= LXC_ATTACH_REMOUNT_PROC_SYS;

	   elevated_privileges
	   Do  not  drop privileges when running command inside the container. If this option is specified, the new process will not be added to the container's cgroup(s) and it will not drop its capabilities before executing.
	   default_options.attach_flags &= ~(LXC_ATTACH_MOVE_TO_CGROUP | LXC_ATTACH_DROP_CAPABILITIES | LXC_ATTACH_APPARMOR);
	   */

	ret = c->attach(c, lxc_attach_run_shell, NULL, &attach_options, &pid);
	if (ret < 0)
		return -1;

	ret = wait_for_pid_status(pid);
	if (ret < 0)
		return -1;

	if (WIFEXITED(ret))
		return WEXITSTATUS(ret);

	return -1;
}

int go_lxc_attach_run_wait(struct lxc_container *c,
		bool clear_env,
		int namespaces,
		long personality,
		uid_t uid, gid_t gid,
		int stdinfd, int stdoutfd, int stderrfd,
		char *initial_cwd,
		char **extra_env_vars,
		char **extra_keep_env,
		const char * const argv[]) {
	int ret;

	lxc_attach_options_t attach_options = LXC_ATTACH_OPTIONS_DEFAULT;

	attach_options.env_policy = LXC_ATTACH_KEEP_ENV;
	if (clear_env) {
		attach_options.env_policy = LXC_ATTACH_CLEAR_ENV;
	}

	attach_options.namespaces = namespaces;
	attach_options.personality = personality;

	attach_options.uid = uid;
	attach_options.gid = gid;

	attach_options.stdin_fd = stdinfd;
	attach_options.stdout_fd = stdoutfd;
	attach_options.stderr_fd = stderrfd;

	attach_options.initial_cwd = initial_cwd;
	attach_options.extra_env_vars = extra_env_vars;
	attach_options.extra_keep_env = extra_keep_env;

	ret = c->attach_run_wait(c, &attach_options, argv[0], argv);
	if (WIFEXITED(ret) && WEXITSTATUS(ret) == 255)
		return -1;
	return ret;
}

bool go_lxc_may_control(struct lxc_container *c) {
	return c->may_control(c);
}

int go_lxc_snapshot(struct lxc_container *c) {
	return c->snapshot(c, NULL);
}

int go_lxc_snapshot_list(struct lxc_container *c, struct lxc_snapshot **ret) {
	return c->snapshot_list(c, ret);
}

bool go_lxc_snapshot_restore(struct lxc_container *c, const char *snapname, const char *newname) {
	return c->snapshot_restore(c, snapname, newname);
}

bool go_lxc_snapshot_destroy(struct lxc_container *c, const char *snapname) {
	return c->snapshot_destroy(c, snapname);
}

bool go_lxc_snapshot_destroy_all(struct lxc_container *c) {
#if VERSION_AT_LEAST(1, 1, 0)
	return c->snapshot_destroy_all(c);
#else
	return false;
#endif

}

bool go_lxc_add_device_node(struct lxc_container *c, const char *src_path, const char *dest_path) {
	return c->add_device_node(c, src_path, dest_path);
}

bool go_lxc_remove_device_node(struct lxc_container *c, const char *src_path, const char *dest_path) {
	return c->remove_device_node(c, src_path, dest_path);
}

bool go_lxc_rename(struct lxc_container *c, const char *newname) {
	return c->rename(c, newname);
}

bool go_lxc_checkpoint(struct lxc_container *c, char *directory, bool stop, bool verbose) {
#if VERSION_AT_LEAST(1, 1, 0)
	return c->checkpoint(c, directory, stop, verbose);
#else
	return false;
#endif
}

bool go_lxc_restore(struct lxc_container *c, char *directory, bool verbose) {
#if VERSION_AT_LEAST(1, 1, 0)
	return c->restore(c, directory, verbose);
#else
	return false;
#endif
}

int go_lxc_migrate(struct lxc_container *c, unsigned int cmd, struct migrate_opts *opts, struct extra_migrate_opts *extras) {
#if VERSION_AT_LEAST(2, 0, 4)
	opts->action_script = extras->action_script;
	opts->ghost_limit = extras->ghost_limit;
#endif

#if VERSION_AT_LEAST(2, 0, 1)
	opts->preserves_inodes = extras->preserves_inodes;
#endif

#if VERSION_AT_LEAST(2, 0, 0)
	return c->migrate(c, cmd, opts, sizeof(*opts));
#else
	return -EINVAL;
#endif
}

bool go_lxc_attach_interface(struct lxc_container *c, const char *dev, const char *dst_dev) {
#if VERSION_AT_LEAST(1, 1, 0)
	return c->attach_interface(c, dev, dst_dev);
#else
	return false;
#endif
}

bool go_lxc_detach_interface(struct lxc_container *c, const char *dev, const char *dst_dev) {
#if VERSION_AT_LEAST(1, 1, 0)
	return c->detach_interface(c, dev, dst_dev);
#else
	return false;
#endif
}
