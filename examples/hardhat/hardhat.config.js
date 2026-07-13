require("dotenv").config();
require("@nomicfoundation/hardhat-toolbox");

/** @type import('hardhat/config').HardhatUserConfig */
const PRIVATE_KEY = process.env.PRIVATE_KEY?.trim();
const accounts =
  PRIVATE_KEY && PRIVATE_KEY.length > 0
    ? [PRIVATE_KEY.startsWith("0x") ? PRIVATE_KEY : `0x${PRIVATE_KEY}`]
    : [];

module.exports = {
  solidity: {
    version: "0.8.24",
    settings: {
      optimizer: { enabled: true, runs: 200 },
      evmVersion: "cancun",
    },
  },
  networks: {
    // Local dew devnet (Anvil #0 pre-funded on path B samples)
    dewLocal: {
      url: process.env.DEW_RPC_URL || "http://127.0.0.1:8545",
      chainId: 2205,
      accounts:
        accounts.length > 0
          ? accounts
          : [
              "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
            ],
    },
    // Public path B — set PRIVATE_KEY to a faucet-funded wallet (never Anvil #0)
    dewPublic: {
      url: process.env.DEW_RPC_URL || "https://rpc-dew.fadosoft.com",
      chainId: 2205,
      accounts,
    },
  },
};
