// Package mempool implements transaction admission for EVM and DewTx (Phase C1).
//
// Both transaction kinds share one pool surface: size limits, fee floors, and
// replace-by-fee. Underpriced / oversized payloads are rejected without blocking
// block production (admission is O(1) map work under the pool lock).
package mempool

import (
	"math/big"

	"github.com/dewnetwork/dew/params"
)

// Config is the configurable admission policy.
type Config struct {
	// MaxGlobal is the maximum number of pending txs in the pool (all senders).
	MaxGlobal int
	// MaxPerSender is the maximum pending txs from a single sender.
	MaxPerSender int
	// MaxTxBytes rejects payloads larger than this (raw wire size).
	MaxTxBytes int
	// MinGasPriceWei is the floor for EVM gas price / effective fee cap.
	// Legacy: gasPrice >= MinGasPriceWei.
	// EIP-1559: gasFeeCap >= MinGasPriceWei and gasTipCap >= MinTipWei.
	MinGasPriceWei *big.Int
	// MinTipWei is the minimum priority fee for EIP-1559 txs (anti-spam tip).
	MinTipWei *big.Int
	// MinDewFeeWei is the flat fee floor for native DewTx.
	MinDewFeeWei uint64
	// PriceBumpPercent is the minimum % increase required to replace an existing
	// same-sender/same-nonce tx (replace-by-fee). 10 means 10% higher.
	// Set 0 to disable RBF (replacements rejected).
	PriceBumpPercent uint64
}

// DefaultConfig returns Phase C1 defaults suitable for private testnet.
func DefaultConfig() Config {
	return Config{
		MaxGlobal:        4096,
		MaxPerSender:     16,
		MaxTxBytes:       128 << 10, // 128 KiB
		MinGasPriceWei:   big.NewInt(1_000_000_000), // 1 gwei
		MinTipWei:        big.NewInt(1),             // 1 wei tip floor
		MinDewFeeWei:     params.MinDewTxFeeWei,
		PriceBumpPercent: 10,
	}
}
