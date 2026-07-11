import { hexToNumber } from "./format";
import {
  ethBlockNumber,
  ethGasPrice,
  ethGetBlockByNumber,
  rpcBatch,
  type RpcBlock,
} from "./rpc";

const DAY_SEC = 86_400;
const HISTORY_DAYS = 14;
const SAMPLES_PER_DAY = 6;
const RECENT_WINDOW = 48;
const MAX_BATCH = 100;

export type DayTxPoint = {
  dayKey: string; // YYYY-MM-DD UTC
  label: string; // short label for axis
  timestamp: number; // day start UTC
  txs: number;
  samples: number;
  estimated: boolean;
};

export type NetworkStats = {
  head: number;
  finalized: number;
  safe: number;
  gasPriceWei: string;
  avgBlockTimeSec: number;
  tps: number;
  /** Tx count in the recent sample window */
  recentTxCount: number;
  recentBlocks: number;
  /** Sum of estimated daily txs over available history window */
  txs14d: number;
  history: DayTxPoint[];
  chainAgeDays: number;
  genesisTimestamp: number | null;
};

function dayKeyUtc(tsSec: number): string {
  const d = new Date(tsSec * 1000);
  return d.toISOString().slice(0, 10);
}

function dayLabel(tsSec: number): string {
  const d = new Date(tsSec * 1000);
  return d.toLocaleDateString("en-US", { month: "short", day: "numeric", timeZone: "UTC" });
}

function startOfUtcDay(tsSec: number): number {
  const d = new Date(tsSec * 1000);
  return Math.floor(Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate()) / 1000);
}

async function getBlocksByNumber(nums: number[], fullTxs: boolean): Promise<(RpcBlock | null)[]> {
  if (nums.length === 0) return [];
  const out: (RpcBlock | null)[] = [];
  for (let i = 0; i < nums.length; i += MAX_BATCH) {
    const chunk = nums.slice(i, i + MAX_BATCH);
    const results = await rpcBatch(
      chunk.map((n) => ({
        method: "eth_getBlockByNumber",
        params: [`0x${n.toString(16)}`, fullTxs],
      })),
    );
    for (const r of results) {
      out.push(r != null && typeof r === "object" ? (r as RpcBlock) : null);
    }
  }
  return out;
}

function txCount(b: RpcBlock | null): number {
  if (!b?.transactions) return 0;
  return b.transactions.length;
}

/**
 * Estimate block number for a past timestamp using average block time and head.
 * Clamped to [0, head].
 */
function estimateBlockAt(
  targetTs: number,
  head: number,
  headTs: number,
  avgBlockTimeSec: number,
): number {
  const dt = Math.max(0, headTs - targetTs);
  const delta = avgBlockTimeSec > 0 ? Math.floor(dt / avgBlockTimeSec) : 0;
  return Math.max(0, Math.min(head, head - delta));
}

/**
 * Fetch network overview stats + 14-day tx history estimated from RPC block samples.
 * Not a full indexer — short chains are more accurate; long chains are sampled.
 */
export async function fetchNetworkStats(): Promise<NetworkStats> {
  const [head, gasPriceWei] = await Promise.all([ethBlockNumber(), ethGasPrice()]);
  const headBlock = await ethGetBlockByNumber(head, false);
  if (!headBlock) {
    throw new Error("Could not load head block");
  }
  const headTs = hexToNumber(headBlock.timestamp);

  // Recent window for TPS + block time
  const recentStart = Math.max(0, head - RECENT_WINDOW + 1);
  const recentNums: number[] = [];
  for (let n = recentStart; n <= head; n++) recentNums.push(n);
  const recentBlocks = await getBlocksByNumber(recentNums, true);
  const validRecent = recentBlocks.filter((b): b is RpcBlock => b != null);

  let recentTxCount = 0;
  for (const b of validRecent) recentTxCount += txCount(b);

  let avgBlockTimeSec = 1;
  if (validRecent.length >= 2) {
    const first = hexToNumber(validRecent[0]!.timestamp);
    const last = hexToNumber(validRecent[validRecent.length - 1]!.timestamp);
    const span = Math.abs(last - first);
    const steps = validRecent.length - 1;
    if (span > 0 && steps > 0) avgBlockTimeSec = span / steps;
  }

  const windowSec =
    validRecent.length >= 2
      ? Math.abs(
          hexToNumber(validRecent[validRecent.length - 1]!.timestamp) -
            hexToNumber(validRecent[0]!.timestamp),
        )
      : avgBlockTimeSec * Math.max(1, validRecent.length);
  const tps = windowSec > 0 ? recentTxCount / windowSec : 0;

  // Genesis probe for chain age
  let genesisTimestamp: number | null = null;
  if (head >= 0) {
    const g = await ethGetBlockByNumber(0, false);
    if (g) genesisTimestamp = hexToNumber(g.timestamp);
  }
  const chainAgeDays =
    genesisTimestamp != null ? Math.max(0, (headTs - genesisTimestamp) / DAY_SEC) : 0;

  // Build 14 daily buckets ending today UTC
  const todayStart = startOfUtcDay(headTs);
  const history: DayTxPoint[] = [];
  let txs14d = 0;

  const sampleNums: number[] = [];
  const dayPlans: {
    dayStart: number;
    fromBlock: number;
    toBlock: number;
    sampleIdx: number[];
  }[] = [];

  for (let d = HISTORY_DAYS - 1; d >= 0; d--) {
    const dayStart = todayStart - d * DAY_SEC;
    const dayEnd = dayStart + DAY_SEC - 1;
    if (genesisTimestamp != null && dayEnd < genesisTimestamp) continue;
    if (dayStart > headTs) continue;

    const fromBlock = estimateBlockAt(
      Math.max(dayStart, genesisTimestamp ?? dayStart),
      head,
      headTs,
      avgBlockTimeSec,
    );
    const toBlock = estimateBlockAt(
      Math.min(dayEnd, headTs),
      head,
      headTs,
      avgBlockTimeSec,
    );
    const lo = Math.min(fromBlock, toBlock);
    const hi = Math.max(fromBlock, toBlock);
    const span = Math.max(0, hi - lo);
    const count = Math.min(SAMPLES_PER_DAY, span + 1);
    const idxs: number[] = [];
    for (let i = 0; i < count; i++) {
      const n = count === 1 ? lo : lo + Math.round((span * i) / (count - 1));
      idxs.push(n);
      sampleNums.push(n);
    }
    dayPlans.push({ dayStart, fromBlock: lo, toBlock: hi, sampleIdx: idxs });
  }

  // Deduplicate sample numbers while preserving fetch
  const unique = [...new Set(sampleNums)].sort((a, b) => a - b);
  const fetched = await getBlocksByNumber(unique, true);
  const byNum = new Map<number, RpcBlock | null>();
  unique.forEach((n, i) => byNum.set(n, fetched[i] ?? null));

  for (const plan of dayPlans) {
    const samples = plan.sampleIdx
      .map((n) => byNum.get(n) ?? null)
      .filter((b): b is RpcBlock => b != null);
    const nSamples = samples.length;
    let dayTxs = 0;
    let estimated = true;
    if (nSamples === 0) {
      dayTxs = 0;
    } else {
      const mean = samples.reduce((s, b) => s + txCount(b), 0) / nSamples;
      const blocksInRange = Math.max(1, plan.toBlock - plan.fromBlock + 1);
      // If range is tiny or fully sampled, use sum of unique samples only when span small
      if (blocksInRange <= nSamples) {
        dayTxs = Math.round(
          samples.reduce((s, b) => s + txCount(b), 0) * (blocksInRange / nSamples),
        );
        estimated = blocksInRange > nSamples;
      } else {
        dayTxs = Math.round(mean * blocksInRange);
        estimated = true;
      }
    }
    txs14d += dayTxs;
    history.push({
      dayKey: dayKeyUtc(plan.dayStart),
      label: dayLabel(plan.dayStart),
      timestamp: plan.dayStart,
      txs: dayTxs,
      samples: nSamples,
      estimated,
    });
  }

  // Dew-BFT: committed head ≈ final; safe = head for explorer MVP
  return {
    head,
    finalized: head,
    safe: head,
    gasPriceWei,
    avgBlockTimeSec,
    tps,
    recentTxCount,
    recentBlocks: validRecent.length,
    txs14d,
    history,
    chainAgeDays,
    genesisTimestamp,
  };
}

export { HISTORY_DAYS };
