package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/chrissnell/sparkyfish/pkg/server"
)

var version = "dev"

func main() {
	if len(os.Args) >= 2 && (os.Args[1] == "-v" || os.Args[1] == "--version") {
		fmt.Println("sparkyfish-server", version)
		os.Exit(0)
	}

	cfg := server.Config{}
	flag.StringVar(&cfg.ListenAddr, "listen-addr", ":7121", "IP:Port to listen on")
	flag.StringVar(&cfg.Cname, "cname", "", "Canonical hostname reported to clients")
	flag.StringVar(&cfg.Location, "location", "", "Physical location of server")
	flag.BoolVar(&cfg.Debug, "debug", false, "Enable verbose logging")
	flag.Parse()

	srv, err := server.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := srv.ListenAndServe(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
