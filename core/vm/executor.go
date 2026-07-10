// Package vm embeds go-ethereum's EVM behind a Dew StateDB bridge.
package vm

import (
	"fmt"
	"math/big"

	ethcommon "github.com/ethereum/go-ethereum/common"
	ethcore "github.com/ethereum/go-ethereum/core"
	ethvm "github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/state"
	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
)

// DefaultChainConfig is Cancun-era rules from genesis (chain id 2026).
func DefaultChainConfig(chainID *big.Int) *params.ChainConfig {
	if chainID == nil {
		chainID = big.NewInt(2026)
	}
	zero := uint64(0)
	return &params.ChainConfig{
		ChainID:                 new(big.Int).Set(chainID),
		HomesteadBlock:          big.NewInt(0),
		EIP150Block:             big.NewInt(0),
		EIP155Block:             big.NewInt(0),
		EIP158Block:             big.NewInt(0),
		ByzantiumBlock:          big.NewInt(0),
		ConstantinopleBlock:     big.NewInt(0),
		PetersburgBlock:         big.NewInt(0),
		IstanbulBlock:           big.NewInt(0),
		MuirGlacierBlock:        big.NewInt(0),
		BerlinBlock:             big.NewInt(0),
		LondonBlock:             big.NewInt(0),
		ArrowGlacierBlock:       big.NewInt(0),
		GrayGlacierBlock:       big.NewInt(0),
		MergeNetsplitBlock:      big.NewInt(0),
		ShanghaiTime:            &zero,
		CancunTime:              &zero,
		TerminalTotalDifficulty: big.NewInt(0),
	}
}

// BlockContext carries block-level EVM inputs.
type BlockContext struct {
	Number    uint64
	Time      uint64
	GasLimit  uint64
	BaseFee   *big.Int
	Coinbase  crypto.Address
	ChainID   *big.Int
	GetHashFn func(uint64) ethcommon.Hash
}

// Message is a single sequential execution request (create or call).
type Message struct {
	From     crypto.Address
	To       *crypto.Address // nil = contract creation
	Value    *uint256.Int
	GasLimit uint64
	GasPrice *big.Int // effective gas price paid by sender
	Data     []byte
	// NoFinalise skips StateDB.Finalise so the caller can RevertToSnapshot
	// (used by eth_call / eth_estimateGas).
	NoFinalise bool
}

// Result is the outcome of applying a message.
type Result struct {
	UsedGas         uint64
	MaxUsedGas      uint64
	Err             error
	ReturnData      []byte
	ContractAddress *crypto.Address
	Logs            []*types.Log
	Failed          bool
}

// Executor runs messages sequentially against a Dew StateDB.
type Executor struct {
	statedb *state.StateDB
	bridge  *Bridge
	config  *params.ChainConfig
	block   BlockContext
}

// NewExecutor builds an executor for the given block context.
func NewExecutor(statedb *state.StateDB, block BlockContext) *Executor {
	if block.BaseFee == nil {
		block.BaseFee = big.NewInt(0)
	}
	if block.GasLimit == 0 {
		block.GasLimit = 30_000_000
	}
	if block.GetHashFn == nil {
		block.GetHashFn = func(uint64) ethcommon.Hash { return ethcommon.Hash{} }
	}
	return &Executor{
		statedb: statedb,
		bridge:  NewBridge(statedb),
		config:  DefaultChainConfig(block.ChainID),
		block:   block,
	}
}

// ApplyMessage executes one create/call with Cancun EVM rules.
// Gas is prepaid; unused gas is refunded to the sender (value not including tip split).
func (e *Executor) ApplyMessage(msg Message) (*Result, error) {
	if msg.Value == nil {
		msg.Value = uint256.NewInt(0)
	}
	if msg.GasPrice == nil {
		msg.GasPrice = big.NewInt(0)
	}
	if msg.GasLimit == 0 {
		return nil, fmt.Errorf("vm: gas limit is zero")
	}

	// Buy gas: deduct gasLimit * gasPrice up front.
	gasCost := new(uint256.Int)
	if overflow := gasCost.SetFromBig(new(big.Int).Mul(new(big.Int).SetUint64(msg.GasLimit), msg.GasPrice)); overflow {
		return nil, fmt.Errorf("vm: gas cost overflow")
	}
	if e.statedb.GetBalance(msg.From).Cmp(gasCost) < 0 {
		return nil, fmt.Errorf("vm: insufficient balance for gas")
	}
	// Also need value
	need := new(uint256.Int).Add(gasCost, msg.Value)
	if e.statedb.GetBalance(msg.From).Cmp(need) < 0 {
		return nil, fmt.Errorf("vm: insufficient balance for gas + value")
	}
	e.statedb.SubBalance(msg.From, gasCost)

	// Clear logs for this tx
	e.statedb.ClearLogs()

	// Random non-nil → merge/shanghai/cancun rules active
	random := ethcommon.Hash{0x01}
	blockCtx := ethvm.BlockContext{
		CanTransfer: ethcore.CanTransfer,
		Transfer:    ethcore.Transfer,
		GetHash:     e.block.GetHashFn,
		Coinbase:    toEthAddr(e.block.Coinbase),
		GasLimit:    e.block.GasLimit,
		BlockNumber: new(big.Int).SetUint64(e.block.Number),
		Time:        e.block.Time,
		Difficulty:  big.NewInt(0),
		BaseFee:     e.block.BaseFee,
		BlobBaseFee: big.NewInt(0),
		Random:      &random,
	}
	txCtx := ethvm.TxContext{
		Origin:   toEthAddr(msg.From),
		GasPrice: msg.GasPrice,
	}

	evm := ethvm.NewEVM(blockCtx, txCtx, e.bridge, e.config, ethvm.Config{})

	// Prepare access lists (Berlin+)
	rules := e.config.Rules(blockCtx.BlockNumber, true, blockCtx.Time)
	var dest *ethcommon.Address
	if msg.To != nil {
		a := toEthAddr(*msg.To)
		dest = &a
	}
	// Warm standard Cancun precompiles (0x01–0x0a).
	precompiles := make([]ethcommon.Address, 0, 10)
	for i := byte(1); i <= 10; i++ {
		precompiles = append(precompiles, ethcommon.BytesToAddress([]byte{i}))
	}
	e.bridge.Prepare(rules, toEthAddr(msg.From), toEthAddr(e.block.Coinbase), dest, precompiles, nil)
	_ = evm // used below

	gasLeft := msg.GasLimit
	var (
		ret         []byte
		err         error
		createdAddr *crypto.Address
	)

	caller := ethvm.AccountRef(toEthAddr(msg.From))
	if msg.To == nil {
		// CREATE increments nonce inside the EVM.
		var ethAddr ethcommon.Address
		ret, ethAddr, gasLeft, err = evm.Create(caller, msg.Data, gasLeft, msg.Value)
		if err == nil {
			a := fromEthAddr(ethAddr)
			createdAddr = &a
		}
	} else {
		// CALL: nonce is incremented by the state transition (even on revert).
		e.statedb.SetNonceJournaled(msg.From, e.statedb.GetNonce(msg.From)+1)
		ret, gasLeft, err = evm.Call(caller, toEthAddr(*msg.To), msg.Data, gasLeft, msg.Value)
	}

	usedGas := msg.GasLimit - gasLeft

	// Apply refund (capped at usedGas/5 post London)
	refund := e.statedb.GetRefund()
	if max := usedGas / 5; refund > max {
		refund = max
	}
	gasLeft += refund
	if gasLeft > msg.GasLimit {
		gasLeft = msg.GasLimit
	}
	usedGas = msg.GasLimit - gasLeft

	// Refund unused gas value to sender
	if msg.GasPrice.Sign() > 0 && gasLeft > 0 {
		refundVal := new(uint256.Int)
		_ = refundVal.SetFromBig(new(big.Int).Mul(new(big.Int).SetUint64(gasLeft), msg.GasPrice))
		e.statedb.AddBalancePrev(msg.From, refundVal)
	}

	// Coinbase tip: for simplicity GasPrice is effective tip when BaseFee=0
	if msg.GasPrice.Sign() > 0 && usedGas > 0 {
		tip := new(uint256.Int)
		_ = tip.SetFromBig(new(big.Int).Mul(new(big.Int).SetUint64(usedGas), msg.GasPrice))
		e.statedb.AddBalancePrev(e.block.Coinbase, tip)
	}

	failed := err != nil
	if !msg.NoFinalise {
		// Finalise clears the journal; only do this for committed transactions.
		e.bridge.Finalise(true)
	}

	logs := e.statedb.Logs()
	// copy logs
	outLogs := make([]*types.Log, len(logs))
	copy(outLogs, logs)
	e.statedb.ClearLogs()

	return &Result{
		UsedGas:         usedGas,
		MaxUsedGas:      usedGas,
		Err:             err,
		ReturnData:      ret,
		ContractAddress: createdAddr,
		Logs:            outLogs,
		Failed:          failed,
	}, nil
}


