// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TODO(tgross): almost everything in this file is lifted directly from the
// stdlib's os/exec/exec.go and syscall/exec_windows.go, stripped down to remove
// non-Windows bits, some legacy cruft from upstream, and methods we don't care
// about here. This gives us the StdinPipe and CombinedOutput methods we want,
// but adds the ProcThreadAttributeList which we need for running Windows
// applications in AppContainers. Ideally we'd get this feature upstreamed and
// then we could remove this package entirely. A similar proposal was rejected
// in https://github.com/golang/go/issues/44005 but hopefully using this package
// as example of the lift involved we can advocate for getting the issue
// reconsidered.

//go:build windows

package winexec

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
)

type Cmd struct {
	*exec.Cmd

	// these are all private fields of exec.Cmd that we're hoisting into this
	// struct so that we can access them in the methods we're implementing
	ctx           context.Context
	childIOFiles  []io.Closer
	parentIOPipes []io.Closer
	goroutine     []func() error
	goroutineErr  <-chan error
	ctxResult     <-chan ctxResult

	// ProcThreadAttributes will get merged with the one that gets created
	// automatically for StartupInfoEx
	ProcThreadAttributes []ProcThreadAttribute
}

// A ctxResult reports the result of watching the Context associated with a
// running command (and sending corresponding signals if needed).
// This is lifted from os/exec/exec.go
type ctxResult struct {
	err   error
	timer *time.Timer
}

// CommandContext returns a new Cmd with a given context. Note we return the
// concrete struct and not an interface here because callers need to update
// fields on the inner exec.Cmd directly
func CommandContext(ctx context.Context, name string, arg ...string) *Cmd {
	if ctx == nil {
		panic("nil Context")
	}
	innerCmd := exec.Command(name, arg...)

	cmd := &Cmd{}
	cmd.Cmd = innerCmd
	cmd.ctx = ctx
	cmd.Cancel = func() error {
		return cmd.Process.Kill()
	}

	if filepath.Base(name) == name {
		lp, err := exec.LookPath(name)
		if lp != "" {
			cmd.Path = lp
		}
		if err != nil {
			cmd.Err = err
		}
	}

	return cmd
}

func (c *Cmd) StdinPipe() (io.WriteCloser, error) {
	if c.Stdin != nil {
		return nil, errors.New("exec: Stdin already set")
	}
	if c.Process != nil {
		return nil, errors.New("exec: StdinPipe after process started")
	}
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	c.Stdin = pr
	c.childIOFiles = append(c.childIOFiles, pr)
	c.parentIOPipes = append(c.parentIOPipes, pw)
	return pw, nil
}

func (c *Cmd) CombinedOutput() ([]byte, error) {
	if c.Stdout != nil {
		return nil, errors.New("exec: Stdout already set")
	}
	if c.Stderr != nil {
		return nil, errors.New("exec: Stderr already set")
	}
	var b bytes.Buffer
	c.Stdout = &b
	c.Stderr = &b
	err := c.Run()
	return b.Bytes(), err
}

func (c *Cmd) Run() error {
	err := c.Start()
	if err != nil {
		return err
	}
	return c.Wait()
}

func (c *Cmd) Start() error {

	if c.Process != nil {
		return errors.New("exec: already started")
	}

	started := false
	defer func() {
		closeDescriptors(c.childIOFiles)
		c.childIOFiles = nil

		if !started {
			closeDescriptors(c.parentIOPipes)
			c.parentIOPipes = nil
		}
	}()

	if c.Path == "" && c.Err == nil {
		c.Err = errors.New("exec: no command")
	}
	if c.Err != nil {
		return c.Err
	}

	if c.Cancel != nil && c.ctx == nil {
		return errors.New("exec: command with a non-nil Cancel was not created with CommandContext")
	}
	if c.ctx != nil {
		select {
		case <-c.ctx.Done():
			return c.ctx.Err()
		default:
		}
	}

	childFiles := make([]*os.File, 0, 3)

	stdin, err := c.childStdin()
	if err != nil {
		return err
	}
	childFiles = append(childFiles, stdin)

	stdout, err := c.childStdout()
	if err != nil {
		return err
	}
	childFiles = append(childFiles, stdout)

	stderr, err := c.childStderr(stdout)
	if err != nil {
		return err
	}
	childFiles = append(childFiles, stderr)

	env, err := c.environ()
	if err != nil {
		return err
	}

	attr := &syscall.ProcAttr{
		Dir: c.Dir,
		Files: []uintptr{
			childFiles[0].Fd(),
			childFiles[1].Fd(),
			childFiles[2].Fd(),
		},
		Env: env,
		Sys: c.SysProcAttr,
	}

	c.Process, err = c.createProcess(c.Path, c.Args, c.ProcThreadAttributes, attr)
	if err != nil {
		return err
	}
	started = true

	if len(c.goroutine) > 0 {
		goroutineErr := make(chan error, 1)
		c.goroutineErr = goroutineErr

		type goroutineStatus struct {
			running  int
			firstErr error
		}
		statusc := make(chan goroutineStatus, 1)
		statusc <- goroutineStatus{running: len(c.goroutine)}
		for _, fn := range c.goroutine {
			go func(fn func() error) {
				err := fn()
				status := <-statusc
				if status.firstErr == nil {
					status.firstErr = err
				}
				status.running--
				if status.running == 0 {
					goroutineErr <- status.firstErr
				} else {
					statusc <- status
				}
			}(fn)
		}
		c.goroutine = nil
	}

	if (c.Cancel != nil || c.WaitDelay != 0) && c.ctx != nil && c.ctx.Done() != nil {
		resultc := make(chan ctxResult)
		c.ctxResult = resultc
		go c.watchCtx(resultc)
	}

	return nil
}

func (c *Cmd) environ() ([]string, error) {
	var err error
	env := c.Env
	if env == nil {
		return os.Environ(), nil
	}
	env, dedupErr := dedupEnv(env)
	if err == nil {
		err = dedupErr
	}
	return addCriticalEnv(env), nil
}

// dedupEnv returns a copy of env with any duplicates removed, in favor of
// later values.
// Items not of the normal environment "key=value" form are preserved unchanged.
// Except on Plan 9, items containing NUL characters are removed, and
// an error is returned along with the remaining values.
func dedupEnv(env []string) ([]string, error) {
	return dedupEnvCase(true, false, env)
}

// dedupEnvCase is dedupEnv with a case option for testing.
// If caseInsensitive is true, the case of keys is ignored.
// If nulOK is false, items containing NUL characters are allowed.
func dedupEnvCase(caseInsensitive, nulOK bool, env []string) ([]string, error) {
	// Construct the output in reverse order, to preserve the
	// last occurrence of each key.
	var err error
	out := make([]string, 0, len(env))
	saw := make(map[string]bool, len(env))
	for n := len(env); n > 0; n-- {
		kv := env[n-1]

		// Reject NUL in environment variables to prevent security issues (#56284);
		// except on Plan 9, which uses NUL as os.PathListSeparator (#56544).
		if !nulOK && strings.IndexByte(kv, 0) != -1 {
			err = errors.New("exec: environment variable contains NUL")
			continue
		}

		i := strings.Index(kv, "=")
		if i == 0 {
			// We observe in practice keys with a single leading "=" on Windows.
			// TODO(#49886): Should we consume only the first leading "=" as part
			// of the key, or parse through arbitrarily many of them until a non-"="?
			i = strings.Index(kv[1:], "=") + 1
		}
		if i < 0 {
			if kv != "" {
				// The entry is not of the form "key=value" (as it is required to be).
				// Leave it as-is for now.
				// TODO(#52436): should we strip or reject these bogus entries?
				out = append(out, kv)
			}
			continue
		}
		k := kv[:i]
		if caseInsensitive {
			k = strings.ToLower(k)
		}
		if saw[k] {
			continue
		}

		saw[k] = true
		out = append(out, kv)
	}

	// Now reverse the slice to restore the original order.
	for i := 0; i < len(out)/2; i++ {
		j := len(out) - i - 1
		out[i], out[j] = out[j], out[i]
	}

	return out, err
}

func addCriticalEnv(env []string) []string {
	if runtime.GOOS != "windows" {
		return env
	}
	for _, kv := range env {
		k, _, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		if strings.EqualFold(k, "SYSTEMROOT") {
			// We already have it.
			return env
		}
	}
	return append(env, "SYSTEMROOT="+os.Getenv("SYSTEMROOT"))
}

func (c *Cmd) watchCtx(resultc chan<- ctxResult) {
	select {
	case resultc <- ctxResult{}:
		return
	case <-c.ctx.Done():
	}

	var err error
	if c.Cancel != nil {
		if interruptErr := c.Cancel(); interruptErr == nil {
			// We appear to have successfully interrupted the command, so any
			// program behavior from this point may be due to ctx even if the
			// command exits with code 0.
			err = c.ctx.Err()
		} else if errors.Is(interruptErr, os.ErrProcessDone) {
			// The process already finished: we just didn't notice it yet.
			// (Perhaps c.Wait hadn't been called, or perhaps it happened to race with
			// c.ctx being cancelled.) Don't inject a needless error.
		} else {
			err = wrappedError{
				prefix: "exec: canceling Cmd",
				err:    interruptErr,
			}
		}
	}
	if c.WaitDelay == 0 {
		resultc <- ctxResult{err: err}
		return
	}

	timer := time.NewTimer(c.WaitDelay)
	select {
	case resultc <- ctxResult{err: err, timer: timer}:
		// c.Process.Wait returned and we've handed the timer off to c.Wait.
		// It will take care of goroutine shutdown from here.
		return
	case <-timer.C:
	}

	killed := false
	if killErr := c.Process.Kill(); killErr == nil {
		// We appear to have killed the process. c.Process.Wait should return a
		// non-nil error to c.Wait unless the Kill signal races with a successful
		// exit, and if that does happen we shouldn't report a spurious error,
		// so don't set err to anything here.
		killed = true
	} else if !errors.Is(killErr, os.ErrProcessDone) {
		err = wrappedError{
			prefix: "exec: killing Cmd",
			err:    killErr,
		}
	}

	if c.goroutineErr != nil {
		select {
		case goroutineErr := <-c.goroutineErr:
			// Forward goroutineErr only if we don't have reason to believe it was
			// caused by a call to Cancel or Kill above.
			if err == nil && !killed {
				err = goroutineErr
			}
		default:
			// Close the child process's I/O pipes, in case it abandoned some
			// subprocess that inherited them and is still holding them open
			// (see https://go.dev/issue/23019).
			//
			// We close the goroutine pipes only after we have sent any signals we're
			// going to send to the process (via Signal or Kill above): if we send
			// SIGKILL to the process, we would prefer for it to die of SIGKILL, not
			// SIGPIPE. (However, this may still cause any orphaned subprocesses to
			// terminate with SIGPIPE.)
			closeDescriptors(c.parentIOPipes)
			// Wait for the copying goroutines to finish, but report ErrWaitDelay for
			// the error: any other error here could result from closing the pipes.
			_ = <-c.goroutineErr
			if err == nil {
				err = ErrWaitDelay
			}
		}

		// Since we have already received the only result from c.goroutineErr,
		// set it to nil to prevent awaitGoroutines from blocking on it.
		c.goroutineErr = nil
	}

	resultc <- ctxResult{err: err}
}

// ErrWaitDelay is returned by (*Cmd).Wait if the process exits with a
// successful status code but its output pipes are not closed before the
// command's WaitDelay expires.
var ErrWaitDelay = errors.New("exec: WaitDelay expired before I/O complete")

// wrappedError wraps an error without relying on fmt.Errorf.
type wrappedError struct {
	prefix string
	err    error
}

func (w wrappedError) Error() string {
	return w.prefix + ": " + w.err.Error()
}

func (w wrappedError) Unwrap() error {
	return w.err
}

func (c *Cmd) Wait() error {
	if c.Process == nil {
		return errors.New("exec: not started")
	}
	if c.ProcessState != nil {
		return errors.New("exec: Wait was already called")
	}

	state, err := c.Process.Wait()
	if err == nil && !state.Success() {
		err = &exec.ExitError{ProcessState: state}
	}
	c.ProcessState = state

	var timer *time.Timer
	if c.ctxResult != nil {
		watch := <-c.ctxResult
		timer = watch.timer
		// If c.Process.Wait returned an error, prefer that.
		// Otherwise, report any error from the watchCtx goroutine,
		// such as a Context cancellation or a WaitDelay overrun.
		if err == nil && watch.err != nil {
			err = watch.err
		}
	}

	if goroutineErr := c.awaitGoroutines(timer); err == nil {
		// Report an error from the copying goroutines only if the program
		// otherwise exited normally on its own. Otherwise, the copying error
		// may be due to the abnormal termination.
		err = goroutineErr
	}
	closeDescriptors(c.parentIOPipes)
	c.parentIOPipes = nil

	return err
}

func (c *Cmd) awaitGoroutines(timer *time.Timer) error {
	defer func() {
		if timer != nil {
			timer.Stop()
		}
		c.goroutineErr = nil
	}()

	if c.goroutineErr == nil {
		return nil // No running goroutines to await.
	}

	if timer == nil {
		if c.WaitDelay == 0 {
			return <-c.goroutineErr
		}

		select {
		case err := <-c.goroutineErr:
			// Avoid the overhead of starting a timer.
			return err
		default:
		}

		// No existing timer was started: either there is no Context associated with
		// the command, or c.Process.Wait completed before the Context was done.
		timer = time.NewTimer(c.WaitDelay)
	}

	select {
	case <-timer.C:
		closeDescriptors(c.parentIOPipes)
		// Wait for the copying goroutines to finish, but ignore any error
		// (since it was probably caused by closing the pipes).
		_ = <-c.goroutineErr
		return ErrWaitDelay

	case err := <-c.goroutineErr:
		return err
	}
}

func closeDescriptors(closers []io.Closer) {
	for _, fd := range closers {
		fd.Close()
	}
}

func (c *Cmd) childStdin() (*os.File, error) {
	if c.Stdin == nil {
		f, err := os.Open(os.DevNull)
		if err != nil {
			return nil, err
		}
		c.childIOFiles = append(c.childIOFiles, f)
		return f, nil
	}

	if f, ok := c.Stdin.(*os.File); ok {
		return f, nil
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	c.childIOFiles = append(c.childIOFiles, pr)
	c.parentIOPipes = append(c.parentIOPipes, pw)
	c.goroutine = append(c.goroutine, func() error {
		_, err := io.Copy(pw, c.Stdin)
		if skipStdinCopyError(err) {
			err = nil
		}
		if err1 := pw.Close(); err == nil {
			err = err1
		}
		return err
	})
	return pr, nil
}

func (c *Cmd) childStdout() (*os.File, error) {
	return c.writerDescriptor(c.Stdout)
}

func (c *Cmd) childStderr(childStdout *os.File) (*os.File, error) {
	if c.Stderr != nil && interfaceEqual(c.Stderr, c.Stdout) {
		return childStdout, nil
	}
	return c.writerDescriptor(c.Stderr)
}

func (c *Cmd) writerDescriptor(w io.Writer) (*os.File, error) {
	if w == nil {
		f, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err != nil {
			return nil, err
		}
		c.childIOFiles = append(c.childIOFiles, f)
		return f, nil
	}

	if f, ok := w.(*os.File); ok {
		return f, nil
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	c.childIOFiles = append(c.childIOFiles, pw)
	c.parentIOPipes = append(c.parentIOPipes, pr)
	c.goroutine = append(c.goroutine, func() error {
		_, err := io.Copy(w, pr)
		pr.Close() // in case io.Copy stopped due to write error
		return err
	})
	return pw, nil
}

// interfaceEqual protects against panics from doing equality tests on
// two interfaces with non-comparable underlying types.
func interfaceEqual(a, b any) bool {
	defer func() {
		recover()
	}()
	return a == b
}

func skipStdinCopyError(err error) bool {
	// Ignore ERROR_BROKEN_PIPE and ERROR_NO_DATA errors copying
	// to stdin if the program completed successfully otherwise.
	// See Issue 20445.
	const _ERROR_NO_DATA = syscall.Errno(0xe8)
	pe, ok := err.(*fs.PathError)
	return ok &&
		pe.Op == "write" && pe.Path == "|1" &&
		(pe.Err == syscall.ERROR_BROKEN_PIPE || pe.Err == _ERROR_NO_DATA)
}

// createEnvBlock converts an array of environment strings into
// the representation required by CreateProcess: a sequence of NUL
// terminated strings followed by a nil.
// Last bytes are two UCS-2 NULs, or four NUL bytes.
// If any string contains a NUL, it returns (nil, EINVAL).
func createEnvBlock(envv []string) (*uint16, error) {
	if len(envv) == 0 {
		return &utf16.Encode([]rune("\x00\x00"))[0], nil
	}
	length := 0
	for _, s := range envv {
		if IndexByteString(s, 0) != -1 {
			return nil, EINVAL
		}
		length += len(s) + 1
	}
	length += 1

	b := make([]byte, length)
	i := 0
	for _, s := range envv {
		l := len(s)
		copy(b[i:i+l], []byte(s))
		copy(b[i+l:i+l+1], []byte{0})
		i = i + l + 1
	}
	copy(b[i:i+1], []byte{0})

	return &utf16.Encode([]rune(string(b)))[0], nil
}

func IndexByteString(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
