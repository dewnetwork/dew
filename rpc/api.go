package rpc

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/core/vm"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/node"
	"github.com/dewnetwork/dew/version"
)

// API binds Ethereum JSON-RPC methods to a Node backend.
type API struct {
	n *node.Node
}

// NewAPI creates the eth/net/web3 method set.
func NewAPI(n *node.Node) *API {
	return &API{n: n}
}

// Handlers returns all Phase A required method handlers plus Phase B dew_* extensions.
func (a *API) Handlers() map[string]Handler {
	return map[string]Handler{
		// identity
		"web3_clientVersion": a.web3ClientVersion,
		"net_version":        a.netVersion,
		"net_listening":      a.netListening,
		"net_peerCount":      a.netPeerCount,
		"eth_chainId":        a.ethChainId,
		// chain
		"eth_blockNumber":                      a.ethBlockNumber,
		"eth_getBlockByNumber":                 a.ethGetBlockByNumber,
		"eth_getBlockByHash":                   a.ethGetBlockByHash,
		"eth_getBlockTransactionCountByNumber": a.ethGetBlockTransactionCountByNumber,
		// state
		"eth_getBalance":          a.ethGetBalance,
		"eth_getTransactionCount": a.ethGetTransactionCount,
		"eth_getCode":             a.ethGetCode,
		"eth_getStorageAt":        a.ethGetStorageAt,
		// txs
		"eth_sendRawTransaction":    a.ethSendRawTransaction,
		"eth_call":                  a.ethCall,
		"eth_estimateGas":           a.ethEstimateGas,
		"eth_getTransactionByHash":  a.ethGetTransactionByHash,
		"eth_getTransactionReceipt": a.ethGetTransactionReceipt,
		// fees
		"eth_gasPrice":             a.ethGasPrice,
		"eth_maxPriorityFeePerGas": a.ethMaxPriorityFeePerGas,
		"eth_feeHistory":           a.ethFeeHistory,
		// logs
		"eth_getLogs": a.ethGetLogs,
		// misc
		"eth_accounts":                   a.ethAccounts,
		"eth_syncing":                    a.ethSyncing,
		"eth_mining":                     a.ethMining,
		"web3_sha3":                      a.web3Sha3,
		"eth_getUncleCountByBlockNumber": a.ethUncleZero,
		"eth_getUncleCountByBlockHash":   a.ethUncleZero,
		// Phase B dew_* extensions
		"dew_sendRawTransaction": a.dewSendRawTransaction,
		"dew_getExecutionStats":  a.dewGetExecutionStats,
	}
}

func (a *API) web3ClientVersion(_ json.RawMessage) (interface{}, error) {
	return version.ClientVersion(), nil
}

func (a *API) netVersion(_ json.RawMessage) (interface{}, error) {
	return a.n.ChainID().String(), nil
}

func (a *API) netListening(_ json.RawMessage) (interface{}, error) { return true, nil }

func (a *API) netPeerCount(_ json.RawMessage) (interface{}, error) { return "0x0", nil }

func (a *API) ethChainId(_ json.RawMessage) (interface{}, error) {
	return EncodeBig(a.n.ChainID()), nil
}

func (a *API) ethBlockNumber(_ json.RawMessage) (interface{}, error) {
	return EncodeUint64(a.n.BlockNumber()), nil
}

func (a *API) ethGasPrice(_ json.RawMessage) (interface{}, error) {
	return EncodeBig(a.n.GasPrice()), nil
}

func (a *API) ethMaxPriorityFeePerGas(_ json.RawMessage) (interface{}, error) {
	return "0x1", nil // 1 wei tip suggestion
}

func (a *API) ethAccounts(_ json.RawMessage) (interface{}, error) {
	return []string{}, nil
}

func (a *API) ethSyncing(_ json.RawMessage) (interface{}, error) { return false, nil }

func (a *API) ethMining(_ json.RawMessage) (interface{}, error) { return true, nil }

func (a *API) ethUncleZero(_ json.RawMessage) (interface{}, error) { return "0x0", nil }

func (a *API) web3Sha3(params json.RawMessage) (interface{}, error) {
	var p []string
	if err := json.Unmarshal(params, &p); err != nil || len(p) < 1 {
		return nil, fmt.Errorf("invalid params")
	}
	b, err := DecodeBytes(p[0])
	if err != nil {
		return nil, err
	}
	return EncodeBytes(crypto.Keccak256(b)), nil
}

func (a *API) ethGetBalance(params json.RawMessage) (interface{}, error) {
	var p []interface{}
	if err := json.Unmarshal(params, &p); err != nil || len(p) < 1 {
		return nil, fmt.Errorf("invalid params")
	}
	addr, err := DecodeAddress(fmt.Sprint(p[0]))
	if err != nil {
		return nil, err
	}
	// block tag ignored → latest for Phase A4
	bal := a.n.GetBalance(addr)
	return EncodeBig(bal.ToBig()), nil
}

func (a *API) ethGetTransactionCount(params json.RawMessage) (interface{}, error) {
	var p []interface{}
	if err := json.Unmarshal(params, &p); err != nil || len(p) < 1 {
		return nil, fmt.Errorf("invalid params")
	}
	addr, err := DecodeAddress(fmt.Sprint(p[0]))
	if err != nil {
		return nil, err
	}
	return EncodeUint64(a.n.GetNonce(addr)), nil
}

func (a *API) ethGetCode(params json.RawMessage) (interface{}, error) {
	var p []interface{}
	if err := json.Unmarshal(params, &p); err != nil || len(p) < 1 {
		return nil, fmt.Errorf("invalid params")
	}
	addr, err := DecodeAddress(fmt.Sprint(p[0]))
	if err != nil {
		return nil, err
	}
	return EncodeBytes(a.n.GetCode(addr)), nil
}

func (a *API) ethGetStorageAt(params json.RawMessage) (interface{}, error) {
	var p []interface{}
	if err := json.Unmarshal(params, &p); err != nil || len(p) < 2 {
		return nil, fmt.Errorf("invalid params")
	}
	addr, err := DecodeAddress(fmt.Sprint(p[0]))
	if err != nil {
		return nil, err
	}
	slot, err := DecodeHash(fmt.Sprint(p[1]))
	if err != nil {
		return nil, err
	}
	return EncodeHash(a.n.GetStorageAt(addr, slot)), nil
}

func (a *API) ethSendRawTransaction(params json.RawMessage) (interface{}, error) {
	var p []string
	if err := json.Unmarshal(params, &p); err != nil || len(p) < 1 {
		return nil, fmt.Errorf("invalid params")
	}
	raw, err := DecodeBytes(p[0])
	if err != nil {
		return nil, err
	}
	hash, err := a.n.SendRawTransaction(raw)
	if err != nil {
		return nil, err
	}
	return EncodeHash(hash), nil
}

func (a *API) ethCall(params json.RawMessage) (interface{}, error) {
	msg, err := parseCallMsg(params)
	if err != nil {
		return nil, err
	}
	out, err := a.n.Call(msg)
	if err != nil {
		// eth_call often returns revert data as error; return as RPC error
		return nil, err
	}
	return EncodeBytes(out), nil
}

func (a *API) ethEstimateGas(params json.RawMessage) (interface{}, error) {
	msg, err := parseCallMsg(params)
	if err != nil {
		return nil, err
	}
	g, err := a.n.EstimateGas(msg)
	if err != nil {
		return nil, err
	}
	return EncodeUint64(g), nil
}

func parseCallMsg(params json.RawMessage) (vm.Message, error) {
	var p []json.RawMessage
	if err := json.Unmarshal(params, &p); err != nil || len(p) < 1 {
		return vm.Message{}, fmt.Errorf("invalid params")
	}
	var arg map[string]interface{}
	if err := json.Unmarshal(p[0], &arg); err != nil {
		return vm.Message{}, err
	}
	msg := vm.Message{
		GasLimit: 30_000_000,
		GasPrice: big.NewInt(0),
		Value:    uint256.NewInt(0),
	}
	if v, ok := arg["from"].(string); ok && v != "" {
		addr, err := DecodeAddress(v)
		if err != nil {
			return msg, err
		}
		msg.From = addr
	}
	if v, ok := arg["to"].(string); ok && v != "" {
		addr, err := DecodeAddress(v)
		if err != nil {
			return msg, err
		}
		msg.To = &addr
	}
	if v, ok := arg["data"].(string); ok {
		b, err := DecodeBytes(v)
		if err != nil {
			return msg, err
		}
		msg.Data = b
	} else if v, ok := arg["input"].(string); ok {
		b, err := DecodeBytes(v)
		if err != nil {
			return msg, err
		}
		msg.Data = b
	}
	if v, ok := arg["value"].(string); ok && v != "" {
		n, err := DecodeBig(v)
		if err != nil {
			return msg, err
		}
		msg.Value = uint256.MustFromBig(n)
	}
	if v, ok := arg["gas"].(string); ok && v != "" {
		g, err := DecodeUint64(v)
		if err != nil {
			return msg, err
		}
		msg.GasLimit = g
	}
	if v, ok := arg["gasPrice"].(string); ok && v != "" {
		gp, err := DecodeBig(v)
		if err != nil {
			return msg, err
		}
		msg.GasPrice = gp
	}
	return msg, nil
}

func (a *API) dewSendRawTransaction(params json.RawMessage) (interface{}, error) {
	var p []string
	if err := json.Unmarshal(params, &p); err != nil || len(p) < 1 {
		return nil, fmt.Errorf("invalid params")
	}
	raw, err := DecodeBytes(p[0])
	if err != nil {
		return nil, err
	}
	hash, err := a.n.SendDewRawTransaction(raw)
	if err != nil {
		return nil, err
	}
	return EncodeHash(hash), nil
}

func (a *API) dewGetExecutionStats(_ json.RawMessage) (interface{}, error) {
	st := a.n.ExecutionStats()
	return map[string]interface{}{
		"current_tps":            st.CurrentTPS,
		"peak_tps":               st.PeakTPS,
		"active_workers":         st.ActiveWorkers,
		"state_db_read_lat_ns":   st.StateDBReadLatNS,
		"state_db_write_lat_ns":  st.StateDBWriteLatNS,
		"conflict_rollback_rate": st.ConflictRate,
		// extras (additive)
		"tx_count":        st.TxCount,
		"rollbacks":       st.Rollbacks,
		"speculative_ok":  st.SpeculativeOK,
		"total_txs":       st.TotalTxs,
		"total_rollbacks": st.TotalRollbacks,
	}, nil
}

func (a *API) ethGetTransactionByHash(params json.RawMessage) (interface{}, error) {
	var p []string
	if err := json.Unmarshal(params, &p); err != nil || len(p) < 1 {
		return nil, fmt.Errorf("invalid params")
	}
	hash, err := DecodeHash(p[0])
	if err != nil {
		return nil, err
	}
	look := a.n.GetTransaction(hash)
	if look == nil {
		return nil, nil
	}
	// Native DewTx path
	if look.DewTx != nil {
		dtx := look.DewTx
		amt := "0x0"
		if dtx.Amount != nil {
			amt = EncodeBig(dtx.Amount.ToBig())
		}
		return map[string]interface{}{
			"hash":             EncodeHash(hash),
			"blockHash":        EncodeHash(look.BlockHash),
			"blockNumber":      EncodeUint64(look.BlockNumber),
			"transactionIndex": EncodeUint64(uint64(look.Index)),
			"from":             EncodeAddress(look.From),
			"to":               EncodeAddress(dtx.Receiver),
			"nonce":            EncodeUint64(dtx.Nonce),
			"value":            amt,
			"type":             EncodeUint64(uint64(types.DewTxType)),
			"input":            EncodeBytes(dtx.Payload),
			"dew":              true,
			"fee":              EncodeUint64(dtx.Fee),
		}, nil
	}
	tx := look.Tx
	if tx == nil {
		return nil, nil
	}
	result := map[string]interface{}{
		"hash":             EncodeHash(hash),
		"blockHash":        EncodeHash(look.BlockHash),
		"blockNumber":      EncodeUint64(look.BlockNumber),
		"transactionIndex": EncodeUint64(uint64(look.Index)),
		"from":             EncodeAddress(look.From),
		"nonce":            EncodeUint64(tx.Nonce()),
		"gas":              EncodeUint64(tx.Gas()),
		"value":            EncodeBig(tx.Value()),
		"input":            EncodeBytes(tx.Data()),
		"type":             EncodeUint64(uint64(tx.Type())),
		"chainId":          EncodeBig(a.n.ChainID()),
	}
	if to := tx.To(); to != nil {
		result["to"] = to.Hex()
	} else {
		result["to"] = nil
	}
	if tx.Type() == 2 {
		result["maxFeePerGas"] = EncodeBig(tx.GasFeeCap())
		result["maxPriorityFeePerGas"] = EncodeBig(tx.GasTipCap())
		result["gasPrice"] = EncodeBig(tx.GasFeeCap())
	} else {
		result["gasPrice"] = EncodeBig(tx.GasPrice())
	}
	v, r, s := tx.RawSignatureValues()
	result["v"] = EncodeBig(v)
	result["r"] = EncodeBig(r)
	result["s"] = EncodeBig(s)
	return result, nil
}

func (a *API) ethGetTransactionReceipt(params json.RawMessage) (interface{}, error) {
	var p []string
	if err := json.Unmarshal(params, &p); err != nil || len(p) < 1 {
		return nil, fmt.Errorf("invalid params")
	}
	hash, err := DecodeHash(p[0])
	if err != nil {
		return nil, err
	}
	rcpt := a.n.GetReceipt(hash)
	if rcpt == nil {
		return nil, nil
	}
	look := a.n.GetTransaction(hash)
	logs := make([]map[string]interface{}, len(rcpt.Logs))
	for i, lg := range rcpt.Logs {
		topics := make([]string, len(lg.Topics))
		for j, t := range lg.Topics {
			topics[j] = EncodeHash(t)
		}
		logs[i] = map[string]interface{}{
			"address":          EncodeAddress(lg.Address),
			"topics":           topics,
			"data":             EncodeBytes(lg.Data),
			"blockNumber":      EncodeUint64(rcpt.BlockNumber),
			"transactionHash":  EncodeHash(rcpt.TxHash),
			"transactionIndex": EncodeUint64(uint64(rcpt.TransactionIndex)),
			"blockHash":        EncodeHash(rcpt.BlockHash),
			"logIndex":         EncodeUint64(uint64(i)),
			"removed":          false,
		}
	}
	out := map[string]interface{}{
		"transactionHash":   EncodeHash(rcpt.TxHash),
		"transactionIndex":  EncodeUint64(uint64(rcpt.TransactionIndex)),
		"blockHash":         EncodeHash(rcpt.BlockHash),
		"blockNumber":       EncodeUint64(rcpt.BlockNumber),
		"from":              EncodeAddress(look.From),
		"cumulativeGasUsed": EncodeUint64(rcpt.CumulativeGasUsed),
		"gasUsed":           EncodeUint64(rcpt.GasUsed),
		"effectiveGasPrice": EncodeBig(rcpt.EffectiveGasPrice),
		"status":            EncodeUint64(rcpt.Status),
		"logs":              logs,
		"logsBloom":         "0x" + strings.Repeat("0", 512),
		"type":              EncodeUint64(uint64(rcpt.Type)),
	}
	if look.Tx.To() != nil {
		out["to"] = look.Tx.To().Hex()
	} else {
		out["to"] = nil
	}
	if rcpt.ContractAddress != nil {
		out["contractAddress"] = EncodeAddress(*rcpt.ContractAddress)
	} else {
		out["contractAddress"] = nil
	}
	return out, nil
}

func (a *API) ethGetBlockByNumber(params json.RawMessage) (interface{}, error) {
	var p []interface{}
	if err := json.Unmarshal(params, &p); err != nil || len(p) < 1 {
		return nil, fmt.Errorf("invalid params")
	}
	tag, err := ParseBlockNumber(p[0])
	if err != nil {
		return nil, err
	}
	full := false
	if len(p) > 1 {
		if b, ok := p[1].(bool); ok {
			full = b
		}
	}
	num, err := ResolveBlockNumber(tag, a.n.BlockNumber())
	if err != nil {
		return nil, nil // eth returns null for missing
	}
	block := a.n.GetBlockByNumber(num)
	if block == nil {
		return nil, nil
	}
	return a.formatBlock(block, full), nil
}

func (a *API) ethGetBlockByHash(params json.RawMessage) (interface{}, error) {
	var p []interface{}
	if err := json.Unmarshal(params, &p); err != nil || len(p) < 1 {
		return nil, fmt.Errorf("invalid params")
	}
	hash, err := DecodeHash(fmt.Sprint(p[0]))
	if err != nil {
		return nil, err
	}
	full := false
	if len(p) > 1 {
		if b, ok := p[1].(bool); ok {
			full = b
		}
	}
	block := a.n.GetBlockByHash(hash)
	if block == nil {
		return nil, nil
	}
	return a.formatBlock(block, full), nil
}

func (a *API) ethGetBlockTransactionCountByNumber(params json.RawMessage) (interface{}, error) {
	var p []interface{}
	if err := json.Unmarshal(params, &p); err != nil || len(p) < 1 {
		return nil, fmt.Errorf("invalid params")
	}
	tag, err := ParseBlockNumber(p[0])
	if err != nil {
		return nil, err
	}
	num, err := ResolveBlockNumber(tag, a.n.BlockNumber())
	if err != nil {
		return "0x0", nil
	}
	// count txs via receipt index scan
	count := uint64(0)
	// simple: check if any tx at this block
	// we don't keep per-block tx list separately; use header gasUsed heuristic + txIndex
	// count from txIndex
	// expose via node — for now iterate isn't available; return 0 for genesis, 1 typical
	_ = num
	if num == 0 {
		return "0x0", nil
	}
	// fallback: if block exists and not genesis, assume at least check receipts
	// Count by scanning is expensive; store is small
	block := a.n.GetBlockByNumber(num)
	if block == nil {
		return nil, nil
	}
	// Dev mode: blocks after genesis each have 1 tx when mined via sendRaw
	// Use gasUsed != 0 as signal
	if block.Header().GasUsed > 0 {
		count = 1
	}
	return EncodeUint64(count), nil
}

func (a *API) formatBlock(block *types.Block, fullTx bool) map[string]interface{} {
	h := block.Header()
	looks := a.n.TransactionsInBlock(h.Number)
	var txs interface{}
	if fullTx {
		list := make([]interface{}, 0, len(looks))
		for _, look := range looks {
			// reuse get-by-hash shape
			raw, _ := json.Marshal([]string{EncodeHash(look.TxHash)})
			item, err := a.ethGetTransactionByHash(raw)
			if err != nil || item == nil {
				list = append(list, EncodeHash(look.TxHash))
				continue
			}
			list = append(list, item)
		}
		txs = list
	} else {
		hashes := make([]string, 0, len(looks))
		for _, look := range looks {
			hashes = append(hashes, EncodeHash(look.TxHash))
		}
		txs = hashes
	}
	out := map[string]interface{}{
		"number":           EncodeUint64(h.Number),
		"hash":             EncodeHash(block.Hash()),
		"parentHash":       EncodeHash(h.ParentHash),
		"nonce":            "0x0000000000000000",
		"sha3Uncles":       "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
		"logsBloom":        "0x" + strings.Repeat("0", 512),
		"transactionsRoot": EncodeHash(h.TxRoot),
		"stateRoot":        EncodeHash(h.StateRoot),
		"receiptsRoot":     EncodeHash(h.ReceiptRoot),
		"miner":            EncodeAddress(h.Proposer),
		"difficulty":       "0x0",
		"totalDifficulty":  "0x0",
		"extraData":        EncodeBytes(h.ExtraData),
		"size":             "0x0",
		"gasLimit":         EncodeUint64(h.GasLimit),
		"gasUsed":          EncodeUint64(h.GasUsed),
		"timestamp":        EncodeUint64(h.Timestamp),
		"uncles":           []string{},
		"baseFeePerGas":    EncodeBig(h.BaseFee),
		"transactions":     txs,
		"mixHash":          "0x" + strings.Repeat("0", 64),
	}
	return out
}

func (a *API) ethFeeHistory(params json.RawMessage) (interface{}, error) {
	// Minimal feeHistory for MetaMask EIP-1559
	var p []interface{}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params")
	}
	blockCount := uint64(1)
	if len(p) > 0 {
		switch v := p[0].(type) {
		case float64:
			blockCount = uint64(v)
		case string:
			if n, err := DecodeUint64(v); err == nil {
				blockCount = n
			}
		}
	}
	if blockCount == 0 {
		blockCount = 1
	}
	if blockCount > 1024 {
		blockCount = 1024
	}
	head := a.n.BlockNumber()
	baseFee := a.n.BaseFee()
	oldest := uint64(0)
	if head+1 > blockCount {
		oldest = head + 1 - blockCount
	}
	baseFees := make([]string, 0, blockCount+1)
	gasUsedRatio := make([]float64, 0, blockCount)
	rewards := make([][]string, 0, blockCount)
	for i := uint64(0); i < blockCount; i++ {
		baseFees = append(baseFees, EncodeBig(baseFee))
		gasUsedRatio = append(gasUsedRatio, 0.5)
		rewards = append(rewards, []string{"0x1", "0x1", "0x1"})
	}
	// next base fee
	baseFees = append(baseFees, EncodeBig(baseFee))
	return map[string]interface{}{
		"oldestBlock":   EncodeUint64(oldest),
		"baseFeePerGas": baseFees,
		"gasUsedRatio":  gasUsedRatio,
		"reward":        rewards,
	}, nil
}

func (a *API) ethGetLogs(params json.RawMessage) (interface{}, error) {
	var p []map[string]interface{}
	if err := json.Unmarshal(params, &p); err != nil || len(p) < 1 {
		return nil, fmt.Errorf("invalid params")
	}
	q := p[0]
	head := a.n.BlockNumber()
	from, to := uint64(0), head
	if v, ok := q["fromBlock"]; ok {
		tag, err := ParseBlockNumber(v)
		if err != nil {
			return nil, err
		}
		from, err = ResolveBlockNumber(tag, head)
		if err != nil {
			return nil, err
		}
	}
	if v, ok := q["toBlock"]; ok {
		tag, err := ParseBlockNumber(v)
		if err != nil {
			return nil, err
		}
		to, err = ResolveBlockNumber(tag, head)
		if err != nil {
			return nil, err
		}
	}
	var addrs []crypto.Address
	if v, ok := q["address"]; ok {
		switch t := v.(type) {
		case string:
			a, err := DecodeAddress(t)
			if err != nil {
				return nil, err
			}
			addrs = append(addrs, a)
		case []interface{}:
			for _, x := range t {
				a, err := DecodeAddress(fmt.Sprint(x))
				if err != nil {
					return nil, err
				}
				addrs = append(addrs, a)
			}
		}
	}
	var topics [][]types.Hash
	if v, ok := q["topics"].([]interface{}); ok {
		for _, level := range v {
			if level == nil {
				topics = append(topics, nil)
				continue
			}
			switch t := level.(type) {
			case string:
				h, err := DecodeHash(t)
				if err != nil {
					return nil, err
				}
				topics = append(topics, []types.Hash{h})
			case []interface{}:
				var alts []types.Hash
				for _, x := range t {
					if x == nil {
						continue
					}
					h, err := DecodeHash(fmt.Sprint(x))
					if err != nil {
						return nil, err
					}
					alts = append(alts, h)
				}
				topics = append(topics, alts)
			}
		}
	}
	matched := a.n.FilterLogs(from, to, addrs, topics)
	out := make([]map[string]interface{}, 0, len(matched))
	for _, il := range matched {
		ts := make([]string, len(il.Log.Topics))
		for i, t := range il.Log.Topics {
			ts[i] = EncodeHash(t)
		}
		out = append(out, map[string]interface{}{
			"address":          EncodeAddress(il.Log.Address),
			"topics":           ts,
			"data":             EncodeBytes(il.Log.Data),
			"blockNumber":      EncodeUint64(il.BlockNumber),
			"transactionHash":  EncodeHash(il.TxHash),
			"transactionIndex": EncodeUint64(uint64(il.TxIndex)),
			"blockHash":        EncodeHash(il.BlockHash),
			"logIndex":         EncodeUint64(uint64(il.Index)),
			"removed":          false,
		})
	}
	return out, nil
}
