package vm

import (
	"fmt"
	"math/big"

	ethcommon "github.com/ethereum/go-ethereum/common"
	ethvm "github.com/ethereum/go-ethereum/core/vm"

	"github.com/dewnetwork/dew/core/state"
	"github.com/dewnetwork/dew/crypto"
)

// Dew system precompile addresses (reserved from 0x100).
var (
	// NativeTransferPrecompile is 0x100 — forward CALLVALUE to a recipient.
	NativeTransferPrecompile = ethcommon.BytesToAddress([]byte{0x01, 0x00})
	// StakingPrecompile is 0x102 — reserved staking entrypoint (Phase B stub).
	StakingPrecompile = ethcommon.BytesToAddress([]byte{0x01, 0x02})
)

// NativeTransferGas is the fixed gas schedule for 0x100.
const NativeTransferGas uint64 = 3_000

// StakingStubGas is the fixed gas for reserved 0x102.
const StakingStubGas uint64 = 2_000

// nativeTransferPrecompile forwards the precompile's received CALLVALUE to a recipient.
//
// Byte layout (frozen for Phase B):
//
//	input = recipient (20 bytes)
//
// Semantics: after EVM transfers callvalue to this address, Run moves the full
// balance of 0x100 to recipient. Deterministic; fixed gas; fail-safe on bad input.
type nativeTransferPrecompile struct {
	statedb *state.StateDB
	self    crypto.Address
}

func (p *nativeTransferPrecompile) RequiredGas(input []byte) uint64 {
	return NativeTransferGas
}

func (p *nativeTransferPrecompile) Run(input []byte) ([]byte, error) {
	if len(input) != 20 {
		return nil, fmt.Errorf("nativeTransfer: input must be exactly 20-byte recipient")
	}
	var to crypto.Address
	copy(to[:], input)

	bal := p.statedb.GetBalance(p.self)
	if bal.IsZero() {
		return ethcommon.LeftPadBytes(big.NewInt(0).Bytes(), 32), nil
	}
	p.statedb.SubBalance(p.self, bal)
	p.statedb.AddBalancePrev(to, bal)
	return ethcommon.LeftPadBytes(bal.ToBig().Bytes(), 32), nil
}

// stakingStubPrecompile reserves 0x102 until the staking module lands.
type stakingStubPrecompile struct{}

func (p *stakingStubPrecompile) RequiredGas(input []byte) uint64 { return StakingStubGas }

func (p *stakingStubPrecompile) Run(input []byte) ([]byte, error) {
	return nil, fmt.Errorf("staking precompile: not enabled (reserved 0x102)")
}

// DewPrecompileAddresses returns addresses to warm in the access list when enabled.
func DewPrecompileAddresses() []ethcommon.Address {
	return []ethcommon.Address{NativeTransferPrecompile, StakingPrecompile}
}

// installDewPrecompiles copies the active fork precompiles and adds Dew system contracts.
func installDewPrecompiles(evm *ethvm.EVM, statedb *state.StateDB, enabled bool) {
	if !enabled {
		return
	}
	rules := evm.ChainConfig().Rules(evm.Context.BlockNumber, evm.Context.Random != nil, evm.Context.Time)
	base := ethvm.ActivePrecompiledContracts(rules)
	merged := make(ethvm.PrecompiledContracts, len(base)+2)
	for k, v := range base {
		merged[k] = v
	}
	var self crypto.Address
	copy(self[:], NativeTransferPrecompile[:])
	merged[NativeTransferPrecompile] = &nativeTransferPrecompile{
		statedb: statedb,
		self:    self,
	}
	merged[StakingPrecompile] = &stakingStubPrecompile{}
	evm.SetPrecompiles(merged)
}

// EnableDewPrecompiles sets the executor feature flag for 0x100+ precompiles.
func (e *Executor) EnableDewPrecompiles(v bool) {
	e.dewPrecompiles = v
}
