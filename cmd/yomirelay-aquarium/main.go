package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	root := flag.String("install", "", "Steam-discovered AQUARIUM installation (auto-detected when omitted)")
	udp := flag.String("udp", "127.0.0.1:17322", "YomiRelay loopback UDP address")
	gameName := flag.String("game-name", "AQUARIUM", "game name placed in dialogue events")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, *root, *udp, *gameName); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
