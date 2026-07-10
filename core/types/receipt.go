package types

import (
	"math/big"

	"github.com/dewnetwork/dew/crypto"
)

// Receipt is produced for every included EVM transaction.
type Receipt struct {
	Type              byte
	Status            uint64 // 1 success / 0 failure
	CumulativeGasUsed uint64
	GasUsed           uint64
	EffectiveGasPrice *big.Int
	Logs              []*Log
	ContractAddress   *crypto.Address
	TxHash            Hash
	BlockHash         Hash
	BlockNumber       uint64
	TransactionIndex  uint
}

// Log is an EVM event log.
type Log struct {
	Address crypto.Address
	Topics  []Hash
	Data    []byte
}
