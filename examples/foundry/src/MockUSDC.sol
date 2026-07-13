// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {MockERC20} from "./MockERC20.sol";

/// @title Mock USDC for Dew testnets.
/// @notice 6 decimals like mainnet Circle USDC. Not real USDC — test only.
contract MockUSDC is MockERC20 {
    /// @param initialSupply Raw units (6 decimals). e.g. 1_000_000e6 = 1M USDC.
    constructor(uint256 initialSupply) MockERC20("USD Coin", "USDC", 6, initialSupply) {}
}
