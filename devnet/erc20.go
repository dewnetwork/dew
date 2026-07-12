package devnet

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/dewnetwork/dew/core/vm"
)

// MeshAdmit is invoked with signed raw tx bytes on each submission so BFT
// validators can include txs admitted only on the RPC full node.
type MeshAdmit func(raw []byte)

func admitMesh(mesh []MeshAdmit, raw []byte) {
	for _, m := range mesh {
		if m != nil {
			m(raw)
		}
	}
}

// DeployAndTransferERC20 deploys the fixture Token via signed txs on the given
// JSON-RPC URL and transfers `amount` to recipient. Returns contract address.
//
// Used by integration tests and can be called from tooling.
func DeployAndTransferERC20(rpcURL string, deployer Account, recipient common.Address, supply, amount *big.Int, mesh ...MeshAdmit) (common.Address, error) {
	client := &rpcClient{url: rpcURL}
	chainID, err := client.chainID()
	if err != nil {
		return common.Address{}, err
	}
	nonce, err := client.nonce(deployer.Address.Hex())
	if err != nil {
		return common.Address{}, err
	}

	parsed, err := abi.JSON(strings.NewReader(vm.TokenABI))
	if err != nil {
		return common.Address{}, err
	}
	bin, err := hex.DecodeString(vm.TokenCreationBytecode)
	if err != nil {
		return common.Address{}, err
	}
	ctor, err := parsed.Pack("", supply)
	if err != nil {
		return common.Address{}, err
	}
	deployData := append(append([]byte{}, bin...), ctor...)

	gasPrice := big.NewInt(1_000_000_000)
	deployTx := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    nonce,
		GasPrice: gasPrice,
		Gas:      3_000_000,
		To:       nil,
		Value:    big.NewInt(0),
		Data:     deployData,
	})
	signer := ethtypes.LatestSignerForChainID(chainID)
	signed, err := ethtypes.SignTx(deployTx, signer, deployer.PrivateKey)
	if err != nil {
		return common.Address{}, err
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		return common.Address{}, err
	}
	admitMesh(mesh, raw)
	txHash, err := client.sendRaw(raw)
	if err != nil {
		return common.Address{}, fmt.Errorf("deploy send: %w", err)
	}
	receipt, err := client.waitReceipt(txHash)
	if err != nil {
		return common.Address{}, err
	}
	if receipt.Status != 1 {
		return common.Address{}, fmt.Errorf("deploy failed status=%d", receipt.Status)
	}
	if receipt.ContractAddress == (common.Address{}) {
		return common.Address{}, fmt.Errorf("missing contract address")
	}
	token := receipt.ContractAddress

	// transfer(recipient, amount)
	calldata, err := parsed.Pack("transfer", recipient, amount)
	if err != nil {
		return common.Address{}, err
	}
	transferTx := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    nonce + 1,
		GasPrice: gasPrice,
		Gas:      100_000,
		To:       &token,
		Value:    big.NewInt(0),
		Data:     calldata,
	})
	signed2, err := ethtypes.SignTx(transferTx, signer, deployer.PrivateKey)
	if err != nil {
		return common.Address{}, err
	}
	raw2, err := signed2.MarshalBinary()
	if err != nil {
		return common.Address{}, err
	}
	admitMesh(mesh, raw2)
	txHash2, err := client.sendRaw(raw2)
	if err != nil {
		return common.Address{}, fmt.Errorf("transfer send: %w", err)
	}
	rc2, err := client.waitReceipt(txHash2)
	if err != nil {
		return common.Address{}, err
	}
	if rc2.Status != 1 {
		return common.Address{}, fmt.Errorf("transfer failed status=%d", rc2.Status)
	}

	// balanceOf(recipient) via eth_call
	balData, err := parsed.Pack("balanceOf", recipient)
	if err != nil {
		return common.Address{}, err
	}
	out, err := client.ethCall(token.Hex(), "0x"+hex.EncodeToString(balData))
	if err != nil {
		return common.Address{}, err
	}
	got := new(big.Int).SetBytes(common.FromHex(out))
	if got.Cmp(amount) != 0 {
		return common.Address{}, fmt.Errorf("recipient balance %s want %s", got, amount)
	}
	return token, nil
}

// eth crypto helper keep import when SignTx uses PrivateKey directly.
var _ = ethcrypto.PubkeyToAddress
