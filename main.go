package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/otterlab-bio/otter/cmd"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	if err := cmd.ExecuteContext(ctx); err != nil {
		log.Print(err)
		os.Exit(cmd.ExitCode(err))
	}
}
