package devnet

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/core/vm"
)

func TestMultiProcessBFT_LongEmpty(t *testing.T) {
	if os.Getenv("DEW_HEAVY_INTEGRATION") == "" {
		t.Skip("set DEW_HEAVY_INTEGRATION=1 for multiproc empty soak ≥150 heights")
	}
	const targetHeight uint64 = 150
	netw, err := StartMultiProcessBFT(MultiProcessConfig{
		HTTPAddr:         "127.0.0.1:0",
		MinBlockInterval: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	var catchUpWG sync.WaitGroup
	catchUpWG.Add(1)
	go func() {
		defer catchUpWG.Done()
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				catchUpFullNode(netw)
			}
		}
	}()
	t.Cleanup(func() {
		close(done)
		catchUpWG.Wait()
		_ = netw.Stop()
	})

	start := time.Now()
	tipHash, tipNum := waitUniformTip(t, netw, targetHeight, 4*time.Minute)
	elapsed := time.Since(start)
	if tipNum < targetHeight {
		t.Fatalf("height=%d want >= %d", tipNum, targetHeight)
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
	logBFTCommitRow(t, "long_empty", tipNum, 200*time.Millisecond, elapsed)
}

// TestMultiProcessBFT_CommitLatencyLab — Track R: short multiproc empty-chain pace.
// Always runs in CI (unlike LongEmpty). Logs BFT_ROW for research-lab baselines.
func TestMultiProcessBFT_CommitLatencyLab(t *testing.T) {
	const targetHeight uint64 = 12
	interval := 50 * time.Millisecond
	enc := true
	netw, err := StartMultiProcessBFT(MultiProcessConfig{
		HTTPAddr:         "127.0.0.1:0",
		EncryptP2P:       &enc,
		MinBlockInterval: interval,
	})
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	var catchUpWG sync.WaitGroup
	catchUpWG.Add(1)
	go func() {
		defer catchUpWG.Done()
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				catchUpFullNode(netw)
			}
		}
	}()
	t.Cleanup(func() {
		close(done)
		catchUpWG.Wait()
		_ = netw.Stop()
	})

	start := time.Now()
	tipHash, tipNum := waitUniformTip(t, netw, targetHeight, 90*time.Second)
	elapsed := time.Since(start)
	if tipNum < targetHeight {
		t.Fatalf("height=%d want >= %d", tipNum, targetHeight)
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
	logBFTCommitRow(t, "commit_latency_lab", tipNum, interval, elapsed)
}

func logBFTCommitRow(t *testing.T, scenario string, tipNum uint64, minInterval time.Duration, elapsed time.Duration) {
	t.Helper()
	ms := float64(elapsed) / float64(time.Millisecond)
	hps := 0.0
	if elapsed > 0 {
		hps = float64(tipNum) / elapsed.Seconds()
	}
	// Mean wall-clock per height (includes boot + catch-up to uniform tip).
	meanMs := 0.0
	if tipNum > 0 {
		meanMs = ms / float64(tipNum)
	}
	t.Logf("BFT_ROW scenario=%s tip=%d elapsed_ms=%.1f mean_ms_per_height=%.1f heights_per_s=%.2f min_interval_ms=%d validators=3 full=1",
		scenario, tipNum, ms, meanMs, hps, minInterval.Milliseconds())
}

func TestMultiProcessBFT_SharedChain(t *testing.T) {
	enc := true
	netw, err := StartMultiProcessBFT(MultiProcessConfig{
		HTTPAddr:         "127.0.0.1:0",
		EncryptP2P:       &enc,
		MinBlockInterval: 50 * time.Millisecond, // keep light CI under default 1s production pace
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
	// Defer BFT so the deploy tx sits in every mempool before the first proposal.
	netw, err := StartMultiProcessBFT(MultiProcessConfig{
		HTTPAddr:       "127.0.0.1:0",
		DeferConsensus: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = netw.Stop() })

	supply := new(big.Int).Mul(big.NewInt(1_000_000), big.NewInt(1e18))
	amount := big.NewInt(1000)
	recipient := common.HexToAddress(User1().Address.Hex())
	faucet := Faucet()

	// Pre-build deploy raw (nonce 0) and admit before consensus starts.
	deployRaw, err := signTokenDeploy(faucet, netw.Validators[0].Node.ChainID(), 0, supply)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range netw.Validators {
		if _, err := v.Node.SendRawTransaction(deployRaw); err != nil {
			t.Fatalf("pre-admit deploy: %v", err)
		}
	}
	if _, err := netw.Full.Node.SendRawTransaction(deployRaw); err != nil {
		t.Fatalf("pre-admit full: %v", err)
	}

	if err := netw.StartConsensus(); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				catchUpFullNode(netw)
			}
		}
	}()
	defer close(done)

	// Wait for deploy receipt on full RPC, then transfer.
	client := &rpcClient{url: netw.RPCURL}
	deployTx := new(ethtypes.Transaction)
	if err := deployTx.UnmarshalBinary(deployRaw); err != nil {
		t.Fatal(err)
	}
	deployHash := deployTx.Hash().Hex()
	rc, err := client.waitReceipt(deployHash)
	if err != nil {
		t.Fatal(err)
	}
	if rc.Status != 1 || rc.ContractAddress == (common.Address{}) {
		t.Fatalf("deploy status=%d addr=%s", rc.Status, rc.ContractAddress.Hex())
	}
	token := rc.ContractAddress

	mesh := func(raw []byte) {
		for _, v := range netw.Validators {
			_, _ = v.Node.SendRawTransaction(raw)
		}
	}
	// Second tx: transfer via helper path (nonce 1).
	transferRaw, err := signTokenTransfer(faucet, netw.Validators[0].Node.ChainID(), 1, token, recipient, amount)
	if err != nil {
		t.Fatal(err)
	}
	mesh(transferRaw)
	txHash2, err := client.sendRaw(transferRaw)
	if err != nil {
		t.Fatal(err)
	}
	rc2, err := client.waitReceipt(txHash2)
	if err != nil {
		t.Fatal(err)
	}
	if rc2.Status != 1 {
		t.Fatalf("transfer status=%d", rc2.Status)
	}

	catchUpFullNode(netw)
	tipNum := netw.Full.Node.BlockNumber()
	if tipNum < 1 {
		t.Fatalf("block number=%d want >= 1 after ERC-20", tipNum)
	}
	tipHash, _ := waitUniformTip(t, netw, tipNum, 60*time.Second)
	for i, v := range netw.Validators {
		blk := v.Node.GetBlockByNumber(tipNum)
		if blk == nil || blk.Hash() != tipHash {
			t.Fatalf("validator %d block %d mismatch after ERC-20", i, tipNum)
		}
	}
	_ = token
}

func signTokenDeploy(deployer Account, chainID *big.Int, nonce uint64, supply *big.Int) ([]byte, error) {
	parsed, err := abi.JSON(strings.NewReader(vm.TokenABI))
	if err != nil {
		return nil, err
	}
	bin, err := hex.DecodeString(vm.TokenCreationBytecode)
	if err != nil {
		return nil, err
	}
	ctor, err := parsed.Pack("", supply)
	if err != nil {
		return nil, err
	}
	data := append(append([]byte{}, bin...), ctor...)
	tx := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    nonce,
		GasPrice: big.NewInt(1_000_000_000),
		Gas:      3_000_000,
		Data:     data,
	})
	signed, err := ethtypes.SignTx(tx, ethtypes.LatestSignerForChainID(chainID), deployer.PrivateKey)
	if err != nil {
		return nil, err
	}
	return signed.MarshalBinary()
}

func signTokenTransfer(from Account, chainID *big.Int, nonce uint64, token, recipient common.Address, amount *big.Int) ([]byte, error) {
	parsed, err := abi.JSON(strings.NewReader(vm.TokenABI))
	if err != nil {
		return nil, err
	}
	calldata, err := parsed.Pack("transfer", recipient, amount)
	if err != nil {
		return nil, err
	}
	tx := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    nonce,
		GasPrice: big.NewInt(1_000_000_000),
		Gas:      100_000,
		To:       &token,
		Data:     calldata,
	})
	signed, err := ethtypes.SignTx(tx, ethtypes.LatestSignerForChainID(chainID), from.PrivateKey)
	if err != nil {
		return nil, err
	}
	return signed.MarshalBinary()
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
	vh := make([]uint64, len(netw.Validators))
	for i, v := range netw.Validators {
		if v != nil && v.Node != nil {
			vh[i] = v.Node.BlockNumber()
		}
	}
	fullH := uint64(0)
	if netw.Full != nil && netw.Full.Node != nil {
		fullH = netw.Full.Node.BlockNumber()
	}
	t.Fatalf("timeout waiting for uniform block at height %d (validators=%v full=%d)", minHeight, vh, fullH)
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
