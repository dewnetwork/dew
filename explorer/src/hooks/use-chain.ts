import { useQuery } from "@tanstack/react-query";
import { config } from "@/lib/config";
import { qk } from "@/lib/query-keys";
import {
  ethBlockNumber,
  ethChainId,
  ethClientVersion,
  ethGasPrice,
  ethGetBalance,
  ethGetBlockByHash,
  ethGetBlockByNumber,
  ethGetCode,
  ethGetTransactionByHash,
  ethGetTransactionCount,
  ethGetTransactionReceipt,
  rpcBatch,
  type RpcBlock,
  type RpcTx,
} from "@/lib/rpc";
import { hexToNumber, isHexHash } from "@/lib/format";
import { fetchNetworkStats } from "@/lib/network-stats";

const HEAD_MS = 4000;

export function useChainId() {
  return useQuery({
    queryKey: qk.chainId(),
    queryFn: () => ethChainId(),
    staleTime: 30_000,
    retry: 2,
  });
}

export function useHead(enabled = true) {
  return useQuery({
    queryKey: qk.head(),
    queryFn: () => ethBlockNumber(),
    refetchInterval: HEAD_MS,
    refetchIntervalInBackground: false,
    enabled,
  });
}

export function useGasPrice() {
  return useQuery({
    queryKey: qk.gasPrice(),
    queryFn: () => ethGasPrice(),
    refetchInterval: HEAD_MS,
    refetchIntervalInBackground: false,
  });
}

export function useClientVersion() {
  return useQuery({
    queryKey: qk.client(),
    queryFn: () => ethClientVersion(),
    staleTime: 60_000,
  });
}

export function useBlock(id: string, fullTxs = true) {
  return useQuery({
    queryKey: qk.block(id, fullTxs),
    queryFn: async () => {
      if (isHexHash(id)) return ethGetBlockByHash(id, fullTxs);
      return ethGetBlockByNumber(id, fullTxs);
    },
    enabled: Boolean(id),
  });
}

export function useTransaction(hash: string) {
  return useQuery({
    queryKey: qk.tx(hash),
    queryFn: () => ethGetTransactionByHash(hash),
    enabled: Boolean(hash),
  });
}

export function useReceipt(hash: string) {
  return useQuery({
    queryKey: qk.receipt(hash),
    queryFn: () => ethGetTransactionReceipt(hash),
    enabled: Boolean(hash),
    refetchInterval: (q) => (q.state.data == null && q.state.error == null ? HEAD_MS : false),
  });
}

export function useAddress(addr: string) {
  return useQuery({
    queryKey: qk.address(addr),
    queryFn: async () => {
      const [balance, nonce, code] = await Promise.all([
        ethGetBalance(addr),
        ethGetTransactionCount(addr),
        ethGetCode(addr),
      ]);
      return { balance, nonce, code };
    },
    enabled: Boolean(addr),
  });
}

export type RecentFeed = {
  blocks: RpcBlock[];
  txs: RpcTx[];
  txCountWindow: number;
};

export function useNetworkStats() {
  return useQuery({
    queryKey: qk.networkStats(),
    queryFn: () => fetchNetworkStats(),
    staleTime: 60_000,
    refetchInterval: 60_000,
    refetchIntervalInBackground: false,
  });
}

export function useRecentActivity(windowBlocks = 12) {
  const head = useHead();
  return useQuery({
    queryKey: qk.recent(head.data ?? -1, windowBlocks),
    enabled: head.data != null,
    queryFn: async (): Promise<RecentFeed> => {
      const h = head.data!;
      const start = Math.max(0, h - windowBlocks + 1);
      const nums: number[] = [];
      for (let n = h; n >= start; n--) nums.push(n);

      const results = await rpcBatch(
        nums.map((n) => ({
          method: "eth_getBlockByNumber",
          params: [`0x${n.toString(16)}`, true],
        })),
      );

      const blocks = results.filter((b): b is RpcBlock => b != null && typeof b === "object");
      const txs: RpcTx[] = [];
      let txCountWindow = 0;
      for (const b of blocks) {
        const list = b.transactions ?? [];
        txCountWindow += list.length;
        for (const t of list) {
          if (typeof t === "string") continue;
          txs.push(t);
          if (txs.length >= windowBlocks) break;
        }
        if (txs.length >= windowBlocks) break;
      }
      return { blocks, txs, txCountWindow };
    },
    refetchInterval: HEAD_MS,
    refetchIntervalInBackground: false,
  });
}

export function useChainOk() {
  const chain = useChainId();
  const expected = config.expectedChainId;
  const actual = chain.data;
  const mismatch = actual != null && actual !== expected;
  return {
    ...chain,
    expected,
    actual,
    mismatch,
    ok: actual === expected,
  };
}

export function blockTxCount(block: RpcBlock): number {
  return block.transactions?.length ?? 0;
}

export function blockNumberOf(block: RpcBlock): number {
  return hexToNumber(block.number ?? "0x0");
}
