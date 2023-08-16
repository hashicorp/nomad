package main

import (
	"context"
	"log"
	"os"
	"os/exec"
	"time"
)

// gocat unix-to-tcp --src /secrets/api.sock --dst 0.0.0.0:4000
func gocat(ctx context.Context) error {
	var err error

	// if the socket doesn't exist at launch, assume we're not in
	// an identity{}-having nomad task.
	if _, err = os.Stat(apiSock); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	// we have an api socket.  lazily just run gocat to turn it into tcp.
	// talk to Charlie if you have thoughts about this. ;P
	tcp := "127.0.0.1:4000"
	if err = os.Setenv("NOMAD_ADDR", "http://"+tcp); err != nil {
		return err
	}

	// if the socket goes away (client goes down), we'll want to try to
	// re-establish the connection to it if/when it re-appears.
	go func() {
		timer := time.NewTimer(0) // run right away first time
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Println("gocat context done, exiting.")
				return
			case <-timer.C:
				timer.Reset(time.Second * 5) // then only try periodically
			}

			cmd := exec.CommandContext(ctx,
				"gocat", "unix-to-tcp", "--src", apiSock, "--dst", tcp)
			cmd.Stdout = log.Writer()
			cmd.Stderr = log.Writer()
			if err = cmd.Start(); err != nil {
				log.Printf("error starting gocat: %s", err)
				continue
			}
			err = cmd.Wait()
			log.Printf("gocat done; err: %s", err)
		}
	}()
	return nil
}
