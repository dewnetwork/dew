package params

const (
	// Orderbook gas (public-testnet-v1 activation budgets; freeze in hardfork doc).
	OrderbookGasPlace  uint64 = 50_000
	OrderbookGasCancel uint64 = 30_000
	OrderbookGasFill   uint64 = 80_000
	OrderbookGasQuery  uint64 = 3_000

	// OrderbookMaxOpenPerMaker is the max concurrent open orders per maker address.
	OrderbookMaxOpenPerMaker uint64 = 64

	// DefaultEnableNativeSwap is off; lab opts in with --native-swap / EnableNativeSwap.
	DefaultEnableNativeSwap = false
)
