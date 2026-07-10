// Command dew is the Dew full node entrypoint.
//
// Phase A4: load genesis, serve Ethereum JSON-RPC over HTTP (default :8545).
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/dewnetwork/dew/config"
	"github.com/dewnetwork/dew/node"
	"github.com/dewnetwork/dew/rpc"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "dew: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return fmt.Errorf("missing command")
	}
	switch args[0] {
	case "version":
		fmt.Println("dew 0.1.0 (phase A4)")
		return nil
	case "help", "-h", "--help":
		printUsage()
		return nil
	case "run", "start":
		return cmdRun(args[1:])
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func cmdRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	genesisPath := fs.String("genesis", "genesis.json", "path to genesis.json")
	httpAddr := fs.String("http.addr", "127.0.0.1", "JSON-RPC HTTP bind address")
	httpPort := fs.Int("http.port", 8545, "JSON-RPC HTTP port")
	httpEnabled := fs.Bool("http", true, "enable JSON-RPC HTTP")
	if err := fs.Parse(args); err != nil {
		return err
	}

	g, err := config.LoadGenesisFile(*genesisPath)
	if err != nil {
		return err
	}
	n, err := node.NewFromGenesis(g)
	if err != nil {
		return err
	}

	fmt.Printf("Dew node started chainId=%s head=%d\n", n.ChainID().String(), n.BlockNumber())

	if !*httpEnabled {
		waitSignal()
		return nil
	}

	srv := rpc.NewServer()
	api := rpc.NewAPI(n)
	srv.RegisterAll(api.Handlers())

	addr := fmt.Sprintf("%s:%d", *httpAddr, *httpPort)
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe(addr)
	}()

	// Wait for interrupt or server error
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sig:
		fmt.Println("shutting down…")
		_ = srv.Close()
		return nil
	case err := <-errCh:
		return err
	}
}

func waitSignal() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `dew — Dew full node

Usage:
  dew run [--genesis genesis.json] [--http.addr 127.0.0.1] [--http.port 8545]
  dew version

`)
}
