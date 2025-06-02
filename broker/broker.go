package main

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"syscall"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered from panic:", r)
		}
	}()
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	qbroker_start(ctx)
	wsB := NewWSBroker(ctx)
	wsB.Start()
	<-ctx.Done()
	log.Printf("Shutdown Server...")
	qbroker_stop()
	wsB.Stop()
}
