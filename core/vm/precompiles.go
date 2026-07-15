package vm

import (
	"fmt"
	"math/big"

	ethcommon "github.com/ethereum/go-ethereum/common"
	ethcore "github.com/ethereum/go-ethereum/core"
	ethvm "github.com/ethereum/go-ethereum/core/vm"
	ethparams "github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/consensus"
	"github.com/dewnetwork/dew/core/native"
	"github.com/dewnetwork/dew/core/state"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/params"
)

// Dew system precompile addresses (low slots from 0x100).
// Formal registry: DewPrecompileSlots(); params freeze numbers match these lows.
var (
	// NativeTransferPrecompile is 0x100 — forward CALLVALUE to a recipient (active).
	NativeTransferPrecompile = ethcommon.BytesToAddress([]byte{0x01, 0x00})
	// NativeSwapPrecompile is 0x101 — limit orderbook (flagged; methods need EnableNativeSwap).
	NativeSwapPrecompile = ethcommon.BytesToAddress([]byte{0x01, 0x01})
	// ReservedNativeSwapPrecompile is a historical alias for NativeSwapPrecompile (0x101).
	ReservedNativeSwapPrecompile = NativeSwapPrecompile
	// StakingPrecompile is 0x102 — staking entrypoint (flagged; methods need EnableStaking).
	StakingPrecompile = ethcommon.BytesToAddress([]byte{0x01, 0x02})
)

// NativeTransferGas is the fixed gas schedule for 0x100 (active only).
// Reserved slots have no live gas schedule.
const NativeTransferGas uint64 = params.NativeTransferPrecompileGas

// PrecompileSlotStatus is the public-testnet-v1 registry status for a Dew slot.
type PrecompileSlotStatus string

const (
	// SlotActive is registered and live when Dew precompiles are on.
	SlotActive PrecompileSlotStatus = "active"
	// SlotReserved is allocated in docs/params but not in the live map.
	SlotReserved PrecompileSlotStatus = "reserved"
	// SlotFlagged is in the live map; methods are gated by a feature flag.
	SlotFlagged PrecompileSlotStatus = "flagged"
)

// DewPrecompileSlot describes one Dew system precompile address (S6 registry).
type DewPrecompileSlot struct {
	// Address is the 20-byte EVM address.
	Address ethcommon.Address
	// Name is a stable identifier for docs and tooling.
	Name string
	// Status is active, reserved, or flagged.
	Status PrecompileSlotStatus
	// LiveInMap is true when installDewPrecompiles registers an implementation.
	LiveInMap bool
	// LowAddr is the uint16 slot number (0x100, 0x101, …).
	LowAddr uint16
}

// DewPrecompileSlots returns the formal Dew precompile registry (ascending by address).
// Only slots with LiveInMap are installed when EnableDewPrecompiles is on.
func DewPrecompileSlots() []DewPrecompileSlot {
	return []DewPrecompileSlot{
		{
			Address:   NativeTransferPrecompile,
			Name:      "native_transfer",
			Status:    SlotActive,
			LiveInMap: true,
			LowAddr:   params.PrecompileNativeTransferAddr,
		},
		{
			Address:   NativeSwapPrecompile,
			Name:      "native_swap",
			Status:    SlotFlagged,
			LiveInMap: true,
			LowAddr:   params.PrecompileNativeSwapReservedAddr,
		},
		{
			Address:   StakingPrecompile,
			Name:      "staking",
			Status:    SlotFlagged,
			LiveInMap: true,
			LowAddr:   params.PrecompileStakingAddr,
		},
	}
}

// NextFreeDewPrecompileSlot is the next unallocated low address for a new Dew precompile.
// Under public-testnet-v1, activating it on a live network requires a hardfork doc.
const NextFreeDewPrecompileSlot uint16 = params.PrecompileNextFreeAddr

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
	// StakeMethodDelegate — payable. input = [0x0a || validator 20]. Credits value-payer.
	StakeMethodDelegate byte = 0x0a
	// StakeMethodUndelegate — input = [0x0b || validator 20 || amount u256]. Actor tx.origin.
	StakeMethodUndelegate byte = 0x0b
	// StakeMethodWithdrawDelegation — input = [0x0c || validator 20]. Actor tx.origin.
	StakeMethodWithdrawDelegation byte = 0x0c
	// StakeMethodSetCommission — input = [0x0d || bps u256]. Actor tx.origin = validator.
	StakeMethodSetCommission byte = 0x0d
	// StakeMethodGetDelegation — input = [0x0e || validator 20 || delegator 20].
	StakeMethodGetDelegation byte = 0x0e
	// StakeMethodGetCommission — input = [0x0f || validator 20].
	StakeMethodGetCommission byte = 0x0f
	// StakeMethodGetDelegatedTotal — input = [0x10 || validator 20].
	StakeMethodGetDelegatedTotal byte = 0x10
	// StakeMethodPendingUndelegation — input = [0x11 || validator 20 || delegator 20].
	StakeMethodPendingUndelegation byte = 0x11
	// StakeMethodClaimRewards — input = [0x12 || validator 20]. Actor tx.origin.
	StakeMethodClaimRewards byte = 0x12
	// StakeMethodPendingRewards — input = [0x13 || validator 20 || delegator 20].
	StakeMethodPendingRewards byte = 0x13
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

// bondCaller returns the immediate CALL value-payer (nested msg.sender) recorded by
// wrapStakingTransfer. Bond requires non-zero CALLVALUE into 0x102.
func (p *stakingPrecompile) bondCaller() (crypto.Address, *uint256.Int, error) {
	if p.valueCtx != nil && p.valueCtx.amount != nil && !p.valueCtx.amount.IsZero() {
		return p.valueCtx.from, p.valueCtx.amount, nil
	}
	return crypto.Address{}, nil, fmt.Errorf("staking: bond requires non-zero value")
}

// actor returns the stake account for zero-value methods (unbond / withdraw).
//
// Fail-closed (S4 / public-testnet-v1): always the top-level tx.origin.
// go-ethereum PrecompiledContract.Run has no call-stack / msg.sender, so a nested
// contract CALL with value=0 cannot be attributed to the intermediate contract.
// Nested payable bond is correct via Transfer hook; nested unbond/withdraw of a
// contract's own stake is unsupported until a hardfork exposes call depth or an
// explicit address argument (ABI change). Do not invent call-stack heuristics.
func (p *stakingPrecompile) actor() crypto.Address {
	return p.origin
}

func (p *stakingPrecompile) RequiredGas(input []byte) uint64 {
	if len(input) == 0 {
		return params.StakingPrecompileGasQuery
	}
	switch input[0] {
	case StakeMethodBond, StakeMethodDelegate:
		return params.StakingPrecompileGasBond
	case StakeMethodUnbond, StakeMethodWithdraw, StakeMethodUndelegate, StakeMethodWithdrawDelegation, StakeMethodClaimRewards:
		return params.StakingPrecompileGasClaimRewards
	case StakeMethodJail:
		return params.StakingPrecompileGasJail
	case StakeMethodSetCommission:
		return params.StakingPrecompileGasSetCommission
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
		// Applies provisional double-sign slash burn then jails.
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
		burned := mod.SlashAndJail(ev.Offender())
		return u256Pad(burned), nil

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

	case StakeMethodDelegate:
		// CALLVALUE already at 0x102; credit immediate value-payer.
		if len(input) != 1+20 {
			return nil, fmt.Errorf("staking: delegate needs method + validator 20")
		}
		var validator crypto.Address
		copy(validator[:], input[1:21])
		delegator, amt, err := p.bondCaller()
		if err != nil {
			return nil, fmt.Errorf("staking: delegate requires non-zero value")
		}
		if err := mod.Delegate(validator, delegator, amt); err != nil {
			return nil, err
		}
		if p.valueCtx != nil {
			p.valueCtx.amount = uint256.NewInt(0)
		}
		return u256Pad(amt), nil

	case StakeMethodUndelegate:
		if len(input) != 1+20+32 {
			return nil, fmt.Errorf("staking: undelegate needs validator 20 + amount u256")
		}
		var validator crypto.Address
		copy(validator[:], input[1:21])
		amount := new(uint256.Int).SetBytes(input[21:53])
		if err := mod.Undelegate(validator, p.actor(), amount, p.blockTime); err != nil {
			return nil, err
		}
		return u256Pad(amount), nil

	case StakeMethodWithdrawDelegation:
		if len(input) != 1+20 {
			return nil, fmt.Errorf("staking: withdrawDelegation needs validator 20")
		}
		var validator crypto.Address
		copy(validator[:], input[1:21])
		out, err := mod.WithdrawDelegation(validator, p.actor(), p.blockTime)
		if err != nil {
			return nil, err
		}
		modBal := p.statedb.GetBalance(p.self)
		if modBal.Cmp(out) < 0 {
			return nil, fmt.Errorf("staking: module escrow insolvent")
		}
		p.statedb.SubBalance(p.self, out)
		p.statedb.AddBalancePrev(p.actor(), out)
		return u256Pad(out), nil

	case StakeMethodSetCommission:
		if len(input) != 1+32 {
			return nil, fmt.Errorf("staking: setCommission needs bps u256")
		}
		bps := new(uint256.Int).SetBytes(input[1:33]).Uint64()
		if err := mod.SetCommission(p.actor(), bps); err != nil {
			return nil, err
		}
		return u256Pad(uint256.NewInt(bps)), nil

	case StakeMethodGetDelegation:
		val, del, err := readTwoAddr(input)
		if err != nil {
			return nil, err
		}
		return u256Pad(mod.Delegation(val, del)), nil

	case StakeMethodGetCommission:
		addr, err := readAddr(input)
		if err != nil {
			return nil, err
		}
		return u256Pad(uint256.NewInt(mod.CommissionBps(addr))), nil

	case StakeMethodGetDelegatedTotal:
		addr, err := readAddr(input)
		if err != nil {
			return nil, err
		}
		return u256Pad(mod.DelegatedTotal(addr)), nil

	case StakeMethodPendingUndelegation:
		val, del, err := readTwoAddr(input)
		if err != nil {
			return nil, err
		}
		amt, unlock := mod.PendingUndelegation(val, del)
		out := make([]byte, 64)
		copy(out[0:32], ethcommon.LeftPadBytes(amt.ToBig().Bytes(), 32))
		copy(out[32:64], ethcommon.LeftPadBytes(new(big.Int).SetUint64(unlock).Bytes(), 32))
		return out, nil

	case StakeMethodClaimRewards:
		// [0x12 || validator 20] — credit matured rewards to tx.origin from module escrow.
		if len(input) != 1+20 {
			return nil, fmt.Errorf("staking: claimRewards needs method + validator 20")
		}
		var validator crypto.Address
		copy(validator[:], input[1:21])
		who := p.actor()
		amt, err := mod.ClaimRewards(validator, who)
		if err != nil {
			return nil, err
		}
		modBal := p.statedb.GetBalance(p.self)
		if modBal.Cmp(amt) < 0 {
			return nil, fmt.Errorf("staking: module reward escrow insolvent")
		}
		p.statedb.SubBalance(p.self, amt)
		p.statedb.AddBalancePrev(who, amt)
		return u256Pad(amt), nil

	case StakeMethodPendingRewards:
		val, del, err := readTwoAddr(input)
		if err != nil {
			return nil, err
		}
		return u256Pad(mod.PendingRewards(val, del)), nil

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

func readTwoAddr(input []byte) (a, b crypto.Address, err error) {
	if len(input) != 1+40 {
		return crypto.Address{}, crypto.Address{}, fmt.Errorf("staking: need method + two 20-byte addresses")
	}
	copy(a[:], input[1:21])
	copy(b[:], input[21:41])
	return a, b, nil
}

func u256Pad(v *uint256.Int) []byte {
	if v == nil {
		v = uint256.NewInt(0)
	}
	return ethcommon.LeftPadBytes(v.ToBig().Bytes(), 32)
}

// DewPrecompileAddresses returns live-map Dew precompile addresses to warm in the
// access list when enabled.
func DewPrecompileAddresses() []ethcommon.Address {
	slots := DewPrecompileSlots()
	out := make([]ethcommon.Address, 0, len(slots))
	for _, s := range slots {
		if s.LiveInMap {
			out = append(out, s.Address)
		}
	}
	return out
}

// installDewPrecompiles copies the active fork precompiles and adds live Dew system
// contracts from DewPrecompileSlots (LiveInMap only).
// stakeVal / obVal are shared with the BlockContext.Transfer hook for payable methods.
func installDewPrecompiles(
	evm *ethvm.EVM,
	statedb *state.StateDB,
	enabled bool,
	origin crypto.Address,
	stakingEnabled bool,
	stakingCfg native.StakingConfig,
	nativeSwapEnabled bool,
	stakeVal *stakeValueCtx,
	obVal *stakeValueCtx,
) {
	if !enabled {
		return
	}
	rules := evm.ChainConfig().Rules(evm.Context.BlockNumber, evm.Context.Random != nil, evm.Context.Time)
	base := ethvm.ActivePrecompiledContracts(rules)
	live := DewPrecompileAddresses()
	merged := make(ethvm.PrecompiledContracts, len(base)+len(live))
	for k, v := range base {
		merged[k] = v
	}
	var selfNative crypto.Address
	copy(selfNative[:], NativeTransferPrecompile[:])
	merged[NativeTransferPrecompile] = &nativeTransferPrecompile{
		statedb: statedb,
		self:    selfNative,
	}
	var selfSwap crypto.Address
	copy(selfSwap[:], NativeSwapPrecompile[:])
	blockNum := uint64(0)
	if evm.Context.BlockNumber != nil {
		blockNum = evm.Context.BlockNumber.Uint64()
	}
	merged[NativeSwapPrecompile] = &orderbookPrecompile{
		statedb:  statedb,
		self:     selfSwap,
		origin:   origin,
		evm:      evm,
		valueCtx: obVal,
		blockNum: blockNum,
		enabled:  nativeSwapEnabled,
	}
	var selfStake crypto.Address
	copy(selfStake[:], StakingPrecompile[:])
	merged[StakingPrecompile] = &stakingPrecompile{
		statedb:   statedb,
		self:      selfStake,
		origin:    origin,
		valueCtx:  stakeVal,
		blockTime: evm.Context.Time,
		enabled:   stakingEnabled,
		cfg:       stakingCfg,
	}
	evm.SetPrecompiles(merged)
}

// wrapDewPrecompileTransfer records value transfers into 0x102 / 0x101 for payable methods.
func wrapDewPrecompileTransfer(stakeVal, obVal *stakeValueCtx) func(ethvm.StateDB, ethcommon.Address, ethcommon.Address, *uint256.Int, *ethparams.Rules) {
	return func(db ethvm.StateDB, from, to ethcommon.Address, amount *uint256.Int, rules *ethparams.Rules) {
		ethcore.Transfer(db, from, to, amount, rules)
		if amount == nil || amount.IsZero() {
			return
		}
		if to == StakingPrecompile && stakeVal != nil {
			stakeVal.from = fromEthAddr(from)
			stakeVal.amount = new(uint256.Int).Set(amount)
			return
		}
		if to == NativeSwapPrecompile && obVal != nil {
			obVal.from = fromEthAddr(from)
			obVal.amount = new(uint256.Int).Set(amount)
		}
	}
}

// wrapStakingTransfer is kept for older call sites; prefers stake-only recording.
func wrapStakingTransfer(valueCtx *stakeValueCtx) func(ethvm.StateDB, ethcommon.Address, ethcommon.Address, *uint256.Int, *ethparams.Rules) {
	return wrapDewPrecompileTransfer(valueCtx, nil)
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

// EnableNativeSwap sets the 0x101 orderbook methods flag (requires dew precompiles on).
func (e *Executor) EnableNativeSwap(v bool) {
	e.nativeSwapEnabled = v
}

// NativeSwapEnabled reports whether 0x101 orderbook methods are live.
func (e *Executor) NativeSwapEnabled() bool {
	return e.nativeSwapEnabled
}

// SetStakingConfig sets min stake / epoch / unbonding parameters used by 0x102.
func (e *Executor) SetStakingConfig(cfg native.StakingConfig) {
	e.stakingCfg = cfg
}

// StakingConfig returns the active staking module config.
func (e *Executor) StakingConfig() native.StakingConfig {
	return e.stakingCfg
}
