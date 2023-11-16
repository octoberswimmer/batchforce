package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/octoberswimmer/batchforce/cmd/batchforce/cmd"
	log "github.com/sirupsen/logrus"
)

func main() {
	Execute()
}

func Execute() {
	ctx, cancel := context.WithCancel(context.Background())
	cancelUponSignal(cancel)
	if err := cmd.RootCmd.ExecuteContext(ctx); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func cancelUponSignal(cancel context.CancelFunc) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		interuptsReceived := 0
		for {
			<-sigs
			if interuptsReceived > 0 {
				os.Exit(1)
			}
			log.Warnln("signal received.  cancelling.")
			cancel()
			interuptsReceived++
		}
	}()
}
