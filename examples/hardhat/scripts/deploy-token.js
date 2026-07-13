const hre = require("hardhat");

/**
 * Deploy Token (1_000_000 DST). Optional TRANSFER_TO gets 1000 DST.
 * Usage:
 *   npx hardhat run scripts/deploy-token.js --network dewLocal
 *   TRANSFER_TO=0x… npx hardhat run scripts/deploy-token.js --network dewLocal
 */
async function main() {
  const [deployer] = await hre.ethers.getSigners();
  const supply = hre.ethers.parseEther("1000000");
  const demo = hre.ethers.parseEther("1000");

  const Token = await hre.ethers.getContractFactory("Token");
  const token = await Token.deploy(supply);
  await token.waitForDeployment();
  const addr = await token.getAddress();

  console.log("Token", addr);
  console.log("Deployer", deployer.address);
  console.log("Supply", supply.toString());
  console.log("Network", hre.network.name, "chainId", (await hre.ethers.provider.getNetwork()).chainId.toString());

  const transferTo = process.env.TRANSFER_TO?.trim();
  if (transferTo) {
    const tx = await token.transfer(transferTo, demo);
    await tx.wait();
    console.log("Transferred", demo.toString(), "to", transferTo);
  }
}

main().catch((err) => {
  console.error(err);
  process.exitCode = 1;
});
