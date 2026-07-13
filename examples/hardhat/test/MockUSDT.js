const { expect } = require("chai");
const { ethers } = require("hardhat");

describe("MockUSDT", function () {
  const supply = 1_000_000n * 1_000_000n; // 1M USDT, 6 decimals

  async function deploy() {
    const [deployer, other, spender] = await ethers.getSigners();
    const MockUSDT = await ethers.getContractFactory("MockUSDT");
    const usdt = await MockUSDT.deploy(supply);
    await usdt.waitForDeployment();
    return { usdt, deployer, other, spender };
  }

  it("has USDT metadata and mints supply to deployer", async function () {
    const { usdt, deployer } = await deploy();
    expect(await usdt.name()).to.equal("Tether USD");
    expect(await usdt.symbol()).to.equal("USDT");
    expect(await usdt.decimals()).to.equal(6);
    expect(await usdt.totalSupply()).to.equal(supply);
    expect(await usdt.balanceOf(deployer.address)).to.equal(supply);
    expect(await usdt.owner()).to.equal(deployer.address);
  });

  it("transfers and approve/transferFrom", async function () {
    const { usdt, deployer, other, spender } = await deploy();
    const amount = 1_000n * 1_000_000n;

    await (await usdt.transfer(other.address, amount)).wait();
    expect(await usdt.balanceOf(other.address)).to.equal(amount);

    await (await usdt.approve(spender.address, amount)).wait();
    await (
      await usdt.connect(spender).transferFrom(deployer.address, other.address, amount)
    ).wait();
    expect(await usdt.balanceOf(other.address)).to.equal(amount * 2n);
  });

  it("owner can mint; non-owner cannot", async function () {
    const { usdt, other } = await deploy();
    const extra = 10n * 1_000_000n;
    await (await usdt.mint(other.address, extra)).wait();
    expect(await usdt.balanceOf(other.address)).to.equal(extra);
    expect(await usdt.totalSupply()).to.equal(supply + extra);

    await expect(usdt.connect(other).mint(other.address, 1n)).to.be.revertedWithCustomError(
      usdt,
      "OwnableUnauthorizedAccount",
    );
  });
});
