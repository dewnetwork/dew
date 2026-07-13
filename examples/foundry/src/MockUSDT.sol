// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {MockERC20} from "./MockERC20.sol";

/// @title Mock USDT for Dew testnets (local + public-testnet-v1).
/// @notice OpenZeppelin ERC-20 with 6 decimals like mainnet Tether. Not real USDT — test only.
/// @dev Owner can mint more supply for faucets / integration tests.
contract MockUSDT is MockERC20 {
    /// @param initialSupply Raw units (6 decimals). e.g. 1_000_000e6 = 1M USDT.
    constructor(uint256 initialSupply) MockERC20("Tether USD", "USDT", 6, initialSupply) {}
}
