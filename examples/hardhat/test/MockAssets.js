const { expect } = require("chai");
const { ethers } = require("hardhat");

const BASKET = [
  {
    name: "MockUSDT",
    tokenName: "Tether USD",
    symbol: "USDT",
    decimals: 6,
    supply: 1_000_000n * 1_000_000n,
  },
  {
    name: "MockUSDC",
    tokenName: "USD Coin",
    symbol: "USDC",
    decimals: 6,
    supply: 1_000_000n * 1_000_000n,
  },
  {
    name: "MockDAI",
    tokenName: "Dai Stablecoin",
    symbol: "DAI",
    decimals: 18,
    supply: 1_000_000n * 10n ** 18n,
  },
  {
    name: "MockWETH",
    tokenName: "Wrapped Ether",
    symbol: "WETH",
    decimals: 18,
    supply: 10_000n * 10n ** 18n,
  },
  {
    name: "MockWBTC",
    tokenName: "Wrapped BTC",
    symbol: "WBTC",
    decimals: 8,
    supply: 100n * 100_000_000n,
  },
];

describe("MockAssets suite", function () {
  for (const asset of BASKET) {
    it(`${asset.symbol}: metadata, supply, mint, transfer`, async function () {
      const [deployer, other] = await ethers.getSigners();
      const Factory = await ethers.getContractFactory(asset.name);
      const token = await Factory.deploy(asset.supply);
      await token.waitForDeployment();

      expect(await token.name()).to.equal(asset.tokenName);
      expect(await token.symbol()).to.equal(asset.symbol);
      expect(await token.decimals()).to.equal(asset.decimals);
      expect(await token.totalSupply()).to.equal(asset.supply);
      expect(await token.balanceOf(deployer.address)).to.equal(asset.supply);
      expect(await token.owner()).to.equal(deployer.address);

      const chunk = asset.supply / 10n;
      await (await token.transfer(other.address, chunk)).wait();
      expect(await token.balanceOf(other.address)).to.equal(chunk);

      const extra = 10n ** BigInt(asset.decimals);
      await (await token.mint(other.address, extra)).wait();
      expect(await token.balanceOf(other.address)).to.equal(chunk + extra);

      await expect(token.connect(other).mint(other.address, 1n)).to.be.revertedWithCustomError(
        token,
        "OwnableUnauthorizedAccount",
      );
    });
  }

  it("MockERC20 accepts custom name/symbol/decimals", async function () {
    const [deployer] = await ethers.getSigners();
    const Factory = await ethers.getContractFactory("MockERC20");
    const supply = 500n * 10n ** 9n;
    const token = await Factory.deploy("Custom", "CST", 9, supply);
    await token.waitForDeployment();

    expect(await token.name()).to.equal("Custom");
    expect(await token.symbol()).to.equal("CST");
    expect(await token.decimals()).to.equal(9);
    expect(await token.totalSupply()).to.equal(supply);
    expect(await token.balanceOf(deployer.address)).to.equal(supply);
  });
});
