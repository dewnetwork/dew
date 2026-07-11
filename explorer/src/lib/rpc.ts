import { config } from "./config";
import { hexToNumber } from "./format";

export type JsonRpcId = number | string;

export class RpcError extends Error {
  readonly code: number;
  readonly data?: unknown;

  constructor(message: string, code: number, data?: unknown) {
    super(message);
    this.name = "RpcError";
    this.code = code;
    this.data = data;
  }
}

let nextId = 1;

export async function rpcCall<T>(
  method: string,
  params: unknown[] = [],
  rpcUrl: string = config.rpcUrl,
): Promise<T> {
  const id = nextId++;
  let res: Response;
  try {
    res = await fetch(rpcUrl, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ jsonrpc: "2.0", id, method, params }),
    });
  } catch (e) {
    throw new RpcError(
      `RPC unreachable (${rpcUrl}): ${e instanceof Error ? e.message : String(e)}`,
      -32000,
    );
  }

  if (!res.ok) {
    throw new RpcError(`HTTP ${res.status} from RPC`, res.status);
  }

  const body = (await res.json()) as {
    result?: T;
    error?: { code: number; message: string; data?: unknown };
  };

  if (body.error) {
    throw new RpcError(body.error.message || "RPC error", body.error.code, body.error.data);
  }
  return body.result as T;
}

/** Batch ≤100 (C6 limit). Returns results in order; throws if any transport error. */
export async function rpcBatch(
  calls: { method: string; params?: unknown[] }[],
  rpcUrl: string = config.rpcUrl,
): Promise<unknown[]> {
  if (calls.length === 0) return [];
  if (calls.length > 100) {
    throw new RpcError("RPC batch exceeds max 100 items", -32600);
  }
  const payload = calls.map((c, i) => ({
    jsonrpc: "2.0",
    id: i + 1,
    method: c.method,
    params: c.params ?? [],
  }));

  let res: Response;
  try {
    res = await fetch(rpcUrl, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
  } catch (e) {
    throw new RpcError(
      `RPC unreachable (${rpcUrl}): ${e instanceof Error ? e.message : String(e)}`,
      -32000,
    );
  }
  if (!res.ok) throw new RpcError(`HTTP ${res.status} from RPC`, res.status);

  const body = (await res.json()) as Array<{
    id: number;
    result?: unknown;
    error?: { code: number; message: string };
  }>;

  if (!Array.isArray(body)) {
    throw new RpcError("Expected JSON-RPC batch array response", -32603);
  }

  const byId = new Map(body.map((r) => [r.id, r]));
  return payload.map((p) => {
    const r = byId.get(p.id);
    if (!r) throw new RpcError(`Missing batch response id ${p.id}`, -32603);
    if (r.error) throw new RpcError(r.error.message, r.error.code);
    return r.result;
  });
}

export type RpcTx = {
  hash: string;
  blockHash?: string | null;
  blockNumber?: string | null;
  transactionIndex?: string | null;
  from: string;
  to: string | null;
  value: string;
  gas: string;
  gasPrice?: string;
  maxFeePerGas?: string;
  maxPriorityFeePerGas?: string;
  input: string;
  nonce: string;
  type?: string;
  chainId?: string;
};

export type RpcBlock = {
  number: string | null;
  hash: string | null;
  parentHash: string;
  timestamp: string;
  miner: string;
  gasLimit: string;
  gasUsed: string;
  baseFeePerGas?: string;
  extraData: string;
  stateRoot: string;
  receiptsRoot?: string;
  transactionsRoot?: string;
  transactions: (string | RpcTx)[];
  size?: string;
};

export type RpcLog = {
  address: string;
  topics: string[];
  data: string;
  logIndex: string;
  transactionHash?: string;
  blockNumber?: string;
};

export type RpcReceipt = {
  transactionHash: string;
  transactionIndex: string;
  blockHash: string;
  blockNumber: string;
  from: string;
  to: string | null;
  cumulativeGasUsed: string;
  gasUsed: string;
  contractAddress: string | null;
  logs: RpcLog[];
  status?: string;
  effectiveGasPrice?: string;
  type?: string;
};

export function toBlockTag(id: string | number | bigint): string {
  if (typeof id === "number" || typeof id === "bigint") {
    return `0x${BigInt(id).toString(16)}`;
  }
  const t = id.trim();
  if (/^\d+$/.test(t)) return `0x${BigInt(t).toString(16)}`;
  return t.toLowerCase();
}

export async function ethChainId(rpcUrl?: string): Promise<number> {
  const hex = await rpcCall<string>("eth_chainId", [], rpcUrl);
  return hexToNumber(hex);
}

export async function ethBlockNumber(rpcUrl?: string): Promise<number> {
  const hex = await rpcCall<string>("eth_blockNumber", [], rpcUrl);
  return hexToNumber(hex);
}

export async function ethGasPrice(rpcUrl?: string): Promise<string> {
  return rpcCall<string>("eth_gasPrice", [], rpcUrl);
}

export async function ethClientVersion(rpcUrl?: string): Promise<string> {
  return rpcCall<string>("web3_clientVersion", [], rpcUrl);
}

export async function ethGetBlockByNumber(
  n: number | string,
  fullTxs: boolean,
  rpcUrl?: string,
): Promise<RpcBlock | null> {
  return rpcCall<RpcBlock | null>("eth_getBlockByNumber", [toBlockTag(n), fullTxs], rpcUrl);
}

export async function ethGetBlockByHash(
  hash: string,
  fullTxs: boolean,
  rpcUrl?: string,
): Promise<RpcBlock | null> {
  return rpcCall<RpcBlock | null>("eth_getBlockByHash", [hash.toLowerCase(), fullTxs], rpcUrl);
}

export async function ethGetTransactionByHash(
  hash: string,
  rpcUrl?: string,
): Promise<RpcTx | null> {
  return rpcCall<RpcTx | null>("eth_getTransactionByHash", [hash.toLowerCase()], rpcUrl);
}

export async function ethGetTransactionReceipt(
  hash: string,
  rpcUrl?: string,
): Promise<RpcReceipt | null> {
  return rpcCall<RpcReceipt | null>("eth_getTransactionReceipt", [hash.toLowerCase()], rpcUrl);
}

export async function ethGetBalance(address: string, rpcUrl?: string): Promise<string> {
  return rpcCall<string>("eth_getBalance", [address.toLowerCase(), "latest"], rpcUrl);
}

export async function ethGetTransactionCount(address: string, rpcUrl?: string): Promise<string> {
  return rpcCall<string>("eth_getTransactionCount", [address.toLowerCase(), "latest"], rpcUrl);
}

export async function ethGetCode(address: string, rpcUrl?: string): Promise<string> {
  return rpcCall<string>("eth_getCode", [address.toLowerCase(), "latest"], rpcUrl);
}
