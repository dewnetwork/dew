package devnet

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dewnetwork/dew/config"
	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/node"
	"github.com/dewnetwork/dew/p2p"
	"github.com/dewnetwork/dew/rpc"
)

// MultiProcessNet is a multi-process BFT devnet: 3 validators + 1 full RPC node.
type MultiProcessNet struct {
	Validators []*node.Stack
	Full       *node.Stack
	RPCURL     string

	genesis  *config.Genesis
	dataRoot string
	mu       sync.Mutex
	httpSrv  *http.Server
	ln       net.Listener
}

// MultiProcessConfig configures StartMultiProcessBFT.
type MultiProcessConfig struct {
	Genesis    *config.Genesis
	HTTPAddr   string // full-node RPC bind (default 127.0.0.1:0)
	EncryptP2P *bool
	// DeferConsensus leaves validators idle until StartConsensus so tests can
	// admit mempool txs before the first proposal (avoids empty-block races).
	DeferConsensus bool
	// MinBlockInterval applied to validator runners (0 = consensus default 1s).
	MinBlockInterval time.Duration
}

// StartMultiProcessBFT boots 3 validator stacks and 1 full-node stack with HTTP RPC.
func StartMultiProcessBFT(cfg MultiProcessConfig) (*MultiProcessNet, error) {
	g := cfg.Genesis
	var err error
	if g == nil {
		g, err = DefaultGenesis()
		if err != nil {
			return nil, err
		}
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = "127.0.0.1:0"
	}
	encrypt := true
	if cfg.EncryptP2P != nil {
		encrypt = *cfg.EncryptP2P
	}

	vals := DefaultValidators()
	dataRoot, err := os.MkdirTemp("", "dew-multiproc-*")
	if err != nil {
		return nil, fmt.Errorf("devnet: temp dir: %w", err)
	}
	netw := &MultiProcessNet{
		Validators: make([]*node.Stack, 0, len(vals)),
		genesis:    g,
		dataRoot:   dataRoot,
	}

	// Start validators sequentially so later nodes can dial earlier bootnodes.
	var bootAddrs []string
	for i, v := range vals {
		n, err := node.Open(g, filepath.Join(dataRoot, fmt.Sprintf("val-%d", i), "chaindata"))
		if err != nil {
			_ = netw.Stop()
			return nil, fmt.Errorf("devnet: validator %d node: %w", i, err)
		}
		stack, err := node.StartStack(node.StackConfig{
			Genesis:          g,
			Node:             n,
			Validator:        true,
			ValidatorKey:     v.PrivateKey,
			P2PListen:        "127.0.0.1:0",
			P2PPrivateKey:    v.PrivateKey,
			Bootnodes:        append([]string(nil), bootAddrs...),
			Encrypt:          encrypt,
			AllowCleartext:   !encrypt,
			DeferRunner:      true,
			MinBlockInterval: cfg.MinBlockInterval,
		})
		if err != nil {
			_ = netw.Stop()
			return nil, fmt.Errorf("devnet: validator %d stack: %w", i, err)
		}
		netw.Validators = append(netw.Validators, stack)
		bootAddrs = append(bootAddrs, stack.Host.ListenAddr())
	}

	// Full node dials all validators.
	fullNode, err := node.Open(g, filepath.Join(dataRoot, "full", "chaindata"))
	if err != nil {
		_ = netw.Stop()
		return nil, fmt.Errorf("devnet: full node: %w", err)
	}
	// Ephemeral P2P identity for the RPC node (not a validator).
	p2pKey, err := AccountFromHex("rpc-p2p", PrivHex3)
	if err != nil {
		_ = netw.Stop()
		return nil, err
	}
	full, err := node.StartStack(node.StackConfig{
		Genesis:        g,
		Node:           fullNode,
		Validator:      false,
		P2PListen:      "127.0.0.1:0",
		P2PPrivateKey:  p2pKey.PrivateKey,
		Bootnodes:      bootAddrs,
		Encrypt:        encrypt,
		AllowCleartext: !encrypt,
	})
	if err != nil {
		_ = netw.Stop()
		return nil, fmt.Errorf("devnet: full stack: %w", err)
	}
	netw.Full = full

	if err := meshValidatorStacks(netw.Validators); err != nil {
		_ = netw.Stop()
		return nil, err
	}
	if !cfg.DeferConsensus {
		if err := netw.StartConsensus(); err != nil {
			_ = netw.Stop()
			return nil, err
		}
	}
	for _, addr := range bootAddrs {
		addr := addr
		go dialBootnodeFull(full.Host, addr)
	}
	// HTTP RPC on full node only.
	srv := rpc.NewServer()
	api := rpc.NewAPI(fullNode)
	handlers := api.Handlers()
	handlers = wrapGossipHandlers(handlers, full)
	srv.RegisterAll(handlers)
	srv.EnableSubscriptions(fullNode, api)

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

	time.Sleep(200 * time.Millisecond)
	catchUpFullNode(netw)
	return netw, nil
}

func catchUpFullNode(netw *MultiProcessNet) {
	if netw == nil {
		return
	}
	full := netw.Full
	if full == nil || full.Host == nil || full.Node == nil {
		return
	}
	host := full.Host
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if netw.Full == nil {
			return
		}
		for _, p := range host.Store().Active() {
			_ = host.SyncMissingFromPeer(p)
		}
		var target uint64
		for _, v := range netw.Validators {
			if v == nil || v.Node == nil {
				continue
			}
			if h := v.Node.BlockNumber(); h > target {
				target = h
			}
		}
		if full.Node.BlockNumber() >= target {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func meshValidatorStacks(stacks []*node.Stack) error {
	for i, a := range stacks {
		for j, b := range stacks {
			if i == j {
				continue
			}
			if _, ok := a.Host.Store().GetActive(b.Host.ID()); ok {
				continue
			}
			if _, err := a.Host.Dial(b.Host.ListenAddr()); err != nil {
				return fmt.Errorf("devnet: mesh %d→%d: %w", i, j, err)
			}
		}
	}
	return nil
}

func dialBootnodeFull(host *p2p.Host, addr string) {
	if host == nil {
		return
	}
	for i := 0; i < 15; i++ {
		if _, err := host.Dial(addr); err == nil {
			return
		}
		time.Sleep(time.Second)
	}
}

// StartConsensus begins BFT on all validator stacks (no-op for full node).
func (m *MultiProcessNet) StartConsensus() error {
	if m == nil {
		return nil
	}
	for i, v := range m.Validators {
		if err := v.StartConsensus(); err != nil {
			return fmt.Errorf("devnet: start consensus validator %d: %w", i, err)
		}
	}
	return nil
}

// Stop shuts down RPC and all stacks.
func (m *MultiProcessNet) Stop() error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.httpSrv != nil {
		_ = m.httpSrv.Close()
		m.httpSrv = nil
	}
	if m.ln != nil {
		_ = m.ln.Close()
		m.ln = nil
	}
	for _, v := range m.Validators {
		_ = v.Stop()
	}
	m.Validators = nil
	if m.Full != nil {
		_ = m.Full.Stop()
		m.Full = nil
	}
	if m.dataRoot != "" {
		_ = os.RemoveAll(m.dataRoot)
		m.dataRoot = ""
	}
	return nil
}

func wrapGossipHandlers(base map[string]rpc.Handler, stack *node.Stack) map[string]rpc.Handler {
	out := make(map[string]rpc.Handler, len(base))
	for k, v := range base {
		out[k] = v
	}
	if h, ok := out["eth_sendRawTransaction"]; ok {
		out["eth_sendRawTransaction"] = gossipAfter(h, stack)
	}
	return out
}

func gossipAfter(inner rpc.Handler, stack *node.Stack) rpc.Handler {
	return func(params json.RawMessage) (interface{}, error) {
		result, err := inner(params)
		if err != nil {
			return nil, err
		}
		if hashHex, ok := result.(string); ok {
			if hash, decErr := rpc.DecodeHash(hashHex); decErr == nil {
				go func(h types.Hash) { _ = stack.GossipTx(h) }(hash)
			}
		}
		return result, nil
	}
}