/** Build-time / public env for the explorer SPA. */

const rawRpc = import.meta.env.PUBLIC_RPC_URL as string | undefined;
const rawChain = import.meta.env.PUBLIC_CHAIN_ID as string | undefined;
const rawBase = import.meta.env.PUBLIC_EXPLORER_BASE as string | undefined;

function optNum(v: string | undefined): number | null {
  if (v == null || v.trim() === "") return null;
  const n = Number(v);
  return Number.isFinite(n) ? n : null;
}

export const config = {
  rpcUrl: (rawRpc?.trim() || "http://127.0.0.1:8545").replace(/\/$/, ""),
  expectedChainId: Number.parseInt(rawChain || "2205", 10) || 2205,
  explorerBase: rawBase?.trim().replace(/\/$/, "") || "",
  freezeTag: "public-testnet-v1",
  networkName: "Dew",
  symbol: "DEW",
  /** Optional market display (no oracle in-protocol). */
  priceUsd: optNum(import.meta.env.PUBLIC_DEW_PRICE_USD as string | undefined),
  priceBtc: optNum(import.meta.env.PUBLIC_DEW_PRICE_BTC as string | undefined),
  priceChange24h: optNum(import.meta.env.PUBLIC_DEW_PRICE_CHANGE_24H as string | undefined),
  marketCapUsd: optNum(import.meta.env.PUBLIC_DEW_MARKET_CAP_USD as string | undefined),
  totalSupply: optNum(import.meta.env.PUBLIC_DEW_TOTAL_SUPPLY as string | undefined),
} as const;

export type AppConfig = typeof config;
