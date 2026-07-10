#!/usr/bin/env node
/**
 * Deploy fixture ERC-20 Token and transfer over Dew JSON-RPC (Phase A7).
 *
 * Usage: node scripts/devnet-erc20.mjs [rpcUrl]
 * Default: http://127.0.0.1:8545
 *
 * Uses Anvil account #0 (faucet) — must be pre-funded in genesis.
 */
const url = process.argv[2] || "http://127.0.0.1:8545";

// Anvil #0
const PRIV =
  "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80";
const FROM = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266";
const TO = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8";

// solc Token creation bytecode (same as core/vm/token_bytecode.go)
const TOKEN_CREATION =
  "608060405234801561000f575f80fd5b5060405161027338038061027383398101604081905261002e91610075565b335f81815260208181526040808320859055518481527fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef910160405180910390a35061008c565b5f60208284031215610085575f80fd5b5051919050565b6101da806100995f395ff3fe608060405234801561000f575f80fd5b5060043610610034575f3560e01c806370a0823114610038578063a9059cbb1461006a575b5f80fd5b61005761004636600461015c565b5f6020819052908152604090205481565b6040519081526020015b60405180910390f35b61007d61007836600461017c565b61008d565b6040519015158152602001610061565b335f908152602081905260408120548211156100d95760405162461bcd60e51b815260206004820152600760248201526662616c616e636560c81b604482015260640160405180910390fd5b335f81815260208181526040808320805487900390556001600160a01b03871680845292819020805487019055518581529192917fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef910160405180910390a350600192915050565b80356001600160a01b0381168114610157575f80fd5b919050565b5f6020828403121561016c575f80fd5b61017582610141565b9392505050565b5f806040838503121561018d575f80fd5b61019683610141565b94602093909301359350505056fea264697066735822122098835b6600b46d3e44e1145030109088288d1c1aa9cdaf7e6c5b7505738177fc64736f6c63430008180033";

async function rpc(method, params = []) {
  const res = await fetch(url, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ jsonrpc: "2.0", id: 1, method, params }),
  });
  const body = await res.json();
  if (body.error) throw new Error(`${method}: ${body.error.message}`);
  return body.result;
}

async function main() {
  let ethers;
  try {
    ethers = await import("ethers");
  } catch {
    console.error(
      "devnet-erc20: install ethers in the monorepo root, or use the Go test:\n" +
        "  go test ./devnet/ -run ERC20 -v\n" +
        "  pnpm add -D ethers   # then re-run this script",
    );
    process.exit(1);
  }

  const { Wallet, JsonRpcProvider, ContractFactory, parseEther } = ethers;
  const provider = new JsonRpcProvider(url, 2026);
  const wallet = new Wallet(PRIV, provider);

  const chainId = await rpc("eth_chainId");
  console.log({ url, chainId, from: FROM });

  // Use ContractFactory with bytecode + minimal ABI
  const abi = [
    "constructor(uint256 supply)",
    "function balanceOf(address) view returns (uint256)",
    "function transfer(address to, uint256 amount) returns (bool)",
    "event Transfer(address indexed from, address indexed to, uint256 value)",
  ];
  const factory = new ContractFactory(abi, "0x" + TOKEN_CREATION, wallet);
  const supply = parseEther("1000000");
  const token = await factory.deploy(supply);
  await token.waitForDeployment();
  const tokenAddr = await token.getAddress();
  console.log("token:", tokenAddr);

  const tx = await token.transfer(TO, 1000n);
  await tx.wait();
  const bal = await token.balanceOf(TO);
  console.log("recipient balance:", bal.toString());
  if (bal !== 1000n) {
    throw new Error("unexpected balance");
  }
  console.log("devnet-erc20: ok");
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
