package devnet

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/dewnetwork/dew/core/types"
)

func TestMultiProcessBFT_SharedChain(t *testing.T) {
	enc := true
	netw, err := StartMultiProcessBFT(MultiProcessConfig{
		HTTPAddr:   "127.0.0.1:0",
		EncryptP2P: &enc,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = netw.Stop() })

	if len(netw.Validators) != 3 {
		t.Fatalf("validators=%d want 3", len(netw.Validators))
	}
	if netw.Full == nil || netw.RPCURL == "" {
		t.Fatal("missing full node RPC")
	}

	// Submit a signed transfer via full-node RPC (faucet → user1).
	if err := submitFaucetTransfer(t, netw.RPCURL); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)

	tipHash, tipNum := waitUniformTip(t, netw, 3, 120*time.Second)
	if tipNum < 3 {
		t.Fatalf("block number=%d want >= 3", tipNum)
	}

	for i, v := range netw.Validators {
		blk := v.Node.GetBlockByNumber(tipNum)
		if blk == nil || blk.Hash() != tipHash {
			got := types.Hash{}
			if blk != nil {
				got = blk.Hash()
			}
			t.Fatalf("validator %d block %d %s != %s", i, tipNum, got.Hex(), tipHash.Hex())
		}
	}
	fullBlk := netw.Full.Node.GetBlockByNumber(tipNum)
	if fullBlk == nil || fullBlk.Hash() != tipHash {
		got := types.Hash{}
		if fullBlk != nil {
			got = fullBlk.Hash()
		}
		t.Fatalf("full node block %d %s != %s", tipNum, got.Hex(), tipHash.Hex())
	}
}

func TestMultiProcessBFT_ERC20(t *testing.T) {
	if os.Getenv("DEW_HEAVY_INTEGRATION") == "" {
		t.Skip("set DEW_HEAVY_INTEGRATION=1 for full ERC-20 multi-process test (see scripts/devnet-erc20.mjs against compose multi)")
	}
	netw, err := StartMultiProcessBFT(MultiProcessConfig{
		HTTPAddr: "127.0.0.1:0",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = netw.Stop() })

	catchUpFullNode(netw)
	time.Sleep(300 * time.Millisecond)

	supply := new(big.Int).Mul(big.NewInt(1_000_000), big.NewInt(1e18))
	amount := big.NewInt(1000)
	recipient := common.HexToAddress(User1().Address.Hex())
	mesh := func(raw []byte) {
		for _, v := range netw.Validators {
			_, _ = v.Node.SendRawTransaction(raw)
		}
	}
	token, err := DeployAndTransferERC20(netw.RPCURL, Faucet(), recipient, supply, amount, mesh)
	if err != nil {
		t.Fatal(err)
	}
	if token == (common.Address{}) {
		t.Fatal("zero token address")
	}

	waitFullNodeSynced(t, netw, 120*time.Second)
	tipNum := netw.Full.Node.BlockNumber()
	if tipNum < 1 {
		t.Fatalf("block number=%d want >= 1 after ERC-20", tipNum)
	}
	tipHash, _ := waitUniformTip(t, netw, tipNum, 120*time.Second)
	for i, v := range netw.Validators {
		blk := v.Node.GetBlockByNumber(tipNum)
		if blk == nil || blk.Hash() != tipHash {
			t.Fatalf("validator %d block %d mismatch after ERC-20", i, tipNum)
		}
	}
}

func waitFullNodeSynced(t *testing.T, netw *MultiProcessNet, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		catchUpFullNode(netw)
		var target uint64
		for _, v := range netw.Validators {
			if h := v.Node.BlockNumber(); h > target {
				target = h
			}
		}
		if netw.Full.Node.BlockNumber() >= target {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("timeout: full node height=%d mempool=%d",
		netw.Full.Node.BlockNumber(), netw.Full.Node.Mempool().Len())
}

func submitFaucetTransfer(t *testing.T, rpcURL string) error {
	t.Helper()
	client := &rpcClient{url: rpcURL}
	chainID, err := client.chainID()
	if err != nil {
		return err
	}
	faucet := Faucet()
	user1 := User1()
	nonce, err := client.nonce(faucet.Address.Hex())
	if err != nil {
		return err
	}
	to := common.HexToAddress(user1.Address.Hex())
	tx := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    nonce,
		GasPrice: big.NewInt(1_000_000_000),
		Gas:      21_000,
		To:       &to,
		Value:    big.NewInt(1),
	})
	signer := ethtypes.LatestSignerForChainID(chainID)
	signed, err := ethtypes.SignTx(tx, signer, faucet.PrivateKey)
	if err != nil {
		return err
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		return err
	}
	_, err = client.sendRaw(raw)
	return err
}

func waitUniformTip(t *testing.T, netw *MultiProcessNet, minHeight uint64, timeout time.Duration) (types.Hash, uint64) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		catchUpFullNode(netw)
		ref := netw.Validators[0].Node.GetBlockByNumber(minHeight)
		if ref == nil {
			time.Sleep(300 * time.Millisecond)
			continue
		}
		want := ref.Hash()
		aligned := true
		for i, v := range netw.Validators {
			blk := v.Node.GetBlockByNumber(minHeight)
			if blk == nil || blk.Hash() != want {
				aligned = false
				t.Logf("validator %d block %d hash mismatch", i, minHeight)
				break
			}
		}
		fullBlk := netw.Full.Node.GetBlockByNumber(minHeight)
		if fullBlk == nil || fullBlk.Hash() != want {
			aligned = false
		}
		if !aligned {
			time.Sleep(300 * time.Millisecond)
			continue
		}
		// Ensure RPC reports at least minHeight.
		bnHex := rpcString(t, netw.RPCURL, "eth_blockNumber", nil)
		num, err := parseHexUint64(bnHex)
		if err != nil {
			t.Fatal(err)
		}
		if num < minHeight {
			time.Sleep(300 * time.Millisecond)
			continue
		}
		return want, minHeight
	}
	t.Fatalf("timeout waiting for uniform block at height %d", minHeight)
	return types.Hash{}, 0
}

func waitRPCHeight(t *testing.T, rpcURL string, minHeight uint64, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		bnHex := rpcString(t, rpcURL, "eth_blockNumber", nil)
		num, err := parseHexUint64(bnHex)
		if err != nil {
			t.Fatal(err)
		}
		if num >= minHeight {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for rpc block >= %d", minHeight)
}

func pollTip(t *testing.T, rpcURL string, minHeight uint64, timeout time.Duration) (types.Hash, uint64) {
	return pollTipWithURL(t, nil, rpcURL, minHeight, timeout)
}

func pollTipWithNet(t *testing.T, netw *MultiProcessNet, minHeight uint64, timeout time.Duration) (types.Hash, uint64) {
	return pollTipWithURL(t, netw, netw.RPCURL, minHeight, timeout)
}

func pollTipWithURL(t *testing.T, netw *MultiProcessNet, url string, minHeight uint64, timeout time.Duration) (types.Hash, uint64) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastNum uint64
	for time.Now().Before(deadline) {
		bnHex := rpcString(t, url, "eth_blockNumber", nil)
		num, err := parseHexUint64(bnHex)
		if err != nil {
			t.Fatal(err)
		}
		lastNum = num
		if netw != nil && (num < minHeight+2 || num%3 == 0) {
			vh := make([]uint64, len(netw.Validators))
			for i, v := range netw.Validators {
				vh[i] = v.Node.BlockNumber()
			}
			t.Logf("poll rpc=%d validators=%v full=%d want>=%d", num, vh, netw.Full.Node.BlockNumber(), minHeight)
		}
		if num >= minHeight {
			raw := rpcRaw(t, url, "eth_getBlockByNumber", []interface{}{bnHex, false})
			var block struct {
				Hash string `json:"hash"`
			}
			if err := json.Unmarshal(raw, &block); err != nil {
				t.Fatal(err)
			}
			hash, err := rpcDecodeHash(block.Hash)
			if err != nil {
				t.Fatal(err)
			}
			return hash, num
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for block >= %d (last=%d)", minHeight, lastNum)
	return types.Hash{}, 0
}

func parseHexUint64(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "0x") {
		s = "0x" + s
	}
	v, err := parseHexBig(s)
	if err != nil {
		return 0, err
	}
	return v.Uint64(), nil
}

func rpcRaw(t *testing.T, url, method string, params []interface{}) json.RawMessage {
	t.Helper()
	client := &rpcClient{url: url}
	raw, err := client.call(method, params)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func rpcDecodeHash(s string) (types.Hash, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "0x")
	if len(s) != 64 {
		return types.Hash{}, fmt.Errorf("invalid hash %q", s)
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return types.Hash{}, err
	}
	return types.BytesToHash(b), nil
}