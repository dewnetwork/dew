package devnet

import (
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/p2p"
)

// TestPrivateNet_ChaosRestartAndSync — C5: kill/restart a host, re-dial, catch up.
func TestPrivateNet_ChaosRestartAndSync(t *testing.T) {
	netw, err := Start(NetworkConfig{
		HTTPAddr:  "127.0.0.1:0",
		EnableP2P: true,
		// EncryptP2P nil → encrypted default
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = netw.Stop() })

	if len(netw.Hosts) < 3 {
		t.Fatalf("need 3 hosts, got %d", len(netw.Hosts))
	}
	if !netw.Hosts[0].EncryptEnabled() {
		t.Fatal("private net must use encrypted P2P by default")
	}

	// Advance chain on host 0 with a few blocks
	for i := uint64(1); i <= 3; i++ {
		h := types.Keccak256Hash([]byte{byte(i), 'c'})
		netw.Chains[0].AddBlock(i, h, []byte{byte(i)})
	}

	// Restart host 2 (chaos)
	if err := netw.RestartHost(2); err != nil {
		t.Fatal(err)
	}
	if err := netw.RedialHost(2); err != nil {
		t.Fatal(err)
	}

	// Wait for peer sessions
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if netw.Hosts[0].PeerCount() >= 1 && netw.Hosts[2].PeerCount() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if netw.Hosts[2].PeerCount() < 1 {
		t.Fatalf("host2 peers=%d after restart", netw.Hosts[2].PeerCount())
	}

	// Sync host2 from host0
	var peer *p2p.Peer
	for _, p := range netw.Hosts[2].Store().Active() {
		if p.ID.Equal(netw.Hosts[0].ID()) {
			peer = p
			break
		}
	}
	if peer == nil {
		// dialer was host2→0 so peer should be host0
		for _, p := range netw.Hosts[2].Store().Active() {
			peer = p
			break
		}
	}
	if peer == nil {
		t.Fatal("no peer for sync")
	}
	peer.Height = netw.Chains[0].Height()
	if err := netw.Hosts[2].SyncFromPeer(peer); err != nil {
		t.Fatal(err)
	}
	if netw.Chains[2].Height() != 3 {
		t.Fatalf("host2 height %d want 3 after sync", netw.Chains[2].Height())
	}

	// ERC-20 still works on RPC after chaos
	supply := new(big.Int).Mul(big.NewInt(1_000_000), big.NewInt(1e18))
	amount := big.NewInt(1000)
	recipient := common.HexToAddress(User1().Address.Hex())
	if _, err := DeployAndTransferERC20(netw.RPCURL, netw.Faucet, recipient, supply, amount); err != nil {
		t.Fatalf("erc20 after chaos: %v", err)
	}

	// BFT still commits
	if _, err := netw.CommitHeights(1); err != nil {
		t.Fatalf("bft after chaos: %v", err)
	}
}
