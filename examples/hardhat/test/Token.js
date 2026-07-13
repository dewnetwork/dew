const { expect } = require("chai");
const { ethers } = require("hardhat");

describe("Token", function () {
  it("mints supply to deployer and transfers", async function () {
    const [deployer, other] = await ethers.getSigners();
    const supply = ethers.parseEther("1000000");
    const Token = await ethers.getContractFactory("Token");
    const token = await Token.deploy(supply);
    await token.waitForDeployment();

    expect(await token.balanceOf(deployer.address)).to.equal(supply);
    const amount = ethers.parseEther("1000");
    await (await token.transfer(other.address, amount)).wait();
    expect(await token.balanceOf(other.address)).to.equal(amount);
    expect(await token.balanceOf(deployer.address)).to.equal(supply - amount);
  });
});
