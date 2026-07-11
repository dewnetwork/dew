package faucet

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// Chain is the subset of JSON-RPC used to drip native DEW.
type Chain interface {
	ChainID(ctx context.Context) (uint64, error)
	Nonce(ctx context.Context, addr string) (uint64, error)
	BalanceWei(ctx context.Context, addr string) (*big.Int, error)
	SendRaw(ctx context.Context, rawTx []byte) (string, error)
}

// Sender signs and submits fixed-amount native transfers.
type Sender struct {
	mu      sync.Mutex
	chain   Chain
	key     *ecdsa.PrivateKey
	from    common.Address
	chainID *big.Int
	amount  *big.Int
	gas     uint64
	gasPrice *big.Int
}

// NewSender builds a Sender from a hex private key and chain client.
func NewSender(chain Chain, privHex string, chainID uint64, amount, gasPrice *big.Int, gas uint64) (*Sender, error) {
	key, err := ethcrypto.HexToECDSA(normalizePrivHex(privHex))
	if err != nil {
		return nil, fmt.Errorf("faucet: private key: %w", err)
	}
	from := ethcrypto.PubkeyToAddress(key.PublicKey)
	return &Sender{
		chain:    chain,
		key:      key,
		from:     from,
		chainID:  new(big.Int).SetUint64(chainID),
		amount:   new(big.Int).Set(amount),
		gas:      gas,
		gasPrice: new(big.Int).Set(gasPrice),
	}, nil
}

// From returns the faucet EOA address.
func (s *Sender) From() common.Address { return s.from }

// Amount returns the drip size.
func (s *Sender) Amount() *big.Int { return new(big.Int).Set(s.amount) }

// DripResult is a successful transfer.
type DripResult struct {
	TxHash string
	From   string
	To     string
	Amount string // wei decimal string
}

// Drip sends AmountWei native DEW to recipient.
func (s *Sender) Drip(ctx context.Context, toHex string) (*DripResult, error) {
	toNorm, err := normalizeAddress(toHex)
	if err != nil {
		return nil, fmt.Errorf("invalid recipient: %w", err)
	}
	to := common.HexToAddress(toNorm)

	// Serialize drips so nonces stay monotonic under concurrency.
	s.mu.Lock()
	defer s.mu.Unlock()

	gotID, err := s.chain.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("chain id: %w", err)
	}
	if gotID != s.chainID.Uint64() {
		return nil, fmt.Errorf("wrong chain id: rpc=%d want=%d", gotID, s.chainID.Uint64())
	}

	bal, err := s.chain.BalanceWei(ctx, s.from.Hex())
	if err != nil {
		return nil, fmt.Errorf("faucet balance: %w", err)
	}
	cost := new(big.Int).Mul(s.gasPrice, new(big.Int).SetUint64(s.gas))
	need := new(big.Int).Add(s.amount, cost)
	if bal.Cmp(need) < 0 {
		return nil, fmt.Errorf("faucet underfunded: balance=%s need>=%s", bal, need)
	}

	nonce, err := s.chain.Nonce(ctx, s.from.Hex())
	if err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}

	tx := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    nonce,
		GasPrice: new(big.Int).Set(s.gasPrice),
		Gas:      s.gas,
		To:       &to,
		Value:    new(big.Int).Set(s.amount),
		Data:     nil,
	})
	signer := ethtypes.LatestSignerForChainID(s.chainID)
	signed, err := ethtypes.SignTx(tx, signer, s.key)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		return nil, err
	}
	hash, err := s.chain.SendRaw(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}
	return &DripResult{
		TxHash: hash,
		From:   s.from.Hex(),
		To:     to.Hex(),
		Amount: s.amount.String(),
	}, nil
}
