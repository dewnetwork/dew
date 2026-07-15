package node_test

import (
	"math/big"
	"testing"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/vm"
	"github.com/dewnetwork/dew/devnet"
	"github.com/dewnetwork/dew/node"
	"github.com/dewnetwork/dew/params"
)

func signCall102Data(t *testing.T, privHex string, chainID *big.Int, nonce uint64, value *big.Int, data []byte) []byte {
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

// TestDelegationLab_Scenario: bond validator → set commission → delegate → rank VP →
// undelegate → withdraw after period 0 (lab genesis uses stakingLabGenesis unbond 0).
func TestDelegationLab_Scenario(t *testing.T) {
	if params.DefaultEnableStaking {
		t.Fatal("public freeze expects staking default off")
	}
	g := stakingLabGenesis(t)
	// Period 0 for same-block undelegation withdraw (lab).
	g.Config.Consensus.UnbondingPeriodSeconds = 0

	n := node.OpenTest(t, g)
	n.SetStakingEnabled(true)
	chainID := n.ChainID()

	val := addrFromPriv(t, devnet.PrivHex1)
	del := addrFromPriv(t, devnet.PrivHex2)
	// Bond validator with high self-stake
	const selfStake int64 = 5000
	if _, err := n.SendRawTransaction(signCall102Data(t, devnet.PrivHex1, chainID, 0, big.NewInt(selfStake), []byte{vm.StakeMethodBond})); err != nil {
		t.Fatalf("bond: %v", err)
	}

	// Commission 10%
	comm := append([]byte{vm.StakeMethodSetCommission}, make([]byte, 32)...)
	uint256.NewInt(1000).WriteToSlice(comm[1:])
	if _, err := n.SendRawTransaction(signCall102Data(t, devnet.PrivHex1, chainID, 1, big.NewInt(0), comm)); err != nil {
		t.Fatalf("setCommission: %v", err)
	}

	// Delegate 3000 from del
	delData := append([]byte{vm.StakeMethodDelegate}, val[:]...)
	if _, err := n.SendRawTransaction(signCall102Data(t, devnet.PrivHex2, chainID, 0, big.NewInt(3000), delData)); err != nil {
		t.Fatalf("delegate: %v", err)
	}

	// VP = 8000
	vpOut, err := n.Call(vm.Message{
		From: del, To: stakePtr(), Data: append([]byte{vm.StakeMethodGetVotingPower}, val[:]...),
		GasLimit: 50_000, GasPrice: big.NewInt(0), Value: uint256.NewInt(0),
	})
	if err != nil {
		t.Fatalf("vp: %v", err)
	}
	if new(uint256.Int).SetBytes(vpOut).Uint64() != 8000 {
		t.Fatalf("vp=%x want 8000", vpOut)
	}

	// Commission query
	cOut, err := n.Call(vm.Message{
		From: val, To: stakePtr(), Data: append([]byte{vm.StakeMethodGetCommission}, val[:]...),
		GasLimit: 50_000, GasPrice: big.NewInt(0), Value: uint256.NewInt(0),
	})
	if err != nil || new(uint256.Int).SetBytes(cOut).Uint64() != 1000 {
		t.Fatalf("commission %v %x", err, cOut)
	}

	// Undelegate 1000 + withdraw (period 0)
	und := append([]byte{vm.StakeMethodUndelegate}, val[:]...)
	und = append(und, make([]byte, 32)...)
	uint256.NewInt(1000).WriteToSlice(und[21:])
	if _, err := n.SendRawTransaction(signCall102Data(t, devnet.PrivHex2, chainID, 1, big.NewInt(0), und)); err != nil {
		t.Fatalf("undelegate: %v", err)
	}
	wd := append([]byte{vm.StakeMethodWithdrawDelegation}, val[:]...)
	if _, err := n.SendRawTransaction(signCall102Data(t, devnet.PrivHex2, chainID, 2, big.NewInt(0), wd)); err != nil {
		t.Fatalf("withdrawDelegation: %v", err)
	}

	// Live delegation 2000
	q := append([]byte{vm.StakeMethodGetDelegation}, val[:]...)
	q = append(q, del[:]...)
	dOut, err := n.Call(vm.Message{
		From: del, To: stakePtr(), Data: q,
		GasLimit: 50_000, GasPrice: big.NewInt(0), Value: uint256.NewInt(0),
	})
	if err != nil || new(uint256.Int).SetBytes(dOut).Uint64() != 2000 {
		t.Fatalf("delegation left %v %x", err, dOut)
	}

	// Pure-delegation address without self-stake never ranks: bond only del as "validator"
	// with 0 self — skip. Active set top should be val.
	countOut, err := n.Call(vm.Message{
		From: val, To: stakePtr(), Data: []byte{vm.StakeMethodActiveCount},
		GasLimit: 50_000, GasPrice: big.NewInt(0), Value: uint256.NewInt(0),
	})
	if err != nil || new(uint256.Int).SetBytes(countOut).Uint64() < 1 {
		t.Fatalf("activeCount %v %x", err, countOut)
	}
}
