const { expect } = require("chai");
const { ethers } = require("hardhat");

describe("Guestbook", function () {
  it("appends messages and reverts empty", async function () {
    const Guestbook = await ethers.getContractFactory("Guestbook");
    const book = await Guestbook.deploy();
    await book.waitForDeployment();

    await (await book.sign("hello dew")).wait();
    expect(await book.totalEntries()).to.equal(1n);
    const [author, , message] = await book.getEntry(0);
    const [signer] = await ethers.getSigners();
    expect(author).to.equal(signer.address);
    expect(message).to.equal("hello dew");

    await expect(book.sign("")).to.be.revertedWith("empty");
  });
});
