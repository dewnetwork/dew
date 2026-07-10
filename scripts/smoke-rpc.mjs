#!/usr/bin/env node
/**
 * Smoke-test Dew JSON-RPC (Phase A4).
 * Usage: node scripts/smoke-rpc.mjs [url]
 * Default url: http://127.0.0.1:8545
 */
const url = process.argv[2] || "http://127.0.0.1:8545";

async function rpc(method, params = []) {
  const res = await fetch(url, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ jsonrpc: "2.0", id: 1, method, params }),
  });
  const body = await res.json();
  if (body.error) {
    throw new Error(`${method}: ${body.error.message}`);
  }
  return body.result;
}

const chainId = await rpc("eth_chainId");
const block = await rpc("eth_blockNumber");
const version = await rpc("web3_clientVersion");
const net = await rpc("net_version");

console.log(JSON.stringify({ url, chainId, blockNumber: block, client: version, net_version: net }, null, 2));

if (chainId !== "0x7ea") {
  console.error("expected eth_chainId 0x7ea (2026)");
  process.exit(1);
}
console.log("smoke-rpc: ok");
