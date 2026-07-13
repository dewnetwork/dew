import { Contract, JsonRpcProvider, isAddress } from "ethers";
import { GUESTBOOK_ABI } from "./config";

export type GuestbookEntry = {
  id: number;
  author: string;
  timestamp: number;
  message: string;
};

export async function fetchChainId(rpcUrl: string): Promise<number> {
  const provider = new JsonRpcProvider(rpcUrl);
  const net = await provider.getNetwork();
  return Number(net.chainId);
}

export async function fetchEntries(
  rpcUrl: string,
  guestbook: string,
  chainId: number,
): Promise<{ total: number; entries: GuestbookEntry[] }> {
  if (!isAddress(guestbook)) {
    throw new Error("Invalid Guestbook address");
  }
  const provider = new JsonRpcProvider(rpcUrl, chainId);
  const book = new Contract(guestbook, GUESTBOOK_ABI, provider);
  const totalBn = await book.totalEntries();
  const total = Number(totalBn);
  if (!Number.isFinite(total) || total < 0) {
    throw new Error("Invalid totalEntries");
  }
  if (total === 0) {
    return { total: 0, entries: [] };
  }

  // Newest first; cap reads for very large books (demo-scale).
  const max = Math.min(total, 200);
  const start = total - max;
  const entries: GuestbookEntry[] = [];
  for (let id = total - 1; id >= start; id--) {
    const [author, ts, message] = await book.getEntry(id);
    entries.push({
      id,
      author,
      timestamp: Number(ts),
      message,
    });
  }
  return { total, entries };
}

/**
 * Sanitize a user-supplied explorer base URL.
 * Only http(s) is allowed; reconstructs from URL parts so DOM-sourced input
 * cannot become a javascript: (or other) href sink.
 */
export function sanitizeExplorerBase(base: string): string | null {
  const raw = base.trim();
  if (!raw) return null;
  let u: URL;
  try {
    u = new URL(raw);
  } catch {
    return null;
  }
  if (u.protocol !== "http:" && u.protocol !== "https:") {
    return null;
  }
  // Rebuild from protocol/host/pathname only (drop userinfo, hash, query).
  const path = u.pathname.replace(/\/$/, "");
  return `${u.protocol}//${u.host}${path === "/" ? "" : path}`;
}

const ETH_ADDR_RE = /^0x[0-9a-fA-F]{40}$/;
const TX_HASH_RE = /^0x[0-9a-fA-F]{64}$/;

/** Safe explorer address URL, or null if base/addr is unusable. */
export function explorerAddressUrl(base: string, addr: string): string | null {
  const b = sanitizeExplorerBase(base);
  if (!b) return null;
  const a = addr.trim();
  if (!ETH_ADDR_RE.test(a) && !isAddress(a)) return null;
  return `${b}/address/${encodeURIComponent(a)}`;
}

/** Safe explorer tx URL, or null if base/hash is unusable. */
export function explorerTxUrl(base: string, hash: string): string | null {
  const b = sanitizeExplorerBase(base);
  if (!b) return null;
  const h = hash.trim();
  if (!TX_HASH_RE.test(h)) return null;
  return `${b}/tx/${encodeURIComponent(h)}`;
}

export function shortAddr(addr: string): string {
  if (addr.length < 12) return addr;
  return `${addr.slice(0, 6)}…${addr.slice(-4)}`;
}

export function formatTs(sec: number): string {
  if (!sec) return "—";
  try {
    return new Date(sec * 1000).toLocaleString();
  } catch {
    return String(sec);
  }
}
