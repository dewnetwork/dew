package vm

import (
	"fmt"
	"math/big"

	ethcommon "github.com/ethereum/go-ethereum/common"
	ethcore "github.com/ethereum/go-ethereum/core"
	ethparams "github.com/ethereum/go-ethereum/params"
	ethvm "github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/consensus"
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
	// StakeMethodJail — input = [0x06 || voteA wire 114 || voteB wire 114] double-sign evidence (D3c).
	StakeMethodJail byte = 0x06
	// StakeMethodIsJailed — input = [0x07 || address 20]. returns 0/1 uint256
	StakeMethodIsJailed byte = 0x07
	// StakeMethodWithdraw — input = [0x08]. claim matured unbond (D3c).
	StakeMethodWithdraw byte = 0x08
	// StakeMethodPendingUnbond — input = [0x09 || address 20]. returns amount||unlockAt (64 bytes).
	StakeMethodPendingUnbond byte = 0x09
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

func (p *nativeTransferPrecompile) Name() string { return "DEW_NATIVE_TRANSFER" }

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

// stakeValueCtx records the latest value transfer into 0x102 for this ApplyMessage.
// EVM Transfer runs before precompile Run, so Bond can credit the immediate caller
// (nested CALL msg.sender) rather than only the top-level tx origin (D3c).
type stakeValueCtx struct {
	from   crypto.Address
	amount *uint256.Int
}

// stakingPrecompile implements 0x102 (Phase C4 / D3c).
type stakingPrecompile struct {
	statedb *state.StateDB
	self    crypto.Address
	// origin is the top-level tx sender (fallback for zero-value methods).
	origin crypto.Address
	// valueCtx is filled by the BlockContext.Transfer hook for payable calls.
	valueCtx *stakeValueCtx
	// blockTime is unix seconds from the current block header (for unbonding).
	blockTime uint64
	enabled   bool
	cfg       native.StakingConfig
}

// bondCaller returns immediate CALL payer when present, else tx origin.
func (p *stakingPrecompile) bondCaller() (crypto.Address, *uint256.Int, error) {
	if p.valueCtx != nil && p.valueCtx.amount != nil && !p.valueCtx.amount.IsZero() {
		return p.valueCtx.from, p.valueCtx.amount, nil
	}
	return crypto.Address{}, nil, fmt.Errorf("staking: bond requires non-zero value")
}

// actor for unbond/withdraw: prefer last value-payer is wrong; use origin for zero-value.
// Nested zero-value unbond still uses origin until EVM exposes call stack to precompiles.
func (p *stakingPrecompile) actor() crypto.Address {
	return p.origin
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
	case StakeMethodWithdraw:
		return params.StakingPrecompileGasUnbond
	case StakeMethodJail:
		return params.StakingPrecompileGasJail
	default:
		return params.StakingPrecompileGasQuery
	}
}

func (p *stakingPrecompile) Name() string { return "DEW_STAKING" }

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
		// CALLVALUE already transferred to 0x102 by EVM; credit immediate CALL payer (D3c nested).
		if len(input) != 1 {
			return nil, fmt.Errorf("staking: bond input must be single method byte")
		}
		bonder, amt, err := p.bondCaller()
		if err != nil {
			return nil, err
		}
		if err := mod.Bond(bonder, amt); err != nil {
			return nil, err
		}
		// Consume value ctx so a second bond in the same call path cannot double-credit.
		if p.valueCtx != nil {
			p.valueCtx.amount = uint256.NewInt(0)
		}
		return ethcommon.LeftPadBytes(amt.ToBig().Bytes(), 32), nil

	case StakeMethodUnbond:
		if len(input) != 1+32 {
			return nil, fmt.Errorf("staking: unbond needs method + uint256 amount")
		}
		amount := new(uint256.Int).SetBytes(input[1:33])
		// Queue unbonding; funds stay at module until Withdraw after period (D3c).
		if err := mod.Unbond(p.actor(), amount, p.blockTime); err != nil {
			return nil, err
		}
		return ethcommon.LeftPadBytes(amount.ToBig().Bytes(), 32), nil

	case StakeMethodWithdraw:
		if len(input) != 1 {
			return nil, fmt.Errorf("staking: withdraw takes no args")
		}
		out, err := mod.Withdraw(p.actor(), p.blockTime)
		if err != nil {
			return nil, err
		}
		modBal := p.statedb.GetBalance(p.self)
		if modBal.Cmp(out) < 0 {
			return nil, fmt.Errorf("staking: module escrow insolvent")
		}
		p.statedb.SubBalance(p.self, out)
		p.statedb.AddBalancePrev(p.actor(), out)
		return ethcommon.LeftPadBytes(out.ToBig().Bytes(), 32), nil

	case StakeMethodPendingUnbond:
		addr, err := readAddr(input)
		if err != nil {
			return nil, err
		}
		amt, unlock := mod.PendingUnbond(addr)
		out := make([]byte, 64)
		copy(out[0:32], ethcommon.LeftPadBytes(amt.ToBig().Bytes(), 32))
		copy(out[32:64], ethcommon.LeftPadBytes(new(big.Int).SetUint64(unlock).Bytes(), 32))
		return out, nil

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
		// D3c: method || voteA(114) || voteB(114) — dual signed votes, verified.
		const need = 1 + 2*consensus.VoteWireSize
		if len(input) != need {
			return nil, fmt.Errorf("staking: jail needs dual-vote evidence (%d bytes, got %d)", need, len(input))
		}
		ev, err := consensus.DecodeDoubleSignEvidenceWire(input[1:])
		if err != nil {
			return nil, fmt.Errorf("staking: evidence decode: %w", err)
		}
		if err := ev.Verify(); err != nil {
			return nil, fmt.Errorf("staking: evidence verify: %w", err)
		}
		mod.Jail(ev.Offender())
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
// valueCtx is shared with the BlockContext.Transfer hook for nested bond attribution.
func installDewPrecompiles(
	evm *ethvm.EVM,
	statedb *state.StateDB,
	enabled bool,
	origin crypto.Address,
	stakingEnabled bool,
	stakingCfg native.StakingConfig,
	valueCtx *stakeValueCtx,
) {
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
	merged[StakingPrecompile] = &stakingPrecompile{
		statedb:   statedb,
		self:      selfStake,
		origin:    origin,
		valueCtx:  valueCtx,
		blockTime: evm.Context.Time,
		enabled:   stakingEnabled,
		cfg:       stakingCfg,
	}
	evm.SetPrecompiles(merged)
}

// wrapStakingTransfer records value transfers into 0x102 for nested CALL bond (D3c).
func wrapStakingTransfer(valueCtx *stakeValueCtx) func(ethvm.StateDB, ethcommon.Address, ethcommon.Address, *uint256.Int, *ethparams.Rules) {
	return func(db ethvm.StateDB, from, to ethcommon.Address, amount *uint256.Int, rules *ethparams.Rules) {
		ethcore.Transfer(db, from, to, amount, rules)
		if valueCtx == nil || amount == nil || amount.IsZero() {
			return
		}
		if to != StakingPrecompile {
			return
		}
		valueCtx.from = fromEthAddr(from)
		valueCtx.amount = new(uint256.Int).Set(amount)
	}
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

// SetStakingConfig sets min stake / epoch / unbonding parameters used by 0x102.
func (e *Executor) SetStakingConfig(cfg native.StakingConfig) {
	e.stakingCfg = cfg
}

// StakingConfig returns the active staking module config.
func (e *Executor) StakingConfig() native.StakingConfig {
	return e.stakingCfg
}
