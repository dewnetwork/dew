/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly PUBLIC_RPC_URL?: string;
  readonly PUBLIC_CHAIN_ID?: string;
  readonly PUBLIC_EXPLORER_BASE?: string;
  readonly PUBLIC_DEW_PRICE_USD?: string;
  readonly PUBLIC_DEW_PRICE_BTC?: string;
  readonly PUBLIC_DEW_PRICE_CHANGE_24H?: string;
  readonly PUBLIC_DEW_MARKET_CAP_USD?: string;
  readonly PUBLIC_DEW_TOTAL_SUPPLY?: string;
  /** Comma-separated `0xaddr` or `SYMBOL:0xaddr` for address token balances */
  readonly PUBLIC_KNOWN_TOKENS?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
