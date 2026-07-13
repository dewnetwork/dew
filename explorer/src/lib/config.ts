/** Build-time / public env for the explorer SPA. */

import { isHexAddress, normalizeHex } from "./format";

const rawRpc = import.meta.env.PUBLIC_RPC_URL as string | undefined;
const rawChain = import.meta.env.PUBLIC_CHAIN_ID as string | undefined;
const rawBase = import.meta.env.PUBLIC_EXPLORER_BASE as string | undefined;
const rawKnown = import.meta.env.PUBLIC_KNOWN_TOKENS as string | undefined;

function optNum(v: string | undefined): number | null {
  if (v == null || v.trim() === "") return null;
  const n = Number(v);
  return Number.isFinite(n) ? n : null;
}

/**
 * Parse known ERC-20 list for address balance tab.
 * Format (comma-separated): `0xaddr` or `SYMBOL:0xaddr`
 * Example: `0xabc…,WETH:0xdef…`
 */
export function parseKnownTokens(
  raw: string | undefined,
): { address: string; label: string | null }[] {
  if (!raw?.trim()) return [];
  const out: { address: string; label: string | null }[] = [];
  const seen = new Set<string>();
  for (const part of raw.split(/[,;\s]+/)) {
    const p = part.trim();
    if (!p) continue;
    let label: string | null = null;
    let addr = p;
    const colon = p.indexOf(":");
    if (colon > 0) {
      label = p.slice(0, colon).trim() || null;
      addr = p.slice(colon + 1).trim();
    }
    if (!isHexAddress(addr)) continue;
    const address = normalizeHex(addr);
    if (seen.has(address)) continue;
    seen.add(address);
    out.push({ address, label });
  }
  return out;
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
  /** Operator-configured tokens for balanceOf on address pages (P1d). */
  knownTokens: parseKnownTokens(rawKnown),
} as const;

export type AppConfig = typeof config;
