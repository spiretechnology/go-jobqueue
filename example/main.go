package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/spiretechnology/go-jobqueue"
)

type MyJob struct {
	ID   int
	Name string
}

func main() {
	// Root context for the program
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Create a manager
	manager := jobqueue.New[MyJob, int](
		&MyJobQueue{},
		&MyJobRunner{},
	)

	// Run the manager
	if err := manager.Run(ctx); err != nil {
		fmt.Println(err.Error())
	}
}
