package params

// Public testnet freeze (Phase C6).
//
// After this freeze, prefer genesis/config parameter changes over wire-format
// or address churn. Mainnet still requires an external audit of consensus +
// VM bridge + crypto (see agents/debt.md).

const (
	// PublicTestnetFreezeTag labels the freeze surface for docs and release notes.
	PublicTestnetFreezeTag = "public-testnet-v1"

	// PublicTestnetChainID is the chain ID for the first public testnet.
	// Matches sample genesis.json and eth_chainId 0x89d.
	// 2026 was avoided: already assigned to Edgeless Network (symbol EwEth) on chainlist.
	PublicTestnetChainID uint64 = 2205

	// PublicTestnetBaseFeeWei is the genesis base fee (1 gwei).
	PublicTestnetBaseFeeWei uint64 = ReferenceBaseFeeWei

	// PublicTestnetBlockGasLimit matches DefaultBlockGasLimit / genesis gasLimit.
	PublicTestnetBlockGasLimit uint64 = DefaultBlockGasLimit

	// Mempool floors frozen with C1 defaults (see mempool.DefaultConfig).
	PublicTestnetMinGasPriceWei = ReferenceBaseFeeWei // 1 gwei
	PublicTestnetMinTipWei      = 1                   // 1 wei
	PublicTestnetMaxTxBytes     = 128 << 10           // 128 KiB
	PublicTestnetMempoolGlobal  = 4096
	PublicTestnetMempoolSender  = 16

	// RPC abuse limits (see rpc package).
	PublicTestnetMaxRPCBodyBytes = 1 << 20 // 1 MiB
	PublicTestnetMaxRPCBatch     = 100

	// Precompile addresses (20-byte low slots in EVM space).
	// Full registry: docs/protocol/addresses.md + core/vm.DewPrecompileSlots (S6).
	// 0x100 = native transfer (active);
	// 0x101 = orderbook (flagged when precompiles on; methods need EnableNativeSwap);
	// 0x102 = staking entrypoint (flagged; methods default off).
	PrecompileNativeTransferAddr     = 0x100
	PrecompileNativeSwapReservedAddr = 0x101
	PrecompileStakingAddr            = 0x102
	// PrecompileNextFreeAddr is the next unallocated Dew slot (not yet assigned).
	// Activating any new live address under public-testnet-v1 requires a hardfork doc.
	PrecompileNextFreeAddr = 0x103
)

// Feature flags expected on public-testnet-v1 operators (documented defaults).
const (
	// PublicTestnetNativePathOn — dew_sendRawTransaction enabled.
	PublicTestnetNativePathOn = DefaultEnableNativePath
	// PublicTestnetPrecompilesOn — 0x100+ registered when executor flag on.
	PublicTestnetPrecompilesOn = DefaultEnableDewPrecompiles
	// PublicTestnetStakingOn — staking methods remain opt-in (default off).
	PublicTestnetStakingOn = DefaultEnableStaking
	// PublicTestnetNativeSwapOn — orderbook methods remain opt-in (default off).
	PublicTestnetNativeSwapOn = DefaultEnableNativeSwap
)
