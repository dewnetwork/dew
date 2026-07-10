// Command dew is the Dew full node entrypoint.
//
// Phases A4–A7: JSON-RPC node, Dew-BFT, P2P, and local multi-validator devnet.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dewnetwork/dew/config"
	"github.com/dewnetwork/dew/devnet"
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
		fmt.Println("dew 0.1.0 (phase A7)")
		return nil
	case "help", "-h", "--help":
		printUsage()
		return nil
	case "run", "start":
		return cmdRun(args[1:])
	case "init":
		return cmdInit(args[1:])
	case "devnet":
		return cmdDevnet(args[1:])
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

func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	out := fs.String("out", "genesis.json", "output genesis path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	g, err := devnet.DefaultGenesis()
	if err != nil {
		return err
	}
	if err := devnet.WriteGenesisJSON(*out, g); err != nil {
		return err
	}
	fmt.Printf("wrote %s (chainId=%s, validators=%d)\n", *out, g.ChainID(), len(g.InitialValidators))
	fmt.Println("next: dew devnet   # or  dew run --genesis", *out)
	return nil
}

func cmdDevnet(args []string) error {
	fs := flag.NewFlagSet("devnet", flag.ContinueOnError)
	httpAddr := fs.String("http.addr", "127.0.0.1", "JSON-RPC bind address")
	httpPort := fs.Int("http.port", devnet.DefaultRPCPort, "JSON-RPC port")
	noP2P := fs.Bool("no-p2p", false, "disable loopback P2P mesh")
	bftHeights := fs.Int("bft.heights", 1, "BFT heights to commit on start (0=skip)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	netw, err := devnet.Start(devnet.NetworkConfig{
		HTTPAddr:  fmt.Sprintf("%s:%d", *httpAddr, *httpPort),
		EnableP2P: !*noP2P,
	})
	if err != nil {
		return err
	}
	defer func() { _ = netw.Stop() }()

	fmt.Print(netw.Info())
	if *bftHeights > 0 {
		evs, err := netw.CommitHeights(*bftHeights)
		if err != nil {
			return fmt.Errorf("bft: %w", err)
		}
		fmt.Printf("  bft:         committed %d height(s), last=%s\n", len(evs), evs[len(evs)-1].BlockHash.Hex())
	}
	fmt.Println()
	fmt.Println("MetaMask: add network chainId 2026, RPC", netw.RPCURL)
	fmt.Println("Foundry:  forge create … --rpc-url", netw.RPCURL, "--private-key", netw.Faucet.PrivHex)
	fmt.Println("Smoke:    node scripts/smoke-rpc.mjs", netw.RPCURL)
	fmt.Println("ERC-20:   node scripts/devnet-erc20.mjs", netw.RPCURL)
	fmt.Println()
	fmt.Println("listening — Ctrl+C to stop")

	// Optional: keep sealing empty BFT heights slowly for demo liveness.
	stop := make(chan struct{})
	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				_, _ = netw.CommitHeights(1)
			}
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	close(stop)
	fmt.Println("shutting down…")
	return nil
}

func waitSignal() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `dew — Dew full node

Usage:
  dew init [--out genesis.json]
  dew devnet [--http.addr 127.0.0.1] [--http.port 8545] [--no-p2p] [--bft.heights 1]
  dew run [--genesis genesis.json] [--http.addr 127.0.0.1] [--http.port 8545]
  dew version

`)
}
