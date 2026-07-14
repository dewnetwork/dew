package main

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/dewnetwork/dew/config"
	"github.com/dewnetwork/dew/core/types"
	dewcrypto "github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/node"
	"github.com/dewnetwork/dew/rpc"
)

// parsePrivateKeyHex decodes a 32-byte secp256k1 private key (optional 0x prefix).
func parsePrivateKeyHex(keyHex string) (*ecdsa.PrivateKey, error) {
	keyHex = strings.TrimPrefix(strings.TrimSpace(keyHex), "0x")
	if keyHex == "" {
		return nil, fmt.Errorf("empty private key")
	}
	raw, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("key hex: %w", err)
	}
	key, err := dewcrypto.ToECDSA(raw)
	if err != nil {
		return nil, fmt.Errorf("key: %w", err)
	}
	return key, nil
}

type bftRunConfig struct {
	Genesis        *config.Genesis
	Node           *node.Node
	Stack          *node.StackConfig
	HTTPAddr       string
	HTTPPort       int
	HTTPEnabled    bool
	Staking        bool
}

func runBFTStack(cfg bftRunConfig) error {
	stackCfg := *cfg.Stack
	stackCfg.Genesis = cfg.Genesis
	stackCfg.Node = cfg.Node

	stack, err := node.StartStack(stackCfg)
	if err != nil {
		return err
	}
	defer func() { _ = stack.Stop() }()

	mode := "full-node"
	if stackCfg.Validator {
		mode = "validator"
	}
	fmt.Printf("Dew %s started chainId=%s head=%d\n", mode, cfg.Node.ChainID().String(), cfg.Node.BlockNumber())
	if stack.Host != nil {
		fmt.Printf("  p2p:         listen=%s encrypt=%v peers=%d\n",
			stack.Host.ListenAddr(), stack.Host.EncryptEnabled(), stack.Host.PeerCount())
	}
	fmt.Println("hint: launch checklist — docs/ops/launch-checklist.md")

	if !cfg.HTTPEnabled {
		waitSignal()
		return nil
	}
	return serveHTTP(cfg.Node, stack, cfg.HTTPAddr, cfg.HTTPPort)
}

func serveHTTP(n *node.Node, stack *node.Stack, httpAddr string, httpPort int) error {
	srv := rpc.NewServer()
	api := rpc.NewAPI(n)
	handlers := api.Handlers()
	if stack != nil && stack.Host != nil {
		handlers = wrapGossipHandlers(handlers, n, stack)
	}
	srv.RegisterAll(handlers)
	srv.EnableSubscriptions(n, api)

	addr := fmt.Sprintf("%s:%d", httpAddr, httpPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	fmt.Printf("  rpc:         http://%s (ws upgrade)\n", ln.Addr().String())

	httpSrv := &http.Server{Handler: srv}
	errCh := make(chan error, 1)
	go func() {
		errCh <- httpSrv.Serve(ln)
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sig:
		fmt.Println("shutting down…")
		_ = httpSrv.Close()
		return nil
	case err := <-errCh:
		return err
	}
}

func wrapGossipHandlers(base map[string]rpc.Handler, n *node.Node, stack *node.Stack) map[string]rpc.Handler {
	out := make(map[string]rpc.Handler, len(base))
	for k, v := range base {
		out[k] = v
	}
	if h, ok := out["eth_sendRawTransaction"]; ok {
		out["eth_sendRawTransaction"] = gossipRawTxHandler(h, stack)
	}
	if h, ok := out["dew_sendRawTransaction"]; ok {
		out["dew_sendRawTransaction"] = gossipDewTxHandler(h, stack)
	}
	_ = n
	return out
}

func gossipRawTxHandler(inner rpc.Handler, stack *node.Stack) rpc.Handler {
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

func gossipDewTxHandler(inner rpc.Handler, stack *node.Stack) rpc.Handler {
	return gossipRawTxHandler(inner, stack)
}