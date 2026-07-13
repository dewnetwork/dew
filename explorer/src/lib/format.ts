/** Display helpers for addresses, hashes, wei, and time. */

const WEI_PER_DEW = 10n ** 18n;
const WEI_PER_GWEI = 10n ** 9n;

export function normalizeHex(value: string): string {
  const v = value.trim().toLowerCase();
  if (v.startsWith("0x")) return v;
  return `0x${v}`;
}

export function isHexAddress(value: string): boolean {
  return /^0x[0-9a-fA-F]{40}$/.test(value.trim());
}

export function isHexHash(value: string): boolean {
  return /^0x[0-9a-fA-F]{64}$/.test(value.trim());
}

export function isBlockNumber(value: string): boolean {
  const t = value.trim();
  if (!/^\d+$/.test(t)) return false;
  try {
    return BigInt(t) >= 0n;
  } catch {
    return false;
  }
}

export type SearchKind = "address" | "tx" | "blockNumber" | "blockHash" | "invalid";

export function classifySearch(raw: string): { kind: SearchKind; value: string } {
  const t = raw.trim();
  if (!t) return { kind: "invalid", value: t };
  if (isHexAddress(t)) return { kind: "address", value: normalizeHex(t) };
  if (isHexHash(t)) return { kind: "tx", value: normalizeHex(t) }; // hash may be block or tx; resolve at nav
  if (isBlockNumber(t)) return { kind: "blockNumber", value: t.replace(/^0+(?=\d)/, "") || "0" };
  return { kind: "invalid", value: t };
}

/** Truncate 0xabc…def (default 4…4). */
export function truncateHex(hex: string, left = 4, right = 4): string {
  const h = hex.startsWith("0x") ? hex : `0x${hex}`;
  if (h.length <= 2 + left + right) return h;
  return `${h.slice(0, 2 + left)}…${h.slice(-right)}`;
}

export function hexToBigInt(hex: string | null | undefined): bigint {
  if (hex == null || hex === "" || hex === "0x") return 0n;
  return BigInt(hex);
}

export function hexToNumber(hex: string | null | undefined): number {
  const n = hexToBigInt(hex);
  if (n > BigInt(Number.MAX_SAFE_INTEGER)) return Number.MAX_SAFE_INTEGER;
  return Number(n);
}

export function formatWei(wei: bigint | string): string {
  const w = typeof wei === "string" ? hexToBigInt(wei) : wei;
  return w.toString();
}

/** Human DEW with up to 18 decimals, trailing zeros trimmed. */
export function formatDew(wei: bigint | string, maxFrac = 6): string {
  const w = typeof wei === "string" ? hexToBigInt(wei) : wei;
  const neg = w < 0n;
  const abs = neg ? -w : w;
  const whole = abs / WEI_PER_DEW;
  const frac = abs % WEI_PER_DEW;
  if (frac === 0n) return `${neg ? "-" : ""}${whole.toString()}`;
  let fracStr = frac.toString().padStart(18, "0").slice(0, maxFrac);
  fracStr = fracStr.replace(/0+$/, "");
  if (!fracStr) return `${neg ? "-" : ""}${whole.toString()}`;
  return `${neg ? "-" : ""}${whole.toString()}.${fracStr}`;
}

export function formatGwei(wei: bigint | string, maxFrac = 4): string {
  const w = typeof wei === "string" ? hexToBigInt(wei) : wei;
  const whole = w / WEI_PER_GWEI;
  const frac = w % WEI_PER_GWEI;
  if (frac === 0n) return whole.toString();
  let fracStr = frac.toString().padStart(9, "0").slice(0, maxFrac).replace(/0+$/, "");
  return fracStr ? `${whole}.${fracStr}` : whole.toString();
}

export function formatNumber(n: number | bigint): string {
  return n.toLocaleString("en-US");
}

export function formatPercent(used: bigint, limit: bigint): number {
  if (limit === 0n) return 0;
  return Number((used * 10000n) / limit) / 100;
}

/** Prefer known ERC-20 / common selectors; else 4-byte hex. */
export function methodLabel(input: string | null | undefined): string {
  // Lazy import avoided — keep pure: re-export naming from abi via dynamic pattern
  if (!input || input === "0x" || input.length < 10) return "Transfer";
  const sel = input.slice(0, 10).toLowerCase();
  const known: Record<string, string> = {
    "0xa9059cbb": "transfer",
    "0x23b872dd": "transferFrom",
    "0x095ea7b3": "approve",
    "0x70a08231": "balanceOf",
    "0x18160ddd": "totalSupply",
    "0x313ce567": "decimals",
    "0x06fdde03": "name",
    "0x95d89b41": "symbol",
    "0xd0e30db0": "deposit",
    "0x2e1a7d4d": "withdraw",
    "0x40c10f19": "mint",
    "0x42966c68": "burn",
  };
  return known[sel] ?? sel;
}

export function relativeTime(tsSec: number, nowMs = Date.now()): string {
  const diff = Math.floor(nowMs / 1000) - tsSec;
  if (diff < 0) return "just now";
  if (diff < 60) return `${diff} sec${diff === 1 ? "" : "s"} ago`;
  if (diff < 3600) {
    const m = Math.floor(diff / 60);
    return `${m} min${m === 1 ? "" : "s"} ago`;
  }
  if (diff < 86400) {
    const h = Math.floor(diff / 3600);
    return `${h} hr${h === 1 ? "" : "s"} ago`;
  }
  const d = Math.floor(diff / 86400);
  return `${d} day${d === 1 ? "" : "s"} ago`;
}

export function absoluteUtc(tsSec: number): string {
  return new Date(tsSec * 1000).toISOString().replace("T", " ").replace(/\.\d{3}Z$/, " UTC");
}

export function shortError(err: unknown): string {
  if (err instanceof Error) return err.message;
  return String(err);
}
