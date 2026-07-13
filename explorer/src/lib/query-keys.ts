import { config } from "./config";

export const qk = {
  chainId: (rpc = config.rpcUrl) => ["chainId", rpc] as const,
  head: (rpc = config.rpcUrl) => ["head", rpc] as const,
  gasPrice: (rpc = config.rpcUrl) => ["gasPrice", rpc] as const,
  client: (rpc = config.rpcUrl) => ["client", rpc] as const,
  block: (id: string, full: boolean, rpc = config.rpcUrl) =>
    ["block", rpc, id, full] as const,
  tx: (hash: string, rpc = config.rpcUrl) => ["tx", rpc, hash.toLowerCase()] as const,
  receipt: (hash: string, rpc = config.rpcUrl) =>
    ["receipt", rpc, hash.toLowerCase()] as const,
  address: (addr: string, rpc = config.rpcUrl) =>
    ["address", rpc, addr.toLowerCase()] as const,
  erc20: (addr: string, rpc = config.rpcUrl) =>
    ["erc20", rpc, addr.toLowerCase()] as const,
  knownTokenBalances: (owner: string, rpc = config.rpcUrl) =>
    ["knownTokenBalances", rpc, owner.toLowerCase(), config.knownTokens.map((t) => t.address).join(",")] as const,
  recent: (head: number, n: number, rpc = config.rpcUrl) =>
    ["recent", rpc, head, n] as const,
  networkStats: (rpc = config.rpcUrl) => ["networkStats", rpc] as const,
};
