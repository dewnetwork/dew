package rpc

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
)

// EncodeUint64 returns 0x-prefixed hex for a uint64.
func EncodeUint64(n uint64) string {
	return fmt.Sprintf("0x%x", n)
}

// EncodeBig returns 0x-prefixed hex for a big.Int (0 → "0x0").
func EncodeBig(n *big.Int) string {
	if n == nil || n.Sign() == 0 {
		return "0x0"
	}
	return "0x" + strings.TrimPrefix(fmt.Sprintf("%x", n), "0")
}

// EncodeBytes returns 0x-prefixed hex (empty → "0x").
func EncodeBytes(b []byte) string {
	if len(b) == 0 {
		return "0x"
	}
	return "0x" + hex.EncodeToString(b)
}

// EncodeHash returns 0x + 64 hex chars.
func EncodeHash(h types.Hash) string {
	return h.Hex()
}

// EncodeAddress returns EIP-55 checksum address.
func EncodeAddress(a crypto.Address) string {
	return a.Hex()
}

// DecodeUint64 parses 0x hex or decimal.
func DecodeUint64(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty number")
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		return strconv.ParseUint(s[2:], 16, 64)
	}
	return strconv.ParseUint(s, 10, 64)
}

// DecodeBig parses 0x hex or decimal into *big.Int.
func DecodeBig(s string) (*big.Int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return big.NewInt(0), nil
	}
	n, ok := new(big.Int).SetString(s, 0)
	if !ok {
		return nil, fmt.Errorf("invalid big integer %q", s)
	}
	return n, nil
}

// DecodeBytes parses 0x-hex bytes.
func DecodeBytes(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0x" {
		return []byte{}, nil
	}
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	if len(s)%2 == 1 {
		s = "0" + s
	}
	return hex.DecodeString(s)
}

// DecodeAddress parses a 20-byte address.
func DecodeAddress(s string) (crypto.Address, error) {
	return crypto.HexToAddress(s)
}

// DecodeHash parses a 32-byte hash.
// Short hex (e.g. storage slot "0x0" / "0x1a") is left-padded to 32 bytes so
// clients like Foundry/geth match Ethereum JSON-RPC quantity-style slots.
func DecodeHash(s string) (types.Hash, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	if s == "" {
		return types.Hash{}, nil
	}
	if len(s)%2 == 1 {
		s = "0" + s
	}
	if len(s) > 64 {
		return types.Hash{}, fmt.Errorf("types: hash must be at most 32 bytes (64 hex chars), got len %d", len(s))
	}
	if len(s) < 64 {
		s = strings.Repeat("0", 64-len(s)) + s
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return types.Hash{}, fmt.Errorf("types: invalid hash hex: %w", err)
	}
	return types.BytesToHash(b), nil
}

// BlockNumberTag is a block height or tag.
type BlockNumberTag int64

const (
	// LatestBlock is "latest"
	LatestBlock BlockNumberTag = -1
	// EarliestBlock is "earliest" (genesis)
	EarliestBlock BlockNumberTag = -2
	// PendingBlock is "pending" (treated as latest in Phase A4)
	PendingBlock BlockNumberTag = -3
)

// ParseBlockNumber parses a block tag or hex number.
func ParseBlockNumber(v interface{}) (BlockNumberTag, error) {
	switch x := v.(type) {
	case nil:
		return LatestBlock, nil
	case string:
		switch strings.ToLower(x) {
		case "latest", "":
			return LatestBlock, nil
		case "earliest":
			return EarliestBlock, nil
		case "pending":
			return PendingBlock, nil
		default:
			n, err := DecodeUint64(x)
			if err != nil {
				return 0, err
			}
			// BlockNumberTag is int64; reject values that would wrap on cast.
			if n > math.MaxInt64 {
				return 0, fmt.Errorf("block number %d exceeds max int64", n)
			}
			return BlockNumberTag(n), nil
		}
	case float64:
		// JSON numbers — only finite, non-negative values in int64 range.
		if math.IsNaN(x) || math.IsInf(x, 0) || x < 0 || x > float64(math.MaxInt64) {
			return 0, fmt.Errorf("invalid block number %v", x)
		}
		return BlockNumberTag(int64(x)), nil
	case json.Number:
		n, err := x.Int64()
		if err != nil {
			return 0, err
		}
		if n < 0 {
			return 0, fmt.Errorf("invalid block number %d", n)
		}
		return BlockNumberTag(n), nil
	default:
		return 0, fmt.Errorf("invalid block number %T", v)
	}
}

// ResolveBlockNumber maps a tag to a concrete height given the head.
func ResolveBlockNumber(tag BlockNumberTag, head uint64) (uint64, error) {
	switch tag {
	case LatestBlock, PendingBlock:
		return head, nil
	case EarliestBlock:
		return 0, nil
	default:
		if tag < 0 {
			return 0, fmt.Errorf("invalid block tag")
		}
		n := uint64(tag)
		if n > head {
			return 0, fmt.Errorf("block %d not found (head %d)", n, head)
		}
		return n, nil
	}
}
