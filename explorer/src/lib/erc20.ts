/**
 * Best-effort ERC-20 metadata via eth_call (no indexer).
 * Failures are soft — non-token contracts simply return null.
 */

import { decodeAbiString, decodeAbiUint } from "./abi";
import { formatDew } from "./format";
import { rpcCall } from "./rpc";

const SEL = {
  name: "0x06fdde03",
  symbol: "0x95d89b41",
  decimals: "0x313ce567",
  totalSupply: "0x18160ddd",
  /** balanceOf(address) — pad address to 32 bytes after selector */
  balanceOf: "0x70a08231",
} as const;

/** Encode balanceOf(owner) calldata. */
export function balanceOfCalldata(owner: string): string {
  const addr = owner.toLowerCase().replace(/^0x/, "").padStart(64, "0");
  return `${SEL.balanceOf}${addr}`;
}

export type Erc20Meta = {
  name: string | null;
  symbol: string | null;
  decimals: number | null;
  totalSupply: bigint | null;
};

async function ethCall(to: string, data: string): Promise<string | null> {
  try {
    const res = await rpcCall<string>("eth_call", [
      { to: to.toLowerCase(), data },
      "latest",
    ]);
    if (!res || res === "0x") return null;
    return res;
  } catch {
    return null;
  }
}

export async function fetchErc20Meta(address: string): Promise<Erc20Meta | null> {
  const [nameRaw, symbolRaw, decRaw, supplyRaw] = await Promise.all([
    ethCall(address, SEL.name),
    ethCall(address, SEL.symbol),
    ethCall(address, SEL.decimals),
    ethCall(address, SEL.totalSupply),
  ]);

  const name = nameRaw ? decodeAbiString(nameRaw) : null;
  const symbol = symbolRaw ? decodeAbiString(symbolRaw) : null;
  let decimals: number | null = null;
  if (decRaw) {
    const d = decodeAbiUint(decRaw);
    if (d != null && d >= 0n && d <= 255n) decimals = Number(d);
  }
  const totalSupply = supplyRaw ? decodeAbiUint(supplyRaw) : null;

  // Require at least one successful token-ish field
  if (name == null && symbol == null && decimals == null && totalSupply == null) {
    return null;
  }
  // Reject if only totalSupply with no name/symbol/decimals (many contracts have storage)
  if (name == null && symbol == null && decimals == null) {
    return null;
  }

  return { name, symbol, decimals, totalSupply };
}

/** Format raw token amount with token decimals (default 18). */
export function formatTokenAmount(
  raw: bigint,
  decimals: number | null | undefined,
  maxFrac = 6,
): string {
  const d = decimals ?? 18;
  if (d < 0 || d > 36) return raw.toString();
  const base = 10n ** BigInt(d);
  const neg = raw < 0n;
  const abs = neg ? -raw : raw;
  const whole = abs / base;
  const frac = abs % base;
  if (frac === 0n) return `${neg ? "-" : ""}${whole.toString()}`;
  let fracStr = frac.toString().padStart(d, "0").slice(0, maxFrac);
  fracStr = fracStr.replace(/0+$/, "");
  if (!fracStr) return `${neg ? "-" : ""}${whole.toString()}`;
  return `${neg ? "-" : ""}${whole.toString()}.${fracStr}`;
}

/** Prefer token decimals; fall back to DEW-style 18 for display helpers. */
export function formatMaybeToken(
  raw: bigint,
  decimals: number | null | undefined,
  symbol?: string | null,
): string {
  const amt = decimals != null ? formatTokenAmount(raw, decimals) : formatDew(raw);
  return symbol ? `${amt} ${symbol}` : amt;
}

export type KnownTokenBalance = {
  address: string;
  label: string | null;
  symbol: string | null;
  decimals: number | null;
  balance: bigint;
  error?: string;
};

/**
 * balanceOf + soft metadata for a configured known-token list.
 * Does not hide zero balances (operator may want to see 0).
 */
export async function fetchKnownTokenBalances(
  owner: string,
  tokens: { address: string; label?: string | null }[],
): Promise<KnownTokenBalance[]> {
  const results = await Promise.all(
    tokens.map(async (t): Promise<KnownTokenBalance> => {
      const addr = t.address.toLowerCase();
      try {
        const balRaw = await ethCall(addr, balanceOfCalldata(owner));
        const balance = balRaw ? (decodeAbiUint(balRaw) ?? 0n) : 0n;
        const meta = await fetchErc20Meta(addr);
        return {
          address: addr,
          label: t.label ?? null,
          symbol: meta?.symbol ?? t.label ?? null,
          decimals: meta?.decimals ?? null,
          balance,
        };
      } catch (e) {
        return {
          address: addr,
          label: t.label ?? null,
          symbol: t.label ?? null,
          decimals: null,
          balance: 0n,
          error: e instanceof Error ? e.message : String(e),
        };
      }
    }),
  );
  return results;
}
