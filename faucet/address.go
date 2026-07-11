package faucet

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

// normalizeAddress returns lowercase 0x-prefixed 40-hex address.
func normalizeAddress(s string) (string, error) {
	s = strings.TrimSpace(s)
	if !common.IsHexAddress(s) {
		return "", fmt.Errorf("invalid address")
	}
	return strings.ToLower(common.HexToAddress(s).Hex()), nil
}
