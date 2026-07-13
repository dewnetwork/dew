const hre = require("hardhat");

/**
 * Deploy MockUSDT (1_000_000 USDT, 6 decimals). Optional TRANSFER_TO gets 1000 USDT.
 * Optional MINT_TO + MINT_AMOUNT for extra owner mint (raw 6-decimal units).
 *
 * Usage:
 *   npx hardhat run scripts/deploy-usdt.js --network dewLocal
 *   TRANSFER_TO=0x… npx hardhat run scripts/deploy-usdt.js --network dewLocal
 *   PRIVATE_KEY=0x… npx hardhat run scripts/deploy-usdt.js --network dewPublic
 */
async function main() {
  const [deployer] = await hre.ethers.getSigners();
  // 1_000_000 * 10^6
  const supply = 1_000_000n * 1_000_000n;
  const demo = 1_000n * 1_000_000n;

  const MockUSDT = await hre.ethers.getContractFactory("MockUSDT");
  const usdt = await MockUSDT.deploy(supply);
  await usdt.waitForDeployment();
  const addr = await usdt.getAddress();

  console.log("MockUSDT", addr);
  console.log("Deployer", deployer.address);
  console.log("Supply (raw 6 dec)", supply.toString());
  console.log("name", await usdt.name());
  console.log("symbol", await usdt.symbol());
  console.log("decimals", (await usdt.decimals()).toString());
  console.log(
    "Network",
    hre.network.name,
    "chainId",
    (await hre.ethers.provider.getNetwork()).chainId.toString(),
  );

  const transferTo = process.env.TRANSFER_TO?.trim();
  if (transferTo) {
    const tx = await usdt.transfer(transferTo, demo);
    await tx.wait();
    console.log("Transferred", demo.toString(), "to", transferTo);
  }

  const mintTo = process.env.MINT_TO?.trim();
  const mintAmount = process.env.MINT_AMOUNT?.trim();
  if (mintTo && mintAmount && BigInt(mintAmount) > 0n) {
    const amount = BigInt(mintAmount);
    const tx = await usdt.mint(mintTo, amount);
    await tx.wait();
    console.log("Minted", amount.toString(), "to", mintTo);
  }

  console.log("Add to MetaMask: token address above, symbol USDT, decimals 6");
  console.log(
    "Explorer: https://explorer-dew.fadosoft.com/address/" + addr,
  );
}

main().catch((err) => {
  console.error(err);
  process.exitCode = 1;
});
