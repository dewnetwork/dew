package vm

import (
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/stateless"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethvm "github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/trie/utils"
	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/state"
	"github.com/dewnetwork/dew/crypto"
)

// Ensure Bridge implements ethvm.StateDB at compile time.
var _ ethvm.StateDB = (*Bridge)(nil)

// Bridge adapts Dew's flat StateDB to go-ethereum's vm.StateDB interface.
type Bridge struct {
	state *state.StateDB
	pc    *utils.PointCache
}

// NewBridge wraps a Dew StateDB for EVM execution.
func NewBridge(s *state.StateDB) *Bridge {
	return &Bridge{
		state: s,
		pc:    utils.NewPointCache(4096),
	}
}

// State returns the underlying Dew StateDB.
func (b *Bridge) State() *state.StateDB { return b.state }

func (b *Bridge) CreateAccount(addr ethcommon.Address) {
	b.state.CreateAccount(fromEthAddr(addr))
}

func (b *Bridge) CreateContract(addr ethcommon.Address) {
	b.state.CreateContract(fromEthAddr(addr))
}

func (b *Bridge) SubBalance(addr ethcommon.Address, amount *uint256.Int, _ tracing.BalanceChangeReason) uint256.Int {
	return b.state.SubBalance(fromEthAddr(addr), amount)
}

func (b *Bridge) AddBalance(addr ethcommon.Address, amount *uint256.Int, _ tracing.BalanceChangeReason) uint256.Int {
	return b.state.AddBalancePrev(fromEthAddr(addr), amount)
}

func (b *Bridge) GetBalance(addr ethcommon.Address) *uint256.Int {
	return b.state.GetBalance(fromEthAddr(addr))
}

func (b *Bridge) GetNonce(addr ethcommon.Address) uint64 {
	return b.state.GetNonce(fromEthAddr(addr))
}

func (b *Bridge) SetNonce(addr ethcommon.Address, nonce uint64) {
	b.state.SetNonceJournaled(fromEthAddr(addr), nonce)
}

func (b *Bridge) GetCodeHash(addr ethcommon.Address) ethcommon.Hash {
	return toEthHash(b.state.GetCodeHash(fromEthAddr(addr)))
}

func (b *Bridge) GetCode(addr ethcommon.Address) []byte {
	return b.state.GetCode(fromEthAddr(addr))
}

func (b *Bridge) SetCode(addr ethcommon.Address, code []byte) {
	b.state.SetCodeJournaled(fromEthAddr(addr), code)
}

func (b *Bridge) GetCodeSize(addr ethcommon.Address) int {
	return b.state.GetCodeSize(fromEthAddr(addr))
}

func (b *Bridge) AddRefund(gas uint64)  { b.state.AddRefund(gas) }
func (b *Bridge) SubRefund(gas uint64)  { b.state.SubRefund(gas) }
func (b *Bridge) GetRefund() uint64     { return b.state.GetRefund() }

func (b *Bridge) GetCommittedState(addr ethcommon.Address, hash ethcommon.Hash) ethcommon.Hash {
	return toEthHash(b.state.GetCommittedState(fromEthAddr(addr), fromEthHash(hash)))
}

func (b *Bridge) GetState(addr ethcommon.Address, hash ethcommon.Hash) ethcommon.Hash {
	return toEthHash(b.state.GetState(fromEthAddr(addr), fromEthHash(hash)))
}

func (b *Bridge) SetState(addr ethcommon.Address, key, value ethcommon.Hash) ethcommon.Hash {
	prev := b.state.SetStateJournaled(fromEthAddr(addr), fromEthHash(key), fromEthHash(value))
	return toEthHash(prev)
}

func (b *Bridge) GetStorageRoot(addr ethcommon.Address) ethcommon.Hash {
	return toEthHash(b.state.GetStorageRoot(fromEthAddr(addr)))
}

func (b *Bridge) GetTransientState(addr ethcommon.Address, key ethcommon.Hash) ethcommon.Hash {
	return toEthHash(b.state.GetTransientState(fromEthAddr(addr), fromEthHash(key)))
}

func (b *Bridge) SetTransientState(addr ethcommon.Address, key, value ethcommon.Hash) {
	b.state.SetTransientState(fromEthAddr(addr), fromEthHash(key), fromEthHash(value))
}

func (b *Bridge) SelfDestruct(addr ethcommon.Address) uint256.Int {
	return b.state.SelfDestruct(fromEthAddr(addr))
}

func (b *Bridge) HasSelfDestructed(addr ethcommon.Address) bool {
	return b.state.HasSelfDestructed(fromEthAddr(addr))
}

func (b *Bridge) SelfDestruct6780(addr ethcommon.Address) (uint256.Int, bool) {
	return b.state.SelfDestruct6780(fromEthAddr(addr))
}

func (b *Bridge) Exist(addr ethcommon.Address) bool {
	return b.state.ExistEVM(fromEthAddr(addr))
}

func (b *Bridge) Empty(addr ethcommon.Address) bool {
	return b.state.Empty(fromEthAddr(addr))
}

func (b *Bridge) AddressInAccessList(addr ethcommon.Address) bool {
	return b.state.AddressInAccessList(fromEthAddr(addr))
}

func (b *Bridge) SlotInAccessList(addr ethcommon.Address, slot ethcommon.Hash) (addressOk bool, slotOk bool) {
	return b.state.SlotInAccessList(fromEthAddr(addr), fromEthHash(slot))
}

func (b *Bridge) AddAddressToAccessList(addr ethcommon.Address) {
	b.state.AddAddressToAccessList(fromEthAddr(addr))
}

func (b *Bridge) AddSlotToAccessList(addr ethcommon.Address, slot ethcommon.Hash) {
	b.state.AddSlotToAccessList(fromEthAddr(addr), fromEthHash(slot))
}

func (b *Bridge) PointCache() *utils.PointCache { return b.pc }

func (b *Bridge) Prepare(
	rules params.Rules,
	sender, coinbase ethcommon.Address,
	dest *ethcommon.Address,
	precompiles []ethcommon.Address,
	txAccesses ethtypes.AccessList,
) {
	var destAddr *crypto.Address
	if dest != nil {
		a := fromEthAddr(*dest)
		destAddr = &a
	}
	pre := make([]crypto.Address, len(precompiles))
	for i, p := range precompiles {
		pre[i] = fromEthAddr(p)
	}
	b.state.Prepare(fromEthAddr(sender), fromEthAddr(coinbase), destAddr, pre)
	// Berlin access list from tx
	if rules.IsBerlin {
		for _, d := range txAccesses {
			b.AddAddressToAccessList(d.Address)
			for _, key := range d.StorageKeys {
				b.AddSlotToAccessList(d.Address, key)
			}
		}
	}
	_ = rules
}

func (b *Bridge) RevertToSnapshot(id int) { b.state.RevertToSnapshot(id) }
func (b *Bridge) Snapshot() int           { return b.state.Snapshot() }

func (b *Bridge) AddLog(log *ethtypes.Log) {
	b.state.AddLog(fromEthLog(log))
}

func (b *Bridge) AddPreimage(_ ethcommon.Hash, _ []byte) {
	// preimage recording not required for Phase A
}

func (b *Bridge) Witness() *stateless.Witness { return nil }

func (b *Bridge) Finalise(deleteEmptyObjects bool) {
	b.state.Finalise(deleteEmptyObjects)
}
