/**
 * Public path B surfaces (public-testnet-v1).
 * Keep in sync with docs/ops/public-testnet.md and docs/ops/try-public.md.
 */
export const PUBLIC_NETWORK = {
  freezeTag: 'public-testnet-v1',
  chainId: 2205,
  /** EIP-155 chain id as 0x-prefixed hex (2205 → 0x89d). */
  chainIdHex: '0x89d',
  chainName: 'Dew public-testnet-v1',
  symbol: 'DEW',
  currencyName: 'DEW',
  decimals: 18,
  rpc: 'https://rpc-dew.fadosoft.com',
  explorer: 'https://explorer-dew.fadosoft.com',
  faucet: 'https://faucet-dew.fadosoft.com',
  guestbook: 'https://guestbook-dew.fadosoft.com',
  pathNote: 'Path B · single-host controlled RPC · not mainnet',
} as const;

/** EIP-3085 `wallet_addEthereumChain` params for the live path B network. */
export function ethereumChainParams() {
  return {
    chainId: PUBLIC_NETWORK.chainIdHex,
    chainName: PUBLIC_NETWORK.chainName,
    nativeCurrency: {
      name: PUBLIC_NETWORK.currencyName,
      symbol: PUBLIC_NETWORK.symbol,
      decimals: PUBLIC_NETWORK.decimals,
    },
    rpcUrls: [PUBLIC_NETWORK.rpc],
    blockExplorerUrls: [PUBLIC_NETWORK.explorer],
  } as const;
}
