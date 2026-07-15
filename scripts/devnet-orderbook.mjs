#!/usr/bin/env node
/**
 * Lab-only: ERC-20 deploy + 0x101 place sell / fill against a Dew node with --native-swap.
 *
 * Usage:
 *   Terminal 1: ./bin/dew run --genesis genesis.json --native-swap --http.port 8545
 *   Terminal 2: node scripts/devnet-orderbook.mjs [rpcUrl]
 *
 * Default: http://127.0.0.1:8545
 * Uses Anvil #0 (maker) and #1 (taker) — must be pre-funded in genesis.
 * Do NOT point this at public Path B (flag off; methods revert).
 */
const url = process.argv[2] || "http://127.0.0.1:8545";

const PRIV0 =
  "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80";
const PRIV1 =
  "59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d";
const OB = "0x0000000000000000000000000000000000000101";

// Mock Token with approve/transferFrom (same as core/vm/token_bytecode.go)
const TOKEN_CREATION =
  "608060405234801561000f575f80fd5b5060405161050238038061050283398101604081905261002e91610075565b335f81815260208181526040808320859055518481527fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef910160405180910390a35061008c565b5f60208284031215610085575f80fd5b5051919050565b610469806100995f395ff3fe608060405234801561000f575f80fd5b5060043610610055575f3560e01c8063095ea7b31461005957806323b872dd1461008157806370a0823114610094578063a9059cbb146100c1578063dd62ed3e146100d4575b5f80fd5b61006c610067366004610347565b6100fe565b60405190151581526020015b60405180910390f35b61006c61008f36600461036f565b61016a565b6100b36100a23660046103a8565b5f6020819052908152604090205481565b604051908152602001610078565b61006c6100cf366004610347565b61021a565b6100b36100e23660046103c8565b600160209081525f928352604080842090915290825290205481565b335f8181526001602090815260408083206001600160a01b038716808552925280832085905551919290917f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925906101589086815260200190565b60405180910390a35060015b92915050565b6001600160a01b0383165f908152600160209081526040808320338452909152812054828110156101ce5760405162461bcd60e51b8152602060048201526009602482015268616c6c6f77616e636560b81b60448201526064015b60405180910390fd5b5f198114610204576101e0838261040d565b6001600160a01b0386165f9081526001602090815260408083203384529091529020555b61020f85858561022f565b506001949350505050565b5f61022633848461022f565b50600192915050565b6001600160a01b0383165f908152602081905260409020548111156102805760405162461bcd60e51b815260206004820152600760248201526662616c616e636560c81b60448201526064016101c5565b6001600160a01b0383165f90815260208190526040812080548392906102a790849061040d565b90915550506001600160a01b0382165f90815260208190526040812080548392906102d3908490610420565b92505081905550816001600160a01b0316836001600160a01b03167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef8360405161031f91815260200190565b60405180910390a3505050565b80356001600160a01b0381168114610342575f80fd5b919050565b5f8060408385031215610358575f80fd5b6103618361032c565b946020939093013593505050565b5f805f60608486031215610381575f80fd5b61038a8461032c565b92506103986020850161032c565b9150604084013590509250925092565b5f602082840312156103b8575f80fd5b6103c18261032c565b9392505050565b5f80604083850312156103d9575f80fd5b6103e28361032c565b91506103f06020840161032c565b90509250929050565b634e487b7160e01b5f52601160045260245ffd5b81810381811115610164576101646103f9565b80820180821115610164576101646103f956fea26469706673582212201a82d84c6e459af2672cc2df90d688e560f44f8fcffae48e6ac0d613f7e9e1a864736f6c63430008180033";

const TOKEN_ABI = [
  "constructor(uint256 supply)",
  "function balanceOf(address) view returns (uint256)",
  "function approve(address spender, uint256 amount) returns (bool)",
  "function transfer(address to, uint256 amount) returns (bool)",
  "function transferFrom(address from, address to, uint256 amount) returns (bool)",
];

function pad32(hexOrBig) {
  let h =
    typeof hexOrBig === "bigint" || typeof hexOrBig === "number"
      ? BigInt(hexOrBig).toString(16)
      : String(hexOrBig).replace(/^0x/i, "");
  return h.padStart(64, "0");
}

function packPlaceSell(tokenAddr, priceX18, baseAmount) {
  const token = tokenAddr.replace(/^0x/i, "").toLowerCase().padStart(40, "0");
  return (
    "0x" +
    "00" + // Place
    "01" + // sell
    token +
    pad32(priceX18) +
    pad32(baseAmount)
  );
}

function packFill(orderId, baseAmount) {
  return "0x" + "02" + pad32(orderId) + pad32(baseAmount);
}

function packBestAsk(tokenAddr) {
  const token = tokenAddr.replace(/^0x/i, "").toLowerCase().padStart(40, "0");
  return "0x" + "05" + token;
}

async function main() {
  let ethers;
  try {
    ethers = await import("ethers");
  } catch {
    console.error(
      "devnet-orderbook: install ethers at monorepo root, or use the Go lab test:\n" +
        "  go test ./node/ -run TestOrderbookLab_Scenario -v\n" +
        "  pnpm add -D ethers",
    );
    process.exit(1);
  }

  const { Wallet, JsonRpcProvider, Contract, ContractFactory, parseEther } =
    ethers;
  // staticNetwork avoids ethers network auto-detect races with auto-mine.
  const provider = new JsonRpcProvider(url, 2205, { staticNetwork: true });
  const maker = new Wallet(PRIV0, provider);
  const taker = new Wallet(PRIV1, provider);

  // Raw eth_getTransactionCount — ethers v6 can cache a stale count after Dew auto-mine.
  async function nextNonce(addr) {
    const hex = await provider.send("eth_getTransactionCount", [addr, "latest"]);
    return Number(BigInt(hex));
  }

  const chainId = await provider.send("eth_chainId", []);
  console.log({ url, chainId, maker: maker.address, taker: taker.address, ob: OB });

  const factory = new ContractFactory(TOKEN_ABI, "0x" + TOKEN_CREATION, maker);
  const supply = parseEther("1000000");
  const token = await factory.deploy(supply, {
    nonce: await nextNonce(maker.address),
    gasLimit: 3_000_000n,
  });
  await token.waitForDeployment();
  const deployTx = token.deploymentTransaction();
  if (deployTx) {
    const deployRcpt = await deployTx.wait();
    if (deployRcpt?.status !== 1) throw new Error("token deploy failed");
  }
  const tokenAddr = await token.getAddress();
  console.log("token:", tokenAddr);

  const tokenAsMaker = new Contract(tokenAddr, TOKEN_ABI, maker);
  const baseAmt = 10n;
  const price = parseEther("1"); // 1 DEW per 1e18 base → quote wei == base units for small sizes
  const approveTx = await tokenAsMaker.approve(OB, baseAmt, {
    nonce: await nextNonce(maker.address),
    gasLimit: 200_000n,
  });
  await approveTx.wait();
  console.log("approved 0x101 for", baseAmt.toString());

  const placeData = packPlaceSell(tokenAddr, price, baseAmt);
  const placeTx = await maker.sendTransaction({
    to: OB,
    data: placeData,
    gasLimit: 500_000n,
    nonce: await nextNonce(maker.address),
  });
  const placeRcpt = await placeTx.wait();
  if (placeRcpt.status !== 1) {
    throw new Error("place sell failed — is the node running with --native-swap?");
  }
  console.log("place sell tx:", placeTx.hash);

  const bestAsk = await provider.call({ to: OB, data: packBestAsk(tokenAddr) });
  const orderId = BigInt("0x" + bestAsk.slice(2 + 64, 2 + 128));
  console.log("orderId:", orderId.toString());
  if (orderId === 0n) throw new Error("bestAsk returned zero orderId");

  const fillBase = 4n;
  const fillData = packFill(orderId, fillBase);
  const fillTx = await taker.sendTransaction({
    to: OB,
    data: fillData,
    value: fillBase, // price 1e18 → quote = base for these units
    gasLimit: 500_000n,
    nonce: await nextNonce(taker.address),
  });
  const fillRcpt = await fillTx.wait();
  if (fillRcpt.status !== 1) throw new Error("fill failed");
  console.log("fill tx:", fillTx.hash);

  const bal = await tokenAsMaker.balanceOf(taker.address);
  console.log("taker base balance:", bal.toString());
  if (bal !== fillBase) {
    throw new Error(`unexpected taker balance ${bal} want ${fillBase}`);
  }
  console.log("devnet-orderbook: ok");
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
