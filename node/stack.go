package node

import (
	"crypto/ecdsa"
	"fmt"
	"path/filepath"
	"time"

	"github.com/dewnetwork/dew/config"
	"github.com/dewnetwork/dew/consensus"
	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/p2p"
)

// StackConfig wires a Dew node with optional BFT validator mode and P2P.
type StackConfig struct {
	Genesis *config.Genesis
	Node    *Node

	ValidatorKey *ecdsa.PrivateKey // required if Validator=true
	Validator    bool

	P2PListen      string
	P2PPrivateKey  *ecdsa.PrivateKey
	Bootnodes      []string
	Encrypt        bool
	AllowCleartext bool
	// DataDir, when set, persists known peers at <DataDir>/peers.json (D3b).
	DataDir string
	// PeerStorePath overrides DataDir/peers.json when non-empty.
	PeerStorePath string
	// DeferRunner delays Runner.Start until Stack.StartConsensus (for test harness mesh setup).
	DeferRunner bool
	// MinBlockInterval paces BFT StartRound after commit (0 = consensus default 1s; negative disables).
	MinBlockInterval time.Duration
}

// Stack is a running node with optional consensus engine, P2P host, and runner.
type Stack struct {
	Node   *Node
	Engine *consensus.Engine
	Host   *p2p.Host
	Runner *consensus.Runner

	stopCh chan struct{}
}

// StartStack boots P2P and, when Validator=true, Dew-BFT engine + runner.
func StartStack(cfg StackConfig) (*Stack, error) {
	if cfg.Node == nil {
		return nil, fmt.Errorf("node: missing node")
	}
	if cfg.Genesis == nil {
		return nil, fmt.Errorf("node: missing genesis")
	}
	if cfg.P2PListen == "" {
		return nil, fmt.Errorf("node: missing p2p listen address")
	}
	if cfg.P2PPrivateKey == nil {
		return nil, fmt.Errorf("node: missing p2p private key")
	}
	if cfg.Validator && cfg.ValidatorKey == nil {
		return nil, fmt.Errorf("node: validator mode requires validator key")
	}

	cfg.Node.SetAutoMine(false)

	stack := &Stack{Node: cfg.Node, stopCh: make(chan struct{})}
	backend := &P2PBackend{N: cfg.Node}

	var engine *consensus.Engine
	handlers := appHandlers(stack, cfg.Validator, &engine)

	peerPath := cfg.PeerStorePath
	if peerPath == "" && cfg.DataDir != "" {
		peerPath = filepath.Join(cfg.DataDir, "peers.json")
	}
	p2pCfg := p2p.Config{
		PrivateKey:     cfg.P2PPrivateKey,
		ChainID:        cfg.Genesis.ChainID(),
		ListenAddr:     cfg.P2PListen,
		MaxPeers:       25,
		Encrypt:        cfg.Encrypt,
		AllowCleartext: cfg.AllowCleartext,
		PeerStorePath:  peerPath,
		Bootnodes:      cfg.Bootnodes,
	}
	host, err := p2p.NewHost(p2pCfg, backend, backend, handlers)
	if err != nil {
		return nil, err
	}
	stack.Host = host

	if cfg.Validator {
		valSet, err := consensus.ValidatorSetFromGenesis(cfg.Genesis)
		if err != nil {
			return nil, err
		}
		addr := crypto.PubkeyToAddress(&cfg.ValidatorKey.PublicKey)
		if _, ok := valSet.Get(addr); !ok {
			return nil, fmt.Errorf("node: validator key %s not in genesis validator set", addr.Hex())
		}

		parent := cfg.Node.CurrentHeader()
		if parent == nil {
			return nil, fmt.Errorf("node: missing genesis header")
		}

		engine, err = consensus.NewEngine(consensus.EngineConfig{
			PrivateKey: cfg.ValidatorKey,
			ValSet:     valSet,
			Parent:     parent,
			Builder: &consensus.MempoolBlockBuilder{
				Exec:   cfg.Node,
				MaxTxs: DefaultMaxTxsPerBlock,
			},
			Validator:   &consensus.ExecutionValidator{Exec: cfg.Node},
			ProposeRoot: parent.StateRoot,
			Broadcast:   p2p.NewBroadcaster(host),
		})
		if err != nil {
			return nil, err
		}
		stack.Engine = engine

		runner := &consensus.Runner{
			Engine:           engine,
			MinBlockInterval: cfg.MinBlockInterval,
			OnCommit: func(ev consensus.CommitEvent) error {
				if ev.Block != nil {
					if err := cfg.Node.ImportCommittedBlock(ev.Block); err != nil {
						return err
					}
				}
				if stack.Host != nil {
					_ = stack.Host.GossipBlock(ev.BlockHash)
				}
				return nil
			},
		}
		stack.Runner = runner
	}

	if err := host.Start(); err != nil {
		return nil, err
	}

	if stack.Runner != nil && !cfg.DeferRunner {
		if err := stack.StartConsensus(); err != nil {
			_ = host.Close()
			return nil, err
		}
	}

	// Bootnodes are dialed by Host redial loop (D3b). Keep a short one-shot
	// boost for compose startup ordering when redial tick has not fired yet.
	for _, addr := range cfg.Bootnodes {
		addr := addr
		if addr == "" {
			continue
		}
		go dialBootnode(host, addr)
	}
	// Full nodes (and lagging validators) periodically pull missing blocks when
	// a peer tip is ahead. Avoids permanent lag from tip-only gossip.
	go stack.syncLoop()
	time.Sleep(50 * time.Millisecond)

	return stack, nil
}

// syncLoop pulls missing blocks only when a peer reports a higher tip.
func (s *Stack) syncLoop() {
	if s == nil || s.Host == nil || s.stopCh == nil {
		return
	}
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-tick.C:
			local := s.Node.BlockNumber()
			for _, p := range s.Host.Store().Active() {
				if p.Height > local {
					_ = s.Host.SyncMissingFromPeer(p)
				}
			}
		}
	}
}

// StartConsensus begins the BFT runner loop (no-op for full nodes).
func (s *Stack) StartConsensus() error {
	if s == nil || s.Runner == nil {
		return nil
	}
	return s.Runner.Start()
}

// GossipTx announces a mempool tx hash to all peers.
func (s *Stack) GossipTx(hash types.Hash) error {
	if s == nil || s.Host == nil {
		return nil
	}
	return s.Host.GossipTx(hash)
}

// Stop shuts down the consensus runner and P2P host.
func (s *Stack) Stop() error {
	if s == nil {
		return nil
	}
	if s.stopCh != nil {
		select {
		case <-s.stopCh:
		default:
			close(s.stopCh)
		}
	}
	if s.Runner != nil {
		s.Runner.Stop()
	}
	var err error
	if s.Host != nil {
		err = s.Host.Close()
	}
	if s.Node != nil {
		if cerr := s.Node.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}
	return err
}

func appHandlers(stack *Stack, validator bool, engine **consensus.Engine) p2p.AppHandlers {
	h := p2p.AppHandlers{
		OnBlock: func(_ uint64, _ types.Hash, raw []byte, from p2p.PeerID) error {
			blk, err := types.UnmarshalBlockBinary(raw)
			if err != nil {
				return err
			}
			if err := stack.Node.ImportCommittedBlock(blk); err != nil {
				// Sequential gap: pull missing range from the announcing peer.
				// Return err so the host can clear inventory "seen" and retry.
				if stack.Host != nil {
					if p, ok := stack.Host.Store().GetActive(from); ok {
						go func() { _ = stack.Host.SyncMissingFromPeer(p) }()
					}
				}
				return err
			}
			// Validators that missed local BFT commit catch up execution above;
			// advance the engine so they rejoin the next height.
			if validator && engine != nil && *engine != nil {
				advanced, advErr := (*engine).ApplySyncedBlock(blk)
				if advErr == nil && advanced {
					_ = (*engine).StartRound()
				}
			}
			return nil
		},
		OnTx: func(hash types.Hash, raw []byte, _ p2p.PeerID) error {
			backend := &P2PBackend{N: stack.Node}
			if backend.HasTx(hash) {
				return nil
			}
			_, err := stack.Node.SendRawTransaction(raw)
			return err
		},
	}
	if validator {
		h.OnProposal = func(wp *p2p.WireProposal, _ p2p.PeerID) error {
			if engine == nil || *engine == nil {
				return nil
			}
			p, err := p2p.WireToProposal(wp)
			if err != nil {
				return err
			}
			return (*engine).HandleProposal(p)
		}
		h.OnVote = func(wv *p2p.WireVote, _ p2p.PeerID) error {
			if engine == nil || *engine == nil {
				return nil
			}
			v, err := p2p.WireToVote(wv)
			if err != nil {
				return err
			}
			return (*engine).HandleVote(v)
		}
	}
	return h
}

func dialBootnode(host *p2p.Host, addr string) {
	for i := 0; i < 15; i++ {
		if _, err := host.Dial(addr); err == nil {
			return
		}
		time.Sleep(time.Second)
	}
}