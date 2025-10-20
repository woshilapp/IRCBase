package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/woshilapp/IRCBaseGo/server"
)

func main() {
	addr := ":8888"
	srv := server.New(addr)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := srv.Run(ctx); err != nil {
		log.Printf("server stopped: %v", err)
		os.Exit(1)
	}
}
