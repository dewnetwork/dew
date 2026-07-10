package devnet

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/dewnetwork/dew/config"
	"github.com/dewnetwork/dew/consensus"
	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/node"
	"github.com/dewnetwork/dew/p2p"
	"github.com/dewnetwork/dew/rpc"
)

// Network is a local Phase A7 devnet: 3 Dew-BFT validators + 1 JSON-RPC node.
//
// Topology (in-process):
//
//	validator-0..2  — consensus.LocalCluster (BFT commits)
//	p2p hosts       — one per validator (+ optional RPC host) on loopback
//	RPC node        — execution backend (auto-mine per tx) on HTTP
//
// Execution (MetaMask / Foundry / ERC-20) uses the RPC node. BFT and P2P prove
// multi-validator networking; full block production via BFT→execute is wired
// further as the node gains ImportBlock in later iterations.
type Network struct {
	Genesis *config.Genesis
	Node    *node.Node
	RPC     *rpc.Server
	RPCURL  string

	Validators []Account
	Faucet     Account
	User1      Account

	Cluster *consensus.LocalCluster
	Hosts   []*p2p.Host // len = validators (+ optional rpc host)

	mu      sync.Mutex
	httpSrv *http.Server
	ln      net.Listener
}

// NetworkConfig configures Start.
type NetworkConfig struct {
	// HTTPAddr bind for JSON-RPC (default 127.0.0.1:8545). Use :0 for ephemeral.
	HTTPAddr string
	// Genesis overrides; nil → DefaultGenesis().
	Genesis *config.Genesis
	// EnableP2P dials validator hosts in a mesh (default true).
	EnableP2P bool
}

// Start boots validators, optional P2P mesh, and the RPC HTTP server.
func Start(cfg NetworkConfig) (*Network, error) {
	g := cfg.Genesis
	var err error
	if g == nil {
		g, err = DefaultGenesis()
		if err != nil {
			return nil, err
		}
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = fmt.Sprintf("127.0.0.1:%d", DefaultRPCPort)
	}

	n, err := node.NewFromGenesis(g)
	if err != nil {
		return nil, fmt.Errorf("devnet: node: %w", err)
	}

	vals := DefaultValidators()
	faucet := Faucet()
	user1 := User1()

	// Consensus: 3-validator local BFT from genesis header.
	parent, err := genesisHeader(g, n)
	if err != nil {
		return nil, err
	}
	root := parent.StateRoot
	nodes := make([]consensus.LocalNode, len(vals))
	for i, v := range vals {
		nodes[i] = consensus.LocalNode{
			Key:         v.PrivateKey,
			Power:       1,
			ProposeRoot: root,
			Validator:   consensus.NewRootValidator(root),
		}
	}
	cluster, err := consensus.NewLocalCluster(parent, nodes)
	if err != nil {
		return nil, fmt.Errorf("devnet: bft cluster: %w", err)
	}

	netw := &Network{
		Genesis:    g,
		Node:       n,
		Validators: vals,
		Faucet:     faucet,
		User1:      user1,
		Cluster:    cluster,
	}

	// P2P: one host per validator with shared memory chain tip from genesis.
	if cfg.EnableP2P {
		if err := netw.startP2P(); err != nil {
			return nil, err
		}
	}

	// RPC HTTP
	srv := rpc.NewServer()
	srv.RegisterAll(rpc.NewAPI(n).Handlers())
	netw.RPC = srv

	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		_ = netw.Stop()
		return nil, fmt.Errorf("devnet: listen rpc: %w", err)
	}
	netw.ln = ln
	netw.RPCURL = "http://" + ln.Addr().String()
	httpSrv := &http.Server{Handler: srv}
	netw.httpSrv = httpSrv
	go func() { _ = httpSrv.Serve(ln) }()

	// Brief settle for P2P dials
	if cfg.EnableP2P {
		time.Sleep(50 * time.Millisecond)
	}
	return netw, nil
}

func genesisHeader(g *config.Genesis, n *node.Node) (*types.Header, error) {
	// Use the live node head (genesis after NewFromGenesis).
	h := n.CurrentHeader()
	if h == nil {
		return nil, fmt.Errorf("devnet: missing genesis header")
	}
	return h, nil
}

func (n *Network) startP2P() error {
	chainID := n.Genesis.ChainID()
	// Shared tip view for handshake heights (static genesis for A7 mesh demo).
	genesisHash := n.Node.CurrentHeader().Hash()
	raw := []byte("genesis")
	n.Hosts = make([]*p2p.Host, 0, len(n.Validators))

	for i, v := range n.Validators {
		mc := p2p.NewMemoryChain(genesisHash, raw)
		h, err := p2p.NewHost(p2p.Config{
			PrivateKey: v.PrivateKey,
			ChainID:    chainID,
			ListenAddr: "127.0.0.1:0",
			MaxPeers:   10,
		}, mc, mc, p2p.AppHandlers{})
		if err != nil {
			return err
		}
		if err := h.Start(); err != nil {
			return err
		}
		n.Hosts = append(n.Hosts, h)
		_ = i
	}
	// Mesh: each dials the previous
	for i := 1; i < len(n.Hosts); i++ {
		if _, err := n.Hosts[i].Dial(n.Hosts[0].ListenAddr()); err != nil {
			return fmt.Errorf("devnet: p2p dial %d→0: %w", i, err)
		}
	}
	if len(n.Hosts) > 2 {
		if _, err := n.Hosts[2].Dial(n.Hosts[1].ListenAddr()); err != nil {
			return fmt.Errorf("devnet: p2p dial 2→1: %w", err)
		}
	}
	return nil
}

// CommitHeights runs k BFT heights on the 3-validator cluster.
func (n *Network) CommitHeights(k int) ([]*consensus.CommitEvent, error) {
	return n.Cluster.RunHeights(k)
}

// Stop shuts down RPC and P2P hosts.
func (n *Network) Stop() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.httpSrv != nil {
		_ = n.httpSrv.Close()
		n.httpSrv = nil
	}
	if n.ln != nil {
		_ = n.ln.Close()
		n.ln = nil
	}
	for _, h := range n.Hosts {
		_ = h.Close()
	}
	n.Hosts = nil
	return nil
}

// ValidatorCount returns the number of BFT validators.
func (n *Network) ValidatorCount() int { return len(n.Validators) }

// P2PPeerCount returns total active P2P sessions across validator hosts.
func (n *Network) P2PPeerCount() int {
	total := 0
	for _, h := range n.Hosts {
		total += h.PeerCount()
	}
	return total
}

// Info is a human-readable summary for CLI.
func (n *Network) Info() string {
	return fmt.Sprintf(
		`Dew local devnet (Phase A7)
  chainId:     %s (0x%x)
  rpc:         %s
  validators:  %d (BFT LocalCluster)
  p2p hosts:   %d (loopback mesh)
  faucet:      %s
  faucet key:  %s  (Anvil #0 — DEV ONLY)
  user1:       %s
`,
		n.Genesis.ChainID().String(),
		n.Genesis.ChainID().Int64(),
		n.RPCURL,
		n.ValidatorCount(),
		len(n.Hosts),
		n.Faucet.Address.Hex(),
		n.Faucet.PrivHex,
		n.User1.Address.Hex(),
	)
}

