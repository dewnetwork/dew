const hre = require("hardhat");

/**
 * Deploy the full mock asset basket (USDT, USDC, DAI, WETH, WBTC).
 * Optional TRANSFER_TO receives a small demo amount of each.
 *
 * Usage:
 *   npx hardhat run scripts/deploy-mock-assets.js --network dewLocal
 *   TRANSFER_TO=0x… npx hardhat run scripts/deploy-mock-assets.js --network dewLocal
 *   PRIVATE_KEY=0x… npx hardhat run scripts/deploy-mock-assets.js --network dewPublic
 */
const BASKET = [
  {
    name: "MockUSDT",
    symbol: "USDT",
    decimals: 6,
    supply: 1_000_000n * 1_000_000n,
    demo: 1_000n * 1_000_000n,
  },
  {
    name: "MockUSDC",
    symbol: "USDC",
    decimals: 6,
    supply: 1_000_000n * 1_000_000n,
    demo: 1_000n * 1_000_000n,
  },
  {
    name: "MockDAI",
    symbol: "DAI",
    decimals: 18,
    supply: 1_000_000n * 10n ** 18n,
    demo: 1_000n * 10n ** 18n,
  },
  {
    name: "MockWETH",
    symbol: "WETH",
    decimals: 18,
    supply: 10_000n * 10n ** 18n,
    demo: 10n * 10n ** 18n,
  },
  {
    name: "MockWBTC",
    symbol: "WBTC",
    decimals: 8,
    supply: 100n * 100_000_000n,
    demo: 1n * 100_000_000n,
  },
];

async function main() {
  const [deployer] = await hre.ethers.getSigners();
  const transferTo = process.env.TRANSFER_TO?.trim();
  const deployed = [];

  console.log("Deployer", deployer.address);
  console.log(
    "Network",
    hre.network.name,
    "chainId",
    (await hre.ethers.provider.getNetwork()).chainId.toString(),
  );
  console.log("--- Mock assets (test only) ---");

  for (const asset of BASKET) {
    const Factory = await hre.ethers.getContractFactory(asset.name);
    const token = await Factory.deploy(asset.supply);
    await token.waitForDeployment();
    const addr = await token.getAddress();

    console.log(asset.symbol, addr);
    console.log("  name    ", await token.name());
    console.log("  decimals", (await token.decimals()).toString());
    console.log("  supply  ", asset.supply.toString());

    if (transferTo) {
      const tx = await token.transfer(transferTo, asset.demo);
      await tx.wait();
      console.log("  demo to ", transferTo, asset.demo.toString());
    }

    deployed.push({ symbol: asset.symbol, decimals: asset.decimals, addr });
  }

  if (transferTo) {
    console.log("Demo amounts transferred to", transferTo);
  }

  console.log("--- MetaMask import ---");
  for (const t of deployed) {
    console.log(`symbol ${t.symbol} decimals ${t.decimals}  ${t.addr}`);
  }

  const known = deployed.map((t) => `${t.symbol}:${t.addr}`).join(",");
  console.log("Explorer known tokens: PUBLIC_KNOWN_TOKENS=" + known);
  console.log(
    "Explorer: https://explorer-dew.fadosoft.com/address/" + deployed[0].addr,
  );
}

main().catch((err) => {
  console.error(err);
  process.exitCode = 1;
});
