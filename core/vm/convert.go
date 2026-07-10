package vm

import (
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
)

func toEthAddr(a crypto.Address) ethcommon.Address {
	return ethcommon.BytesToAddress(a[:])
}

func fromEthAddr(a ethcommon.Address) crypto.Address {
	var out crypto.Address
	copy(out[:], a[:])
	return out
}

func toEthHash(h types.Hash) ethcommon.Hash {
	return ethcommon.BytesToHash(h[:])
}

func fromEthHash(h ethcommon.Hash) types.Hash {
	return types.BytesToHash(h[:])
}

func fromEthLog(l *ethtypes.Log) *types.Log {
	if l == nil {
		return nil
	}
	topics := make([]types.Hash, len(l.Topics))
	for i, t := range l.Topics {
		topics[i] = fromEthHash(t)
	}
	return &types.Log{
		Address: fromEthAddr(l.Address),
		Topics:  topics,
		Data:    append([]byte(nil), l.Data...),
	}
}
