package node_test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/config"
	"github.com/dewnetwork/dew/consensus"
	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/core/vm"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/devnet"
	"github.com/dewnetwork/dew/node"
	"github.com/dewnetwork/dew/params"
)

// stakingLabGenesis is a private-lab genesis: short epoch, zero unbond wait, low min stake.
func stakingLabGenesis(t *testing.T) *config.Genesis {
	t.Helper()
	g, err := devnet.DefaultGenesis()
	if err != nil {
		t.Fatal(err)
	}
	if g.Config == nil {
		t.Fatal("nil config")
	}
	g.Config.Consensus = &config.ConsensusConfig{
		Type:                   "dew-bft",
		EpochLength:            2, // rotate every even height
		UnbondingPeriodSeconds: 0, // withdraw immediately after unbond (lab)
		MinValidatorStake:      "1000",
		ActiveValidatorCap:     2,
	}
	return g
}

func stakePrecompileAddr() common.Address {
	return common.BytesToAddress([]byte{0x01, 0x02})
}

func stakePtr() *crypto.Address {
	a := crypto.MustHexToAddress("0x0000000000000000000000000000000000000102")
	return &a
}

func signCall102(t *testing.T, privHex string, chainID *big.Int, nonce uint64, value *big.Int, data []byte) []byte {
	t.Helper()
	key, err := ethcrypto.HexToECDSA(privHex)
	if err != nil {
		t.Fatal(err)
	}
	to := stakePrecompileAddr()
	tx := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    nonce,
		To:       &to,
		Value:    value,
		Gas:      300_000,
		GasPrice: big.NewInt(1_000_000_000),
		Data:     data,
	})
	signed, err := ethtypes.SignTx(tx, ethtypes.LatestSignerForChainID(chainID), key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func addrFromPriv(t *testing.T, privHex string) crypto.Address {
	t.Helper()
	key, err := ethcrypto.HexToECDSA(privHex)
	if err != nil {
		t.Fatal(err)
	}
	return crypto.PubkeyToAddress(&key.PublicKey)
}

// TestStakingLab_Scenario walks bond → ActiveSet rank → epoch rotation → unbond/withdraw → jail.
// Lab-only path: SetStakingEnabled(true). Public-testnet default remains off.
func TestStakingLab_Scenario(t *testing.T) {
	if params.DefaultEnableStaking || params.PublicTestnetStakingOn {
		t.Fatal("public freeze expects staking default off")
	}

	g := stakingLabGenesis(t)
	n := node.OpenTest(t, g)
	if n.StakingEnabled() {
		t.Fatal("node must default staking off")
	}
	n.SetStakingEnabled(true)

	chainID := n.ChainID()
	a0 := addrFromPriv(t, devnet.PrivHex0)
	a1 := addrFromPriv(t, devnet.PrivHex1)
	a2 := addrFromPriv(t, devnet.PrivHex2)

	// --- Bond: a1 high stake, a2 lower ---
	const stakeHi, stakeLo uint64 = 9000, 5000
	if _, err := n.SendRawTransaction(signCall102(t, devnet.PrivHex1, chainID, 0, big.NewInt(int64(stakeHi)), []byte{vm.StakeMethodBond})); err != nil {
		t.Fatalf("bond a1: %v", err)
	}
	if _, err := n.SendRawTransaction(signCall102(t, devnet.PrivHex2, chainID, 0, big.NewInt(int64(stakeLo)), []byte{vm.StakeMethodBond})); err != nil {
		t.Fatalf("bond a2: %v", err)
	}

	// Active set ranking (module order: higher power first).
	countData, err := n.Call(vm.Message{
		From: a0, To: stakePtr(), Data: []byte{vm.StakeMethodActiveCount},
		GasLimit: 50_000, GasPrice: big.NewInt(0), Value: uint256.NewInt(0),
	})
	if err != nil {
		t.Fatalf("activeCount: %v", err)
	}
	if new(uint256.Int).SetBytes(countData).Uint64() != 2 {
		t.Fatalf("activeCount=%x want 2", countData)
	}
	idx0 := append([]byte{vm.StakeMethodActiveAt}, make([]byte, 32)...)
	at0, err := n.Call(vm.Message{
		From: a0, To: stakePtr(), Data: idx0,
		GasLimit: 50_000, GasPrice: big.NewInt(0), Value: uint256.NewInt(0),
	})
	if err != nil {
		t.Fatalf("activeAt0: %v", err)
	}
	var rank0 crypto.Address
	copy(rank0[:], at0[12:32])
	if rank0 != a1 {
		t.Fatalf("rank0=%s want a1=%s", rank0.Hex(), a1.Hex())
	}

	// --- Epoch rotation at even height ---
	h := n.BlockNumber()
	for h%2 != 0 {
		if _, err := n.SendRawTransaction(signLegacyTransfer(t, devnet.PrivHex0, chainID, n.GetNonce(a0), common.Address(a0), big.NewInt(0), big.NewInt(1_000_000_000))); err != nil {
			t.Fatalf("pad block: %v", err)
		}
		h = n.BlockNumber()
	}
	vs, err := n.TryRotateValidatorSet(h)
	if err != nil {
		t.Fatalf("TryRotateValidatorSet: %v", err)
	}
	if vs == nil {
		t.Fatal("expected ActiveSet-based BFT set at epoch boundary")
	}
	// BFT set is address-sorted; assert membership + power (not rank order).
	if len(vs.Validators) != 2 {
		t.Fatalf("rotated set size %d", len(vs.Validators))
	}
	if p := vs.PowerOf(a1); p != stakeHi {
		t.Fatalf("a1 power %d want %d", p, stakeHi)
	}
	if p := vs.PowerOf(a2); p != stakeLo {
		t.Fatalf("a2 power %d want %d", p, stakeLo)
	}
	if vs.PowerOf(a0) != 0 {
		t.Fatal("unbonded a0 must not appear in ActiveSet rotation")
	}
	if mid, err := n.TryRotateValidatorSet(h + 1); err != nil || mid != nil {
		t.Fatalf("mid-epoch want nil,nil got %v %v", mid, err)
	}

	// --- Unbond half of a1 + immediate withdraw (period 0) ---
	unbondAmt := stakeHi / 2
	unbondData := append([]byte{vm.StakeMethodUnbond}, make([]byte, 32)...)
	uint256.NewInt(unbondAmt).WriteToSlice(unbondData[1:])
	if _, err := n.SendRawTransaction(signCall102(t, devnet.PrivHex1, chainID, 1, big.NewInt(0), unbondData)); err != nil {
		t.Fatalf("unbond: %v", err)
	}
	if _, err := n.SendRawTransaction(signCall102(t, devnet.PrivHex1, chainID, 2, big.NewInt(0), []byte{vm.StakeMethodWithdraw})); err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	stakeQ := append([]byte{vm.StakeMethodGetSelfStake}, a1[:]...)
	left, err := n.Call(vm.Message{
		From: a1, To: stakePtr(), Data: stakeQ,
		GasLimit: 50_000, GasPrice: big.NewInt(0), Value: uint256.NewInt(0),
	})
	if err != nil {
		t.Fatalf("get stake: %v", err)
	}
	if new(uint256.Int).SetBytes(left).Uint64() != stakeHi-unbondAmt {
		t.Fatalf("stake after unbond/withdraw %x want %d", left, stakeHi-unbondAmt)
	}

	// --- Double-sign jail (optional lab path) ---
	offKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	offender := crypto.PubkeyToAddress(&offKey.PublicKey)
	// Fund offender.
	if _, err := n.SendRawTransaction(signLegacyTransfer(t, devnet.PrivHex0, chainID, n.GetNonce(a0), common.Address(offender), big.NewInt(1_000_000_000_000_000_000), big.NewInt(1_000_000_000))); err != nil {
		t.Fatalf("fund offender: %v", err)
	}
	// Bond with eth-compatible ECDSA key (same curve / address).
	ethKey, err := ethcrypto.ToECDSA(crypto.FromECDSA(offKey))
	if err != nil {
		t.Fatal(err)
	}
	to102 := stakePrecompileAddr()
	bondTx := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce: 0, To: &to102, Value: big.NewInt(2000), Gas: 300_000,
		GasPrice: big.NewInt(1_000_000_000), Data: []byte{vm.StakeMethodBond},
	})
	signedBond, err := ethtypes.SignTx(bondTx, ethtypes.LatestSignerForChainID(chainID), ethKey)
	if err != nil {
		t.Fatal(err)
	}
	rawBond, _ := signedBond.MarshalBinary()
	if _, err := n.SendRawTransaction(rawBond); err != nil {
		t.Fatalf("bond offender: %v", err)
	}
	var ha, hb types.Hash
	ha[0], hb[0] = 0xaa, 0xbb
	va := &consensus.Vote{Type: consensus.VotePrecommit, Height: 1, Round: 0, BlockHash: ha}
	vb := &consensus.Vote{Type: consensus.VotePrecommit, Height: 1, Round: 0, BlockHash: hb}
	if err := consensus.SignVote(va, offKey); err != nil {
		t.Fatal(err)
	}
	if err := consensus.SignVote(vb, offKey); err != nil {
		t.Fatal(err)
	}
	wire, err := consensus.EncodeDoubleSignEvidenceWire(va, vb)
	if err != nil {
		t.Fatal(err)
	}
	jailIn := append([]byte{vm.StakeMethodJail}, wire...)
	if _, err := n.SendRawTransaction(signCall102(t, devnet.PrivHex0, chainID, n.GetNonce(a0), big.NewInt(0), jailIn)); err != nil {
		t.Fatalf("jail: %v", err)
	}
	qJail := append([]byte{vm.StakeMethodIsJailed}, offender[:]...)
	jailOut, err := n.Call(vm.Message{
		From: a0, To: stakePtr(), Data: qJail,
		GasLimit: 50_000, GasPrice: big.NewInt(0), Value: uint256.NewInt(0),
	})
	if err != nil {
		t.Fatalf("isJailed: %v", err)
	}
	if new(uint256.Int).SetBytes(jailOut).Uint64() != 1 {
		t.Fatalf("offender %s not jailed: %x", offender.Hex(), jailOut)
	}

	// Fresh node keeps staking off (public / private default).
	n2 := node.OpenTest(t, stakingLabGenesis(t))
	if n2.StakingEnabled() {
		t.Fatal("fresh node must keep staking off by default")
	}
}
