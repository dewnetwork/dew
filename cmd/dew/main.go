// Command dew is the Dew full node entrypoint.
//
// Phases A4–A7 + C: JSON-RPC node, Dew-BFT, P2P, local multi-validator devnet,
// and optional multi-process P2P mesh for packaging.
package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/dewnetwork/dew/config"
	dewcrypto "github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/devnet"
	"github.com/dewnetwork/dew/node"
	"github.com/dewnetwork/dew/p2p"
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
		fmt.Println("dew 0.1.0 (public-testnet-v1)")
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
	staking := fs.Bool("staking", false, "enable live 0x102 staking methods (C4)")
	p2pListen := fs.String("p2p.listen", "", "if set, start P2P host (e.g. 0.0.0.0:30303)")
	p2pKeyHex := fs.String("p2p.key", "", "32-byte hex private key for P2P identity (required with --p2p.listen)")
	p2pBoot := fs.String("p2p.bootnodes", "", "comma-separated host:port peers to dial")
	p2pEncrypt := fs.Bool("p2p.encrypt", true, "encrypted P2P sessions (C2 default)")
	p2pCleartext := fs.Bool("p2p.allow-cleartext", false, "permit cleartext when --p2p.encrypt=false")
	dataDir := fs.String("datadir", node.DefaultDataDir, "data directory (default /var/lib/dew): chaindata/ Pebble + peers.json")
	validator := fs.Bool("validator", false, "enable Dew-BFT validator mode")
	validatorKey := fs.String("validator.key", "", "hex secp256k1 key for --validator")
	noAutoMine := fs.Bool("no-auto-mine", false, "admit txs to mempool only (no local seal)")
	minBlockInterval := fs.String("bft.min-block-interval", "", "min time between committed heights (Go duration; empty=default 1s; negative disables e.g. -1ns)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var minBlock time.Duration
	if s := strings.TrimSpace(*minBlockInterval); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil {
			return fmt.Errorf("bft.min-block-interval: %w", err)
		}
		minBlock = d
	}

	g, err := config.LoadGenesisFile(*genesisPath)
	if err != nil {
		return err
	}
	dir := strings.TrimSpace(*dataDir)
	if dir == "" {
		dir = node.DefaultDataDir
	}
	n, err := node.Open(g, node.ChainDataDir(dir))
	if err != nil {
		return err
	}
	defer func() { _ = n.Close() }()
	// Keep *dataDir consistent for P2P peers.json path.
	*dataDir = dir
	if *staking {
		n.SetStakingEnabled(true)
	}

	if *validator {
		*noAutoMine = true
	}
	if *noAutoMine {
		n.SetAutoMine(false)
	}

	if *validator {
		if *p2pListen == "" {
			return fmt.Errorf("validator mode requires --p2p.listen")
		}
		if strings.TrimSpace(*validatorKey) == "" {
			return fmt.Errorf("validator mode requires --validator.key")
		}
		valKey, err := parsePrivateKeyHex(*validatorKey)
		if err != nil {
			return fmt.Errorf("validator key: %w", err)
		}
		p2pKey, err := parsePrivateKeyHex(*p2pKeyHex)
		if err != nil {
			return fmt.Errorf("p2p key: %w", err)
		}
		return runBFTStack(bftRunConfig{
			Genesis:     g,
			Node:        n,
			HTTPAddr:    *httpAddr,
			HTTPPort:    *httpPort,
			HTTPEnabled: *httpEnabled,
			Stack: &node.StackConfig{
				Validator:        true,
				ValidatorKey:     valKey,
				P2PListen:        *p2pListen,
				P2PPrivateKey:    p2pKey,
				Bootnodes:        splitCSV(*p2pBoot),
				Encrypt:          *p2pEncrypt,
				AllowCleartext:   *p2pCleartext,
				DataDir:          *dataDir,
				MinBlockInterval: minBlock,
			},
		})
	}

	if *noAutoMine && *p2pListen != "" {
		p2pKey, err := parsePrivateKeyHex(*p2pKeyHex)
		if err != nil {
			return fmt.Errorf("p2p key: %w", err)
		}
		return runBFTStack(bftRunConfig{
			Genesis:     g,
			Node:        n,
			HTTPAddr:    *httpAddr,
			HTTPPort:    *httpPort,
			HTTPEnabled: *httpEnabled,
			Stack: &node.StackConfig{
				Validator:        false,
				P2PListen:        *p2pListen,
				P2PPrivateKey:    p2pKey,
				Bootnodes:        splitCSV(*p2pBoot),
				Encrypt:          *p2pEncrypt,
				AllowCleartext:   *p2pCleartext,
				DataDir:          *dataDir,
				MinBlockInterval: minBlock,
			},
		})
	}

	var p2pHost *p2p.Host
	if *p2pListen != "" {
		h, err := startP2PHost(n, *p2pListen, *p2pKeyHex, *p2pBoot, *p2pEncrypt, *p2pCleartext, *dataDir)
		if err != nil {
			return err
		}
		p2pHost = h
		defer func() { _ = p2pHost.Close() }()
	}

	fmt.Printf("Dew node started chainId=%s head=%d staking=%v\n", n.ChainID().String(), n.BlockNumber(), n.StakingEnabled())
	if p2pHost != nil {
		fmt.Printf("  p2p:         listen=%s encrypt=%v peers=%d\n", p2pHost.ListenAddr(), p2pHost.EncryptEnabled(), len(p2pHost.Store().Active()))
	}
	fmt.Println("hint: launch checklist — docs/ops/launch-checklist.md")
	fmt.Println("hint: packaging — deploy/README.md")

	if !*httpEnabled {
		waitSignal()
		return nil
	}

	return serveHTTP(n, nil, *httpAddr, *httpPort)
}

// startP2PHost binds an encrypted (default) P2P listener and dials bootnodes.
// Uses a MemoryChain snapshot of genesis tip — multi-process BFT production is residual.
func startP2PHost(n *node.Node, listen, keyHex, bootCSV string, encrypt, allowCleartext bool, dataDir string) (*p2p.Host, error) {
	keyHex = strings.TrimPrefix(strings.TrimSpace(keyHex), "0x")
	if keyHex == "" {
		return nil, fmt.Errorf("p2p: --p2p.key required when --p2p.listen is set")
	}
	raw, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("p2p: key hex: %w", err)
	}
	priv, err := dewcrypto.ToECDSA(raw)
	if err != nil {
		return nil, fmt.Errorf("p2p: key: %w", err)
	}

	h := n.CurrentHeader()
	if h == nil {
		return nil, fmt.Errorf("p2p: missing genesis header")
	}
	mc := p2p.NewMemoryChain(h.Hash(), []byte("genesis"))
	boot := splitCSV(bootCSV)
	peerPath := ""
	if strings.TrimSpace(dataDir) != "" {
		peerPath = filepath.Join(dataDir, "peers.json")
	}
	cfg := p2p.Config{
		PrivateKey:     priv,
		ChainID:        n.ChainID(),
		ListenAddr:     listen,
		MaxPeers:       25,
		Encrypt:        encrypt,
		AllowCleartext: allowCleartext,
		PeerStorePath:  peerPath,
		Bootnodes:      boot,
	}
	host, err := p2p.NewHost(cfg, mc, mc, p2p.AppHandlers{})
	if err != nil {
		return nil, err
	}
	if err := host.Start(); err != nil {
		return nil, err
	}

	// One-shot retries still help when peers start out of order (compose).
	// Continuous redial is owned by Host (D3b).
	for _, addr := range boot {
		addr := strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		go dialWithRetry(host, addr, 15, time.Second)
	}
	// Brief settle so Active() is non-empty when peers are already up.
	time.Sleep(100 * time.Millisecond)
	return host, nil
}

func dialWithRetry(h *p2p.Host, addr string, attempts int, gap time.Duration) {
	for i := 0; i < attempts; i++ {
		if _, err := h.Dial(addr); err == nil {
			fmt.Printf("  p2p:         dialed %s\n", addr)
			return
		}
		time.Sleep(gap)
	}
	fmt.Fprintf(os.Stderr, "dew: p2p dial %s failed after %d attempts\n", addr, attempts)
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return strings.Split(s, ",")
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
	fmt.Println("MetaMask: add network chainId 2205, RPC", netw.RPCURL)
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
  dew run [--genesis genesis.json] [--datadir /var/lib/dew] [--http.addr 127.0.0.1] [--http.port 8545] [--staking]
          [--validator] [--validator.key HEX] [--no-auto-mine]
          [--p2p.listen host:port] [--p2p.key HEX] [--p2p.bootnodes a:port,b:port]
          [--p2p.encrypt] [--p2p.allow-cleartext]
  dew version

Launch checklist: docs/ops/launch-checklist.md
Packaging:        deploy/README.md
`)
}
