const hre = require("hardhat");

/**
 * Deploy Guestbook; optional first message via MESSAGE (default Hello Dew…).
 * Usage:
 *   npx hardhat run scripts/deploy-guestbook.js --network dewLocal
 *   MESSAGE="hi" npx hardhat run scripts/deploy-guestbook.js --network dewPublic
 */
async function main() {
  const message =
    process.env.MESSAGE !== undefined
      ? process.env.MESSAGE
      : "Hello Dew public-testnet-v1";

  const Guestbook = await hre.ethers.getContractFactory("Guestbook");
  const book = await Guestbook.deploy();
  await book.waitForDeployment();
  const addr = await book.getAddress();

  console.log("Guestbook", addr);
  console.log("Network", hre.network.name);

  if (message.length > 0) {
    const tx = await book.sign(message);
    const receipt = await tx.wait();
    const total = await book.totalEntries();
    console.log("First entry id", total - 1n);
    console.log("Message", message);
    console.log("Tx", receipt.hash);
  }
}

main().catch((err) => {
  console.error(err);
  process.exitCode = 1;
});
