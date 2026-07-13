const hre = require("hardhat");

/**
 * Post one message. Env: GUESTBOOK (required), MESSAGE (optional).
 * Usage:
 *   GUESTBOOK=0x… MESSAGE="hello" npx hardhat run scripts/sign-guestbook.js --network dewLocal
 * Public default contract (path B): 0x83bB4E539BE46503481E66094b01b854990BF84a
 */
async function main() {
  const guestbook = process.env.GUESTBOOK?.trim();
  if (!guestbook) {
    throw new Error("Set GUESTBOOK=0x… (contract address)");
  }
  const message = process.env.MESSAGE?.trim() || "signed via hardhat";

  const book = await hre.ethers.getContractAt("Guestbook", guestbook);
  const tx = await book.sign(message);
  const receipt = await tx.wait();
  const total = await book.totalEntries();

  console.log("Guestbook", guestbook);
  console.log("Entry id", total - 1n);
  console.log("Total", total.toString());
  console.log("Tx", receipt.hash);
  console.log("Message", message);
}

main().catch((err) => {
  console.error(err);
  process.exitCode = 1;
});
