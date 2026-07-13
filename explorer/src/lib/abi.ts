/**
 * Minimal ABI helpers for explorer decode (no external ABI lib).
 * Selectors and event topics for common ERC-20 / ETH-style calls.
 */

import { hexToBigInt, isHexAddress, normalizeHex } from "./format";

/** 4-byte function selectors → short labels (lowercase hex with 0x). */
export const METHOD_SELECTORS: Record<string, string> = {
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
  "0xa0712d68": "mint(uint256)",
  "0x39509351": "increaseAllowance",
  "0xa457c2d7": "decreaseAllowance",
  "0x8da5cb5b": "owner",
  "0xf2fde38b": "transferOwnership",
  "0x715018a6": "renounceOwnership",
  "0x5c60da1b": "implementation",
  "0x3659cfe6": "upgradeTo",
  "0x4f1ef286": "upgradeToAndCall",
};

/** keccak256 topic0 for ERC-20 Transfer(address,address,uint256) */
export const TOPIC_TRANSFER =
  "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef";

/** keccak256 topic0 for ERC-20 Approval(address,address,uint256) */
export const TOPIC_APPROVAL =
  "0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925";

export function methodSelector(input: string | null | undefined): string | null {
  if (!input || input === "0x" || input.length < 10) return null;
  return normalizeHex(input).slice(0, 10);
}

export function methodName(input: string | null | undefined): string {
  const sel = methodSelector(input);
  if (!sel) return "Transfer";
  return METHOD_SELECTORS[sel] ?? sel;
}

/** Pad / unpad 32-byte word to address (last 20 bytes). */
export function wordToAddress(word: string): string | null {
  const h = normalizeHex(word).replace(/^0x/, "");
  if (h.length !== 64) return null;
  const addr = `0x${h.slice(24)}`;
  return isHexAddress(addr) ? addr : null;
}

export function wordToUint(word: string): bigint {
  return hexToBigInt(normalizeHex(word));
}

/** Decode ABI-encoded string from eth_call result (dynamic string). */
export function decodeAbiString(hexData: string): string | null {
  try {
    const h = normalizeHex(hexData).replace(/^0x/, "");
    if (h.length < 128) {
      // Some tokens return short-string in first 32 bytes (rare); try UTF-8 strip
      if (h.length === 0) return null;
      return null;
    }
    const offset = Number(BigInt("0x" + h.slice(0, 64)));
    if (!Number.isFinite(offset) || offset < 0) return null;
    const offHex = offset * 2;
    if (h.length < offHex + 64) return null;
    const len = Number(BigInt("0x" + h.slice(offHex, offHex + 64)));
    if (!Number.isFinite(len) || len < 0 || len > 10_000) return null;
    const start = offHex + 64;
    const end = start + len * 2;
    if (h.length < end) return null;
    const bytes = h.slice(start, end);
    const out: number[] = [];
    for (let i = 0; i < bytes.length; i += 2) {
      out.push(parseInt(bytes.slice(i, i + 2), 16));
    }
    const s = new TextDecoder("utf-8", { fatal: false }).decode(new Uint8Array(out));
    const cleaned = s.replace(/\0/g, "").trim();
    return cleaned || null;
  } catch {
    return null;
  }
}

export function decodeAbiUint(hexData: string): bigint | null {
  try {
    const h = normalizeHex(hexData);
    if (h === "0x" || h.length < 3) return null;
    return hexToBigInt(h);
  } catch {
    return null;
  }
}

export type TokenTransferRow = {
  kind: "transfer" | "approval";
  token: string;
  from: string;
  to: string;
  value: bigint;
  logIndex: number;
};

export function decodeTokenLog(log: {
  address: string;
  topics: string[];
  data: string;
  logIndex: string;
}): TokenTransferRow | null {
  if (!log.topics?.length) return null;
  const t0 = normalizeHex(log.topics[0] ?? "");
  const idx = Number(BigInt(log.logIndex || "0x0"));

  if (t0 === TOPIC_TRANSFER && log.topics.length >= 3) {
    const from = wordToAddress(log.topics[1]!);
    const to = wordToAddress(log.topics[2]!);
    if (!from || !to) return null;
    // value in data (single word) or topics[3] for some nonstandard
    let value = 0n;
    if (log.data && log.data !== "0x") {
      value = wordToUint(log.data.length >= 66 ? log.data.slice(0, 66) : log.data);
    } else if (log.topics[3]) {
      value = wordToUint(log.topics[3]);
    }
    return {
      kind: "transfer",
      token: log.address.toLowerCase(),
      from,
      to,
      value,
      logIndex: idx,
    };
  }

  if (t0 === TOPIC_APPROVAL && log.topics.length >= 3) {
    const from = wordToAddress(log.topics[1]!);
    const to = wordToAddress(log.topics[2]!);
    if (!from || !to) return null;
    let value = 0n;
    if (log.data && log.data !== "0x") {
      value = wordToUint(log.data.length >= 66 ? log.data.slice(0, 66) : log.data);
    }
    return {
      kind: "approval",
      token: log.address.toLowerCase(),
      from,
      to,
      value,
      logIndex: idx,
    };
  }

  return null;
}

/** transfer(address,uint256) calldata → recipient + amount */
export function decodeTransferCalldata(input: string): { to: string; value: bigint } | null {
  const h = normalizeHex(input).replace(/^0x/, "");
  if (h.length < 8 + 64 + 64) return null;
  if (`0x${h.slice(0, 8)}` !== "0xa9059cbb") return null;
  const to = wordToAddress("0x" + h.slice(8, 72));
  const value = wordToUint("0x" + h.slice(72, 136));
  if (!to) return null;
  return { to, value };
}
