package devnet

import (
	"encoding/json"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

func TestDevnet_ThreeValidatorsOneRPC_ERC20(t *testing.T) {
	netw, err := Start(NetworkConfig{
		HTTPAddr:  "127.0.0.1:0",
		EnableP2P: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = netw.Stop() })

	if netw.ValidatorCount() != 3 {
		t.Fatalf("validators=%d", netw.ValidatorCount())
	}
	if netw.RPCURL == "" {
		t.Fatal("missing RPC URL")
	}

	// BFT: 3 validators commit several heights
	evs, err := netw.CommitHeights(3)
	if err != nil {
		t.Fatalf("bft: %v", err)
	}
	if len(evs) != 3 {
		t.Fatalf("commits=%d", len(evs))
	}
	for i, eng := range netw.Cluster.Engines() {
		if eng.LastCommit == nil || eng.LastCommit.BlockHash != evs[len(evs)-1].BlockHash {
			t.Fatalf("engine[%d] disagree on last commit", i)
		}
	}

	// P2P mesh should have sessions
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && netw.P2PPeerCount() < 2 {
		time.Sleep(10 * time.Millisecond)
	}
	if netw.P2PPeerCount() < 2 {
		t.Fatalf("p2p peers=%d want >=2", netw.P2PPeerCount())
	}

	// RPC identity
	chainID := rpcString(t, netw.RPCURL, "eth_chainId", nil)
	if chainID != "0x7ea" {
		t.Fatalf("chainId=%s", chainID)
	}

	// ERC-20 deploy + transfer over RPC
	supply := new(big.Int).Mul(big.NewInt(1_000_000), big.NewInt(1e18))
	amount := big.NewInt(1000)
	recipient := common.HexToAddress(User1().Address.Hex())
	token, err := DeployAndTransferERC20(netw.RPCURL, netw.Faucet, recipient, supply, amount)
	if err != nil {
		t.Fatal(err)
	}
	if token == (common.Address{}) {
		t.Fatal("zero token address")
	}

	// Head advanced via auto-mine
	bn := rpcString(t, netw.RPCURL, "eth_blockNumber", nil)
	if bn == "0x0" || bn == "0x" {
		t.Fatalf("block number still genesis: %s", bn)
	}
}

func TestBuildGenesis_HasThreeValidators(t *testing.T) {
	g, err := DefaultGenesis()
	if err != nil {
		t.Fatal(err)
	}
	if len(g.InitialValidators) != 3 {
		t.Fatalf("initialValidators=%d", len(g.InitialValidators))
	}
	if g.ChainID().Int64() != DefaultChainID {
		t.Fatal(g.ChainID())
	}
	for _, v := range g.InitialValidators {
		if v.VotingPower == 0 || v.Address == "" || !strings.HasPrefix(v.PubKey, "0x04") {
			t.Fatalf("bad validator entry: %+v", v)
		}
	}
	if len(g.Alloc) < 3 {
		t.Fatalf("alloc size %d", len(g.Alloc))
	}
}

func rpcString(t *testing.T, url, method string, params []interface{}) string {
	t.Helper()
	body, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0", "id": 1, "method": method, "params": params,
	})
	res, err := http.Post(url, "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result string `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatal(out.Error.Message)
	}
	return out.Result
}
