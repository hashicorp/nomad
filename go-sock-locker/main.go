package main

/* Notes
* levels of user app awareness of locking:
  * use our go sdk in your application (as this thing does)
  * use a sidecar like this thing, and your app just checks a file (or env if template->restart)
  * nomad manages the lifecycle of your main task automatically, somehow...
*/

/* TODO
* bug: occasional flapping? -- I think due to the TTL grace period not actually grace-ing
* bug: lock TTL does not (always) get reached?
* feat: different TTLs -- feed through an api.VariableLock{} instead of setting on controller
* bug: file can get left behind on SIGKILL -- "work" job can delete the file after looking at it.
  ^ this is avoided by using template{} instead of this sidecar's alloc/lock file...
*/

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/api"
)

func main() {
	hclog.DefaultLevel = hclog.Debug

	// set up a tcp listener on the socket if relevant. hiiii Charlie!
	// it has a separate context so auto-release can occur.
	goCatCtx, goCatCancel := context.WithCancel(context.Background())
	defer goCatCancel()
	if err := gocat(goCatCtx); err != nil {
		panic(err)
	}

	// this context can be canceled to stop everything else.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// handle signals
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill, syscall.SIGHUP, syscall.SIGABRT, syscall.SIGTERM) // term seems to be nomad default
	go func() {
		s := <-sig
		log.Printf("got signal: %s", s)
		cancel()
	}()

	// nomad api client
	conf := api.DefaultConfig()
	c, err := api.NewClient(conf)
	if err != nil {
		panic(err)
	}
	defer c.Close()
	log.Println("nomad addr:", c.Address())

	var wg sync.WaitGroup

	// lock and auto-renew
	l := &Lock{client: c}
	logger := hclog.New(&hclog.LoggerOptions{Name: "sock-locker"})
	controller := NewHALockController(l, logger, time.Second*15)
	protectMe := func(inCtx context.Context) {
		wg.Add(1)
		defer wg.Done()

		logger.Info("will write to file", "f", lockFile)
		defer deleteFile()
		for {
			select {
			case <-ctx.Done():
				log.Println("all done!")
				return
			case <-inCtx.Done():
				log.Println("inner done!")
				return
			case <-time.After(time.Second * 5):
				log.Println("it me; i am active.")
				writeFile()
			}
		}
	}

	// TODO: make sure this bug gets fixed in nomad:
	// http: error authenticating built API request: error="allocation is terminal" url="/v1/var/locker/lock?lock-release=&namespace=default&region=global" method=PUT
	// needs alloc.ClientTerminalStatus() instead of alloc.TerminalStatus() in nomad/acl.go VerifyClaim()
	// this happens after "all done!" and "removing /alloc/lock"
	defer func() {
		// TODO: only if we do have the lock...
		log.Println("we're DONE here. releasing lock.")
		if err := l.Release(ctx); err != nil {
			log.Println("error releasing:", err)
		}
	}()

	// start the lock controller. (blocks)
	err = controller.Start(ctx, protectMe)
	if err != nil {
		panic(err)
	}

	wg.Wait()
}

func writeFile() {
	//log.Printf("writing file: %s; id: %s", lockFile, allocID)
	f, err := os.OpenFile(lockFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Printf("error opening %s: %s", lockFile, err)
	}
	_, err = f.Write([]byte(allocID))
	if err != nil {
		log.Printf("error writing alloc id: %s", err)
	}
}

func deleteFile() {
	log.Printf("removing %s", lockFile)
	if err := os.Remove(lockFile); err != nil {
		if !os.IsNotExist(err) {
			log.Printf("error removing lockfile: %s", err)
		}
	}
}
