/** Optional P1e history indexer HTTP client. Graceful no-op when URL unset. */

import { config } from "./config";
import type { DayTxPoint } from "./network-stats";

export type IndexerStatus = {
  rpcUrl: string;
  chainId?: string;
  tip: number;
  indexed: number;
  lag: number;
};

export type IndexerTx = {
  hash: string;
  blockNumber: number;
  transactionIndex: number;
  from: string;
  to?: string;
  value: string;
  status?: number;
  gasUsed?: number;
};

export type IndexerTransfer = {
  blockNumber: number;
  txHash: string;
  logIndex: number;
  token: string;
  from: string;
  to: string;
  amount: string;
};

export type IndexerVolumePoint = {
  dayKey?: string;
  blockFrom?: number;
  blockTo?: number;
  txCount: number;
  gasUsed: number;
  timestamp?: number;
};

function base(): string | null {
  const u = config.indexerUrl;
  return u || null;
}

export function indexerEnabled(): boolean {
  return Boolean(base());
}

async function getJSON<T>(path: string): Promise<T> {
  const b = base();
  if (!b) throw new Error("indexer not configured");
  const res = await fetch(`${b}${path}`);
  if (!res.ok) {
    throw new Error(`indexer HTTP ${res.status}`);
  }
  return (await res.json()) as T;
}

export async function fetchIndexerStatus(): Promise<IndexerStatus> {
  return getJSON<IndexerStatus>("/v1/status");
}

export async function fetchAddressTxs(
  addr: string,
  limit = 25,
  offset = 0,
): Promise<IndexerTx[]> {
  const q = new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
  });
  const body = await getJSON<{ txs: IndexerTx[] }>(
    `/v1/address/${encodeURIComponent(addr)}/txs?${q}`,
  );
  return body.txs ?? [];
}

export async function fetchAddressTransfers(
  addr: string,
  limit = 25,
  offset = 0,
): Promise<IndexerTransfer[]> {
  const q = new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
  });
  const body = await getJSON<{ transfers: IndexerTransfer[] }>(
    `/v1/address/${encodeURIComponent(addr)}/transfers?${q}`,
  );
  return body.transfers ?? [];
}

/** Map indexer daily volume → chart series. */
export async function fetchIndexerVolumeHistory(): Promise<DayTxPoint[]> {
  const body = await getJSON<{ points: IndexerVolumePoint[] }>(
    "/v1/stats/volume?from=0",
  );
  const points = body.points ?? [];
  return points.map((p) => {
    const ts = p.timestamp ?? 0;
    const dayKey = p.dayKey || (ts ? new Date(ts * 1000).toISOString().slice(0, 10) : "");
    const label = dayKey
      ? new Date(`${dayKey}T00:00:00Z`).toLocaleDateString("en-US", {
          month: "short",
          day: "numeric",
          timeZone: "UTC",
        })
      : "—";
    return {
      dayKey,
      label,
      timestamp: ts,
      txs: p.txCount,
      samples: 1,
      estimated: false,
    };
  });
}
