package vm

import (
	"fmt"
	"math/big"

	ethcommon "github.com/ethereum/go-ethereum/common"
	ethvm "github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/native"
	"github.com/dewnetwork/dew/core/state"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/params"
)

// Dew system precompile addresses (reserved from 0x100).
var (
	// NativeTransferPrecompile is 0x100 — forward CALLVALUE to a recipient.
	NativeTransferPrecompile = ethcommon.BytesToAddress([]byte{0x01, 0x00})
	// StakingPrecompile is 0x102 — staking entrypoint (Phase C4).
	StakingPrecompile = ethcommon.BytesToAddress([]byte{0x01, 0x02})
)

// NativeTransferGas is the fixed gas schedule for 0x100.
const NativeTransferGas uint64 = 3_000

// Staking method bytes (fail-closed fixed layout; not full Solidity ABI).
const (
	// StakeMethodBond — payable self-stake. input = [0x00]. Credits tx sender.
	StakeMethodBond byte = 0x00
	// StakeMethodUnbond — input = [0x01 || amount uint256 BE].
	StakeMethodUnbond byte = 0x01
	// StakeMethodGetSelfStake — input = [0x02 || address 20]. returns uint256
	StakeMethodGetSelfStake byte = 0x02
	// StakeMethodGetVotingPower — input = [0x03 || address 20]
	StakeMethodGetVotingPower byte = 0x03
	// StakeMethodActiveCount — input = [0x04]. returns uint256
	StakeMethodActiveCount byte = 0x04
	// StakeMethodActiveAt — input = [0x05 || index uint256]. returns address left-padded 32
	StakeMethodActiveAt byte = 0x05
	// StakeMethodJail — input = [0x06 || address 20 || evidenceHash 32]. jails validator
	StakeMethodJail byte = 0x06
	// StakeMethodIsJailed — input = [0x07 || address 20]. returns 0/1 uint256
	StakeMethodIsJailed byte = 0x07
)

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

// stakingPrecompile implements 0x102 (Phase C4).
//
// Caller and callValue are the top-level tx sender / value for this ApplyMessage
// (EOA self-stake path). Nested contract staking is residual debt.
type stakingPrecompile struct {
	statedb   *state.StateDB
	self      crypto.Address
	caller    crypto.Address
	callValue *uint256.Int
	enabled   bool
	cfg       native.StakingConfig
}

func (p *stakingPrecompile) RequiredGas(input []byte) uint64 {
	if len(input) == 0 {
		return params.StakingPrecompileGasQuery
	}
	switch input[0] {
	case StakeMethodBond:
		return params.StakingPrecompileGasBond
	case StakeMethodUnbond:
		return params.StakingPrecompileGasUnbond
	case StakeMethodJail:
		return params.StakingPrecompileGasJail
	default:
		return params.StakingPrecompileGasQuery
	}
}

func (p *stakingPrecompile) Run(input []byte) ([]byte, error) {
	if !p.enabled {
		return nil, fmt.Errorf("staking precompile: not enabled (set EnableStaking)")
	}
	if len(input) < 1 {
		return nil, fmt.Errorf("staking: empty input")
	}
	mod := native.NewStakingModule(p.statedb, p.cfg)
	switch input[0] {
	case StakeMethodBond:
		// CALLVALUE already transferred to 0x102 by EVM; credit only this call's value.
		if len(input) != 1 {
			return nil, fmt.Errorf("staking: bond input must be single method byte")
		}
		amt := p.callValue
		if amt == nil || amt.IsZero() {
			return nil, fmt.Errorf("staking: bond requires non-zero value")
		}
		// Keep value locked at module address; record stake for caller.
		if err := mod.Bond(p.caller, amt); err != nil {
			return nil, err
		}
		return ethcommon.LeftPadBytes(amt.ToBig().Bytes(), 32), nil

	case StakeMethodUnbond:
		if len(input) != 1+32 {
			return nil, fmt.Errorf("staking: unbond needs method + uint256 amount")
		}
		amount := new(uint256.Int).SetBytes(input[1:33])
		out, err := mod.Unbond(p.caller, amount)
		if err != nil {
			return nil, err
		}
		// Return funds from module escrow to caller.
		modBal := p.statedb.GetBalance(p.self)
		if modBal.Cmp(out) < 0 {
			return nil, fmt.Errorf("staking: module escrow insolvent")
		}
		p.statedb.SubBalance(p.self, out)
		p.statedb.AddBalancePrev(p.caller, out)
		return ethcommon.LeftPadBytes(out.ToBig().Bytes(), 32), nil

	case StakeMethodGetSelfStake:
		addr, err := readAddr(input)
		if err != nil {
			return nil, err
		}
		return u256Pad(mod.SelfStake(addr)), nil

	case StakeMethodGetVotingPower:
		addr, err := readAddr(input)
		if err != nil {
			return nil, err
		}
		return u256Pad(mod.VotingPower(addr)), nil

	case StakeMethodActiveCount:
		if len(input) != 1 {
			return nil, fmt.Errorf("staking: activeCount takes no args")
		}
		n := uint64(len(mod.ActiveSet()))
		return u256Pad(uint256.NewInt(n)), nil

	case StakeMethodActiveAt:
		if len(input) != 1+32 {
			return nil, fmt.Errorf("staking: activeAt needs index")
		}
		idx := new(uint256.Int).SetBytes(input[1:33]).Uint64()
		set := mod.ActiveSet()
		if idx >= uint64(len(set)) {
			return nil, fmt.Errorf("staking: activeAt out of range")
		}
		return ethcommon.LeftPadBytes(set[idx].Address.Bytes(), 32), nil

	case StakeMethodJail:
		// evidence: method || addr(20) || evidenceHash(32); hash must be non-zero
		if len(input) != 1+20+32 {
			return nil, fmt.Errorf("staking: jail needs address + evidence hash")
		}
		var addr crypto.Address
		copy(addr[:], input[1:21])
		var ev typesHash
		copy(ev[:], input[21:53])
		if ev.isZero() {
			return nil, fmt.Errorf("staking: empty evidence rejected")
		}
		// Fail closed on malformed; accept any non-zero evidence hash as placeholder
		// until full double-sign verification lands (must not silently ignore).
		mod.Jail(addr)
		return u256Pad(uint256.NewInt(1)), nil

	case StakeMethodIsJailed:
		addr, err := readAddr(input)
		if err != nil {
			return nil, err
		}
		v := uint64(0)
		if mod.IsJailed(addr) {
			v = 1
		}
		return u256Pad(uint256.NewInt(v)), nil

	default:
		return nil, fmt.Errorf("staking: unknown method 0x%02x", input[0])
	}
}

type typesHash [32]byte

func (h typesHash) isZero() bool {
	for _, b := range h {
		if b != 0 {
			return false
		}
	}
	return true
}

func readAddr(input []byte) (crypto.Address, error) {
	if len(input) != 1+20 {
		return crypto.Address{}, fmt.Errorf("staking: need method + 20-byte address")
	}
	var a crypto.Address
	copy(a[:], input[1:21])
	return a, nil
}

func u256Pad(v *uint256.Int) []byte {
	if v == nil {
		v = uint256.NewInt(0)
	}
	return ethcommon.LeftPadBytes(v.ToBig().Bytes(), 32)
}

// DewPrecompileAddresses returns addresses to warm in the access list when enabled.
func DewPrecompileAddresses() []ethcommon.Address {
	return []ethcommon.Address{NativeTransferPrecompile, StakingPrecompile}
}

// installDewPrecompiles copies the active fork precompiles and adds Dew system contracts.
func installDewPrecompiles(evm *ethvm.EVM, statedb *state.StateDB, enabled bool, caller crypto.Address, callValue *uint256.Int, stakingEnabled bool) {
	if !enabled {
		return
	}
	rules := evm.ChainConfig().Rules(evm.Context.BlockNumber, evm.Context.Random != nil, evm.Context.Time)
	base := ethvm.ActivePrecompiledContracts(rules)
	merged := make(ethvm.PrecompiledContracts, len(base)+2)
	for k, v := range base {
		merged[k] = v
	}
	var selfNative crypto.Address
	copy(selfNative[:], NativeTransferPrecompile[:])
	merged[NativeTransferPrecompile] = &nativeTransferPrecompile{
		statedb: statedb,
		self:    selfNative,
	}
	var selfStake crypto.Address
	copy(selfStake[:], StakingPrecompile[:])
	cv := uint256.NewInt(0)
	if callValue != nil {
		cv = new(uint256.Int).Set(callValue)
	}
	merged[StakingPrecompile] = &stakingPrecompile{
		statedb:   statedb,
		self:      selfStake,
		caller:    caller,
		callValue: cv,
		enabled:   stakingEnabled,
		cfg:       native.DefaultStakingConfig(),
	}
	evm.SetPrecompiles(merged)
}

// EnableDewPrecompiles sets the executor feature flag for 0x100+ precompiles.
func (e *Executor) EnableDewPrecompiles(v bool) {
	e.dewPrecompiles = v
}

// EnableStaking sets the C4 staking module flag for 0x102 (requires dew precompiles on).
func (e *Executor) EnableStaking(v bool) {
	e.stakingEnabled = v
}

// StakingEnabled reports whether 0x102 active methods are live.
func (e *Executor) StakingEnabled() bool {
	return e.stakingEnabled
}
